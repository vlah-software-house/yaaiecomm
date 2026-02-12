# ROADMAP.md — ForgeCommerce

> Development roadmap in phases. Each builds on the previous.
> EU-first with VAT compliance from Phase 0.

---

## Phase 0 — Foundation (Weeks 1-3)

> Goal: Scaffolding, full schema (including VAT), authentication, build pipeline.

### 0.1 Project Setup

- [ ] Initialize Go module, Nuxt 3, Playwright projects
- [ ] docker-compose: PostgreSQL 16, mailpit, stripe-mock
- [ ] Configure sqlc, templ, air, golangci-lint
- [ ] CI pipeline skeleton, Makefile/Taskfile

### 0.2 Database Foundation

- [ ] Migration 001: `eu_countries` (all 27 EU members: code, name, local_vat_name, currency)
- [ ] Migration 002: `store_settings` (including VAT fields: vat_enabled, vat_number, vat_country, vat_prices_include_vat, vat_default_category, vat_b2b_reverse_charge)
- [ ] Migration 003: `store_shipping_countries` (which countries store sells to)
- [ ] Migration 004: `vat_categories` (standard, reduced, reduced_alt, super_reduced, parking, zero, exempt)
- [ ] Migration 005: `vat_rates` (per country per rate_type, with valid_from/to, source, synced_at)
- [ ] Migration 006: `vies_validation_cache`
- [ ] Migration 007: `admin_users`, `sessions`, `admin_audit_log`
- [ ] Migration 008: `categories`, `product_categories`
- [ ] Migration 009: `products` (includes vat_category_id FK)
- [ ] Migration 010: `product_vat_overrides` (per-product per-country)
- [ ] Migration 011: `product_attributes`, `product_attribute_options`
- [ ] Migration 012: `product_variants`, `product_variant_options`
- [ ] Migration 013: `product_images`, `media_assets`
- [ ] Migration 014: `raw_materials`, `raw_material_categories`, `raw_material_attributes`
- [ ] Migration 015: `product_bom_entries`, `attribute_option_bom_entries`, `attribute_option_bom_modifiers`, `variant_bom_overrides`
- [ ] Migration 016: `stock_movements`
- [ ] Migration 017: `customers`, `customer_addresses` (customer.vat_number for B2B)
- [ ] Migration 018: `orders` (vat fields), `order_items` (vat snapshots), `order_events`
- [ ] Migration 019: `discounts`, `coupons`, `coupon_usage`
- [ ] Migration 020: `shipping_config`, `shipping_zones`
- [ ] Migration 021: `webhook_endpoints`, `webhook_deliveries`
- [ ] All `down` migrations. `seed.sql` with EU countries, VAT rates, sample products.

### 0.3 VAT Rate Sync Service

- [ ] EC TEDB SOAP client (Go): call `RetrieveVatRates`, parse XML response
- [ ] euvatrates.com JSON fallback: fetch and parse `rates.json`
- [ ] Sync logic: try TEDB -> fallback to JSON -> fallback to DB cache
- [ ] Store rates in `vat_rates` table with source tracking
- [ ] In-memory cache: `map[string]CountryVATRates` refreshed on sync
- [ ] Startup sync: run on application boot
- [ ] Scheduled sync: Go internal cron (midnight UTC daily)
- [ ] Manual sync endpoint: `POST /admin/settings/vat/sync`
- [ ] Rate change detection and audit logging
- [ ] Unit tests: mock both sources, test fallback chain, test rate parsing

### 0.4 Admin Authentication & 2FA

- [ ] Password hashing (bcrypt, cost 12)
- [ ] Login: email/password -> 2FA check
- [ ] 2FA setup: TOTP secret (pquerna/otp), QR code (skip2/go-qrcode), verify
- [ ] Recovery codes: generate 8, hash storage, burn on use
- [ ] Session management: PostgreSQL-backed, httpOnly cookies
- [ ] Middlewares: session, CSRF, force-2FA
- [ ] Audit logging

### 0.5 Admin Layout Shell

- [ ] Base layout (templ): sidebar, header, content
- [ ] HTMX + Alpine.js in static assets
- [ ] Login, 2FA setup/verify, dashboard (placeholder) templates
- [ ] Responsive CSS, flash messages

### 0.6 E2E Foundation

