# ForgeCommerce Deployment Guide

## Prerequisites

- Docker & Docker Compose
- PostgreSQL 16+ (or use Docker)
- Node.js 22+ (for storefront build)
- Go 1.22+ (for API build)
- Stripe account (test or production keys)

---

## Environment Variables

Create a `.env` file or set environment variables:

```env
# Server
PORT=8080
ADMIN_PORT=8081
BASE_URL=https://store.example.com
ADMIN_URL=https://admin.store.example.com

# Database
DATABASE_URL=postgres://forge:SECURE_PASSWORD@localhost:5432/forgecommerce?sslmode=require

# Auth
JWT_SECRET=<random-64-char-string>
SESSION_SECRET=<random-64-char-string>
TOTP_ISSUER=ForgeCommerce

# Stripe
STRIPE_SECRET_KEY=sk_live_...
STRIPE_WEBHOOK_SECRET=whsec_...
STRIPE_PUBLIC_KEY=pk_live_...

# Media
MEDIA_STORAGE=local
MEDIA_PATH=./media

# Email
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_FROM=orders@example.com

# VAT
VAT_SYNC_ENABLED=true
VAT_SYNC_CRON=0 0 * * *
VAT_TEDB_TIMEOUT=30s
VAT_EUVATRATES_FALLBACK_URL=https://euvatrates.com/rates.json
VIES_TIMEOUT=10s
VIES_CACHE_TTL=24h
```

---

## Development Setup

### 1. Start PostgreSQL

```bash
docker-compose up -d postgres
```

### 2. Run API Server

```bash
cd api
go run ./cmd/server
```

The API starts on `:8080` (public API) and `:8081` (admin panel).

### 3. Run Storefront

```bash
cd storefront
npm install
npm run dev
```

The storefront starts on `:3000` with HMR.

### 4. Access Services

- **Storefront:** http://localhost:3000
- **Admin Panel:** http://localhost:8081/admin/login
- **API Health:** http://localhost:8080/api/v1/health
- **Mailpit:** http://localhost:8025 (email capture)

---

## Docker Deployment

### Using Docker Compose

```bash
docker-compose up -d
```

This starts all services:

| Service     | Port  | Description               |
|-------------|-------|---------------------------|
| postgres    | 5432  | PostgreSQL 16             |
| api         | 8080  | Go API + Admin (8081)     |
| storefront  | 3000  | Nuxt 3 SSR               |
| stripe-mock | 12111 | Stripe API mock (dev)     |
| mailpit     | 8025  | Email capture (dev)       |

### Building Images

```bash
# API (multi-stage: Go build → Alpine)
docker build -t forgecommerce-api -f api/Dockerfile .

# Storefront (multi-stage: Node build → Node runtime)
docker build -t forgecommerce-storefront -f storefront/Dockerfile .
```

### Production Docker Compose

For production, remove `stripe-mock` and `mailpit`, and configure real services:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: forgecommerce
      POSTGRES_USER: forge
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    restart: always

  api:
    image: forgecommerce-api:latest
    ports:
      - "8080:8080"
      - "8081:8081"
    environment:
      DATABASE_URL: postgres://forge:${DB_PASSWORD}@postgres:5432/forgecommerce?sslmode=disable
      # ... all env vars
    depends_on:
      - postgres
    restart: always

  storefront:
    image: forgecommerce-storefront:latest
    ports:
      - "3000:3000"
    environment:
      NUXT_PUBLIC_API_BASE: http://api:8080
    depends_on:
      - api
    restart: always
```

---

## Database

### Migrations

Migrations run automatically on API server startup. They are located in:

```
api/internal/database/migrations/
  001_eu_countries.up.sql / .down.sql
  002_store_settings.up.sql / .down.sql
  ...
  021_webhooks.up.sql / .down.sql
```

### Manual Migration

```bash
# Using golang-migrate CLI
migrate -database "${DATABASE_URL}" -path api/internal/database/migrations up

# Rollback last migration
migrate -database "${DATABASE_URL}" -path api/internal/database/migrations down 1
```

### Seed Data

Load initial data (EU countries, VAT categories, VAT rates, sample admin user):

```bash
psql "${DATABASE_URL}" < scripts/seed.sql
```

The seed script uses `ON CONFLICT DO NOTHING` and is safe to run multiple times.

### Backups

```bash
# Full backup
pg_dump "${DATABASE_URL}" > backup_$(date +%Y%m%d).sql

# Restore
psql "${DATABASE_URL}" < backup_20260212.sql
```

---

## First-Time Setup

1. **Start the stack** (`docker-compose up` or manual)
2. **Seed the database** (`psql < scripts/seed.sql`)
3. **Log in to admin** at `/admin/login`
   - Default: `admin@forgecommerce.local` / `admin123`
   - You will be prompted to set up 2FA on first login
4. **Configure VAT settings** at `/admin/settings`
   - Enable/disable VAT
   - Set your store's VAT number and country
   - Select which EU countries you sell to
   - Trigger a VAT rate sync
5. **Create products** at `/admin/products`
6. **Configure Stripe webhook** in Stripe Dashboard:
   - URL: `https://your-domain.com/api/v1/webhooks/stripe`
   - Events: `checkout.session.completed`, `payment_intent.succeeded`, `payment_intent.payment_failed`

---

## Reverse Proxy (Nginx)

```nginx
# Storefront
server {
    listen 443 ssl;
    server_name store.example.com;

    ssl_certificate /etc/ssl/certs/store.pem;
    ssl_certificate_key /etc/ssl/private/store.key;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # API passthrough
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Admin panel
server {
    listen 443 ssl;
    server_name admin.store.example.com;

    ssl_certificate /etc/ssl/certs/admin.pem;
    ssl_certificate_key /etc/ssl/private/admin.key;

    location / {
        proxy_pass http://localhost:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Health Checks

- **API:** `GET /api/v1/health` → `{"status":"ok"}`
- **Admin:** `GET /admin/health` → `{"status":"ok"}`
- **Database:** API health check implicitly validates DB connection

---

## Monitoring

### Structured Logging

The API uses `slog` with JSON output. All requests are logged with:
- Method, path, status code, duration
- Request ID for tracing
- Error details on failures

### Key Metrics to Monitor

- API response times (p50, p95, p99)
- Error rates (5xx responses)
- Database connection pool usage
- VAT sync success/failure
- Stripe webhook processing time
- Order creation rate

---

## Security Checklist

Before going to production:

- [ ] Change default admin password
- [ ] Set strong `JWT_SECRET` and `SESSION_SECRET` (64+ chars)
- [ ] Enable HTTPS (TLS) on all endpoints
- [ ] Configure CORS to only allow your storefront domain
- [ ] Verify Stripe webhook secret is set
- [ ] Enable 2FA for all admin users
- [ ] Set up database backups (daily minimum)
- [ ] Configure rate limiting at reverse proxy level
- [ ] Review security headers (CSP, HSTS, X-Frame-Options)
- [ ] Disable debug logging in production
- [ ] Verify VAT rate sync is working
- [ ] Test VIES validation against real EC service
- [ ] Set `SameSite=Strict` and `Secure` on session cookies