- [ ] Playwright config, DB helpers, admin auth helpers
- [ ] Test: Admin login -> 2FA setup -> dashboard
- [ ] Test: Admin login with existing 2FA
- [ ] CI: E2E in docker-compose.test

**Exit Criteria:** Auth with 2FA works. Full schema deployed. VAT rates syncing from EC on startup. CI running.

---

## Phase 1 — Products, Inventory & VAT Config (Weeks 4-8)

> Goal: Full product CRUD with attributes, variants, BOM, VAT settings, country config.

### 1.1 Store Settings — VAT & Countries (Admin)

- [ ] Settings page: VAT configuration
  - [ ] Enable/disable VAT (master toggle)
  - [ ] Store VAT number (with VIES self-validation)
  - [ ] Store country selection
  - [ ] Prices include VAT toggle
  - [ ] Default VAT category dropdown
  - [ ] B2B reverse charge toggle
- [ ] Settings page: Selling countries
  - [ ] Checkbox grid of all 27 EU countries
  - [ ] Select all / deselect all
  - [ ] Store's own country always enabled and highlighted
- [ ] Settings page: Current VAT rates display
  - [ ] Table of rates per enabled country (all rate types)
  - [ ] Last sync timestamp and source
  - [ ] "Sync Now" button (HTMX)
  - [ ] Manual rate edit (overrides auto-sync, marked as "manual" source)

### 1.2 Product CRUD (Admin)

- [ ] sqlc queries, ProductService, admin handlers
- [ ] List (paginated, HTMX live search), create, edit (tabbed), delete/archive
- [ ] Product status: draft -> active -> archived
- [ ] Slug auto-generation, SEO fields

### 1.3 Product VAT Settings

- [ ] VAT tab on product edit (or section in Details tab):
  - [ ] VAT category dropdown (overrides store default)
  - [ ] Per-country override table: add country + category pairs
  - [ ] Only enabled selling countries available in dropdown
  - [ ] Inline help explaining the resolution priority

### 1.4 Attributes & Variant Generation

- [ ] Attributes tab: add/edit/remove attributes and options (HTMX)
- [ ] Variant generation: Cartesian product, auto-SKU, preserve existing
- [ ] Variants tab: table with inline editing (price, weight, stock, barcode, active)

### 1.5 Category & Image Management

- [ ] Category CRUD with hierarchy
- [ ] Image upload: validate, resize, WebP convert, drag-drop, reorder, assign to variant/option

### 1.6 Raw Materials & BOM

- [ ] Raw material CRUD with attributes, stock adjustment, movement logging
- [ ] BOM tab: Layer 1 (product), Layer 2a (option materials), Layer 2b (option modifiers), Layer 3 (variant overrides)
- [ ] Producibility calculator + preview (HTMX)
- [ ] Inventory dashboard: low stock alerts, movements, producibility

### 1.7 E2E Tests — Phase 1

- [ ] Configure VAT settings (enable, set country, select selling countries)
- [ ] VAT rate sync (manual trigger, verify display)
- [ ] Create product with VAT category + per-country overrides
- [ ] Create product with attributes, generate variants
- [ ] Configure BOM, verify producibility
- [ ] Raw material management, stock adjustments

**Exit Criteria:** Products fully manageable with attributes/variants/BOM/VAT. VAT rates sync and display. Country restrictions configured. E2E coverage.

---

## Phase 2 — Storefront & Checkout with VAT (Weeks 9-14)

> Goal: Customer store with EU VAT-aware checkout, Stripe, orders.

### 2.1 Public API

- [ ] Products, categories, variant matrix endpoints
- [ ] `GET /api/v1/countries` — enabled selling countries
- [ ] Cart endpoints (variant_id based)
- [ ] `POST /api/v1/cart/:id/vat-number` — validate B2B VAT number (VIES)
- [ ] `POST /api/v1/checkout/calculate` — preview with VAT per destination country

### 2.2 VAT Calculation Service

- [ ] Implement full VAT calculation algorithm:
  - [ ] Resolve VAT category (override -> product -> store default)
  - [ ] Look up rate for destination country + rate type
  - [ ] Fallback to standard if rate type doesn't exist in country
  - [ ] Handle prices-include-VAT and prices-exclude-VAT
  - [ ] Handle B2B reverse charge (0% when VIES-validated cross-border)
- [ ] VIES client: SOAP call to EC, response parsing, caching (24h TTL)
- [ ] Unit tests: exhaustive matrix (every country, every rate type, B2B, same-country, disabled VAT)

### 2.3 Stripe Integration

- [ ] Checkout Session: line items with VAT, shipping
- [ ] Handle VAT in Stripe: pass tax amounts or use Stripe Tax (evaluate)
- [ ] Webhook: checkout.session.completed -> create order with VAT snapshots
- [ ] Idempotency, stripe-mock for dev

### 2.4 Shipping (Country-Aware)

- [ ] Destination country validation (must be enabled)
- [ ] Base fee calculation (fixed/weight/size), optional shipping zones
- [ ] Per-product extra fee
- [ ] Admin: shipping config + shipping zone editor

### 2.5 Order Management

- [ ] Order creation with full VAT snapshot (per-item rates, totals, reverse charge flag)
- [ ] Admin: order list, detail (VAT breakdown visible), status management
- [ ] Transactional emails: order confirmation (shows VAT breakdown)

### 2.6 Nuxt 3 Storefront

- [ ] Pages: home, category, product detail, cart, checkout redirect, confirmation
- [ ] **Variant picker**: attribute selectors, reactive filtering, stock/price display
- [ ] **Country selector**: dropdown of enabled countries, updates VAT display
- [ ] **VAT display**: shows rate + amount, updates when country changes
- [ ] **B2B VAT number field**: enter number, validate via API, show reverse charge if valid
- [ ] Tailwind CSS, responsive, SEO (meta, JSON-LD)

### 2.7 Customer Accounts (can defer)

- [ ] Registration, login, JWT
- [ ] Customer vat_number field (B2B customers can save their VAT number)

### 2.8 E2E Tests — Phase 2

- [ ] Browse -> variant select -> cart -> checkout -> confirmation
- [ ] Country selection: VAT updates on country change
- [ ] B2B: enter VAT number -> reverse charge -> updated totals
- [ ] Attempt disabled country -> rejected
- [ ] Shipping calculation per zone
- [ ] Order VAT snapshots match checkout calculation
- [ ] Stripe webhook creates correct order

**Exit Criteria:** Full checkout with EU VAT. Country restrictions enforced. B2B reverse charge works. VAT snapshots on orders.

---

## Phase 3 — Discounts, Coupons & Refinement (Weeks 15-18)

> Goal: Discount engine, coupons, admin polish.

### 3.1 Discount Engine

- [ ] Evaluate rules, conditions, scopes, stacking
- [ ] Discounts on net or gross amounts (configurable)
- [ ] Admin: discount CRUD with conditions builder

### 3.2 Coupon System

- [ ] Coupon CRUD, cart application, usage tracking, error messages

### 3.3 Admin Dashboard

- [ ] Widgets: revenue, orders, low stock, pending orders
- [ ] HTMX auto-refresh

### 3.4 Admin User Management

- [ ] CRUD, roles, permissions, 2FA status

### 3.5 Import/Export

- [ ] CSV import/export for products, raw materials, stock

### 3.6 E2E Tests — Phase 3

- [ ] Coupon flows, discount stacking, dashboard accuracy

**Exit Criteria:** Discounts and coupons work. Admin dashboard operational.

---

## Phase 4 — Reports, VAT Reports & Production (Weeks 19-22)

> Goal: Sales + VAT reporting, production batches, production readiness.

### 4.1 Sales Reports

- [ ] Daily/weekly/monthly aggregation
- [ ] Charts (Chart.js/uPlot), comparison overlays, predictions
- [ ] Metrics: revenue (net + gross), VAT collected, order count, AOV
- [ ] CSV export

### 4.2 VAT Report (for tax filing)

- [ ] **VAT summary per country**: total sales, VAT collected, grouped by rate type
- [ ] **Period selection**: monthly, quarterly, yearly
- [ ] **B2B reverse charge report**: orders with reverse charge, customer VAT numbers
- [ ] **Export**: CSV format suitable for submitting to tax authorities
- [ ] This is a high-value feature for EU businesses — makes quarterly VAT filing straightforward

```
VAT Report — Q1 2026

Country     Rate Type    Net Sales    VAT Collected   Gross Sales
Spain       Standard     €12,450.00   €2,614.50       €15,064.50
Spain       Reduced      €3,200.00    €320.00         €3,520.00
France      Standard     €5,100.00    €1,020.00       €6,120.00
France      Reduced      €800.00      €44.00          €844.00
Germany     Standard     €8,300.00    €1,577.00       €9,877.00
Germany     Reduced      €1,200.00    €84.00          €1,284.00
...

B2B Reverse Charge:
  12 orders, €15,600.00 net, VAT: €0.00
  Customer VAT numbers: DE123456789, FR12345678901, ...

Total VAT Collected: €8,247.50
Total Net Revenue: €46,350.00
```

### 4.3 Sales Predictions

- [ ] WMA, day-of-week adjustment, YoY blending
- [ ] Prediction line on chart
- [ ] Unit tests with mock data

### 4.4 Production Batches (Optional)

- [ ] Plan -> validate materials -> complete (atomic BOM deduction)
- [ ] Admin: batch list, create, detail
- [ ] Cost tracking

### 4.5 Webhook System

- [ ] Endpoint registration, HMAC signing, async delivery, retry, log

### 4.6 Performance & Hardening

- [ ] Query optimization, caching, rate limiting
- [ ] Security headers, load testing

### 4.7 Deployment Preparation

- [ ] Multi-stage Dockerfiles, health checks, structured logging, graceful shutdown
- [ ] Backup strategy, deployment docs

### 4.8 E2E Tests — Phase 4

- [ ] Sales report accuracy, prediction display
- [ ] VAT report: per-country breakdown matches orders
- [ ] VAT report: B2B reverse charge section
- [ ] Production batch flow
- [ ] Webhook test ping

**Exit Criteria:** Sales + VAT reports working. VAT report suitable for filing. Production batches work. App hardened.

---

## Phase 5 — Polish & Launch (Weeks 23-25)

### 5.1 Storefront Polish

- [ ] Loading states, error pages, OG tags, performance, Lighthouse 90+

### 5.2 Admin Polish

- [ ] Keyboard shortcuts, help text, confirmations, batch actions

### 5.3 Documentation

- [ ] API docs, admin guide, deployment guide, developer setup
- [ ] VAT model documentation (for accountants/business owners)
- [ ] Attributes/variants/BOM model docs

### 5.4 Final Testing

- [ ] Smoke suite (@critical), full regression, accessibility (axe)

### 5.5 Launch Checklist

- [ ] All tests passing, security checklist completed
- [ ] Stripe production keys, SMTP configured
- [ ] DNS + TLS, DB backups, monitoring
- [ ] VAT rate sync verified in production
- [ ] VIES validation working against real EC service
- [ ] Super admin with 2FA created

---

## Future Considerations

**Near-Term:** Multi-currency, multi-language, OSS scheme (One-Stop Shop for cross-border EU VAT), search (Meilisearch), email marketing, reviews.

**Medium-Term:** Redis, job queue (River), S3 media, multiple payment providers, real-time carrier shipping, supplier management, multi-warehouse, Stripe Tax integration.

**Long-Term:** Multi-tenant, marketplace, subscriptions, advanced analytics, mobile app, GraphQL, plugin system, non-EU country support.

---

## Technical Debt & Quality Gates

- Every PR: lint, unit, integration tests
- E2E on merge to main
- Performance budget: <200ms p95 listings, <100ms detail
- Test coverage: 80%+ services, 100% VAT/discount/shipping/BOM engines
- Monthly dependency audit

---

## Summary

| Phase | Focus | Weeks | Key Deliverables |
|-------|-------|-------|-----------------|
| **0** | Foundation | 3 | Auth + 2FA, full schema, VAT rate sync, CI |
| **1** | Products & Inventory | 5 | Attributes, variants, BOM, VAT config, country restrictions |
| **2** | Storefront & Checkout | 6 | Nuxt store, VAT-aware checkout, B2B reverse charge, Stripe |
| **3** | Discounts & Polish | 4 | Discount engine, coupons, admin dashboard |
| **4** | Reports & Production | 4 | Sales + VAT reports, predictions, production batches, webhooks |
| **5** | Launch | 3 | Polish, docs, testing, deployment |
| **Total** | | **~25 weeks** | |
