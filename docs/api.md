# ForgeCommerce Public API Reference

> Base URL: `http://localhost:8080/api/v1`
> Content-Type: `application/json`
> Authentication: JWT Bearer token (customer routes only)

---

## Health Check

```
GET /api/v1/health
```

**Response:** `200 OK`
```json
{ "status": "ok" }
```

---

## Products

### List Products

```
GET /api/v1/products?page=1&per_page=20&category=&status=active
```

**Query Parameters:**

| Parameter  | Type   | Default | Description                    |
|-----------|--------|---------|--------------------------------|
| page      | int    | 1       | Page number                    |
| per_page  | int    | 20      | Items per page (max 100)       |
| category  | string | —       | Filter by category slug        |
| status    | string | active  | Filter by product status       |

**Response:** `200 OK`
```json
{
  "products": [
    {
      "id": "uuid",
      "name": "Leather Messenger Bag",
      "slug": "leather-messenger-bag",
      "short_description": "Handcrafted leather bag",
      "base_price": "99.00",
      "compare_at_price": "129.00",
      "status": "active",
      "has_variants": true,
      "images": [{ "url": "/media/...", "alt_text": "...", "is_primary": true }]
    }
  ],
  "total": 42,
  "page": 1,
  "per_page": 20
}
```

### Get Product

```
GET /api/v1/products/{slug}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "Leather Messenger Bag",
  "slug": "leather-messenger-bag",
  "description": "Full HTML description...",
  "short_description": "Handcrafted leather bag",
  "base_price": "99.00",
  "compare_at_price": "129.00",
  "status": "active",
  "has_variants": true,
  "base_weight_grams": 500,
  "vat_category": "standard",
  "seo_title": "...",
  "seo_description": "...",
  "attributes": [
    {
      "id": "uuid",
      "name": "color",
      "display_name": "Color",
      "type": "color_swatch",
      "options": [
        { "id": "uuid", "value": "black", "display_value": "Black", "color_hex": "#000000" },
        { "id": "uuid", "value": "tan", "display_value": "Tan", "color_hex": "#D2B48C" }
      ]
    }
  ],
  "images": [],
  "categories": []
}
```

### List Product Variants

```
GET /api/v1/products/{slug}/variants
```

**Response:** `200 OK`
```json
{
  "variants": [
    {
      "id": "uuid",
      "sku": "LMB-BLK-STD",
      "price": "99.00",
      "compare_at_price": null,
      "stock_quantity": 15,
      "weight_grams": 500,
      "is_active": true,
      "options": {
        "color": "black",
        "size": "standard"
      }
    }
  ]
}
```

---

## Categories

### List Categories

```
GET /api/v1/categories
```

**Response:** `200 OK`
```json
{
  "categories": [
    {
      "id": "uuid",
      "name": "Bags",
      "slug": "bags",
      "description": "...",
      "parent_id": null,
      "children": []
    }
  ]
}
```

---

## Countries

### List Enabled Selling Countries

```
GET /api/v1/countries
```

Returns only countries the store has enabled for shipping/selling.

**Response:** `200 OK`
```json
{
  "countries": [
    { "code": "ES", "name": "Spain", "currency": "EUR" },
    { "code": "DE", "name": "Germany", "currency": "EUR" },
    { "code": "FR", "name": "France", "currency": "EUR" }
  ]
}
```

---

## Cart

### Create Cart

```
POST /api/v1/cart
```

**Request Body:** _(empty or with optional country)_
```json
{ "country_code": "ES" }
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "expires_at": "2026-02-19T12:00:00Z",
  "created_at": "2026-02-12T12:00:00Z"
}
```

### Get Cart

```
GET /api/v1/cart/{id}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "country_code": "ES",
  "items": [
    {
      "id": "uuid",
      "variant_id": "uuid",
      "product_name": "Leather Messenger Bag",
      "variant_name": "Black / Standard",
      "sku": "LMB-BLK-STD",
      "quantity": 2,
      "unit_price": "99.00",
      "total_price": "198.00"
    }
  ],
  "subtotal": "198.00",
  "item_count": 2,
  "vat_number": null,
  "coupon_code": null
}
```

### Update Cart

```
PATCH /api/v1/cart/{id}
```

**Request Body:**
```json
{ "country_code": "DE" }
```

### Add Item to Cart

```
POST /api/v1/cart/{id}/items
```

**Request Body:**
```json
{
  "variant_id": "uuid",
  "quantity": 1
}
```

**Response:** `201 Created` — updated cart

### Update Cart Item

```
PATCH /api/v1/cart/{id}/items/{itemId}
```

**Request Body:**
```json
{ "quantity": 3 }
```

### Remove Cart Item

```
DELETE /api/v1/cart/{id}/items/{itemId}
```

**Response:** `204 No Content`

---

## VAT Number Validation (B2B)

### Validate & Apply VAT Number

```
POST /api/v1/cart/{id}/vat-number
```

Validates a EU VAT number via VIES and applies reverse charge if valid for cross-border B2B.

**Request Body:**
```json
{ "vat_number": "DE123456789" }
```

**Response:** `200 OK`
```json
{
  "valid": true,
  "company_name": "Example GmbH",
  "company_address": "Berlin, Germany",
  "reverse_charge": true,
  "message": "VAT number validated. Reverse charge applies."
}
```

**Error Response:** `422 Unprocessable Entity`
```json
{
  "valid": false,
  "message": "VAT number is not valid according to VIES."
}
```

---

## Checkout

### Calculate Totals (Preview)

```
POST /api/v1/checkout/calculate
```

Preview order totals including VAT for a specific destination country.

**Request Body:**
```json
{
  "cart_id": "uuid",
  "country_code": "FR",
  "vat_number": null
}
```

**Response:** `200 OK`
```json
{
  "subtotal_net": "198.00",
  "vat_amount": "39.60",
  "vat_rate": 20.0,
  "vat_rate_type": "standard",
  "shipping_fee": "8.50",
  "discount_amount": "0.00",
  "total": "246.10",
  "reverse_charge": false,
  "country_code": "FR",
  "items": [
    {
      "product_name": "Leather Messenger Bag",
      "quantity": 2,
      "unit_price": "99.00",
      "vat_rate": 20.0,
      "vat_amount": "39.60",
      "line_total": "237.60"
    }
  ]
}
```

### Create Checkout Session

```
POST /api/v1/checkout
```

Creates a Stripe Checkout Session and returns the redirect URL.

**Request Body:**
```json
{
  "cart_id": "uuid",
  "email": "customer@example.com",
  "country_code": "ES",
  "shipping_address": {
    "line1": "Calle Mayor 1",
    "city": "Madrid",
    "postal_code": "28001",
    "country": "ES"
  },
  "vat_number": null
}
```

**Response:** `200 OK`
```json
{
  "checkout_url": "https://checkout.stripe.com/...",
  "session_id": "cs_test_..."
}
```

---

## Customer Authentication

### Register

```
POST /api/v1/customers/register
```

**Request Body:**
```json
{
  "email": "customer@example.com",
  "password": "securepassword",
  "first_name": "Maria",
  "last_name": "Garcia"
}
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "email": "customer@example.com",
  "access_token": "eyJhb...",
  "refresh_token": "eyJhb..."
}
```

### Login

```
POST /api/v1/customers/login
```

**Request Body:**
```json
{
  "email": "customer@example.com",
  "password": "securepassword"
}
```

**Response:** `200 OK`
```json
{
  "access_token": "eyJhb...",
  "refresh_token": "eyJhb...",
  "expires_in": 900
}
```

### Refresh Token

```
POST /api/v1/customers/refresh
```

**Request Body:**
```json
{ "refresh_token": "eyJhb..." }
```

### Get Profile (Authenticated)

```
GET /api/v1/customers/me
Authorization: Bearer <access_token>
```

### Update Profile (Authenticated)

```
PATCH /api/v1/customers/me
Authorization: Bearer <access_token>
```

### List My Orders (Authenticated)

```
GET /api/v1/customers/me/orders
Authorization: Bearer <access_token>
```

---

## Stripe Webhooks

```
POST /api/v1/webhooks/stripe
```

Receives Stripe webhook events. Signature verified via `STRIPE_WEBHOOK_SECRET`.

**Handled Events:**
- `checkout.session.completed` — Creates order from completed checkout
- `payment_intent.succeeded` — Updates order payment status
- `payment_intent.payment_failed` — Marks order payment as failed

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": "Human-readable error message",
  "code": "error_code"
}
```

**Common Status Codes:**

| Code | Meaning                          |
|------|----------------------------------|
| 400  | Bad Request (invalid input)      |
| 401  | Unauthorized (missing/invalid JWT) |
| 404  | Not Found                        |
| 409  | Conflict (e.g., duplicate email) |
| 422  | Unprocessable Entity (validation) |
| 429  | Rate Limited                     |
| 500  | Internal Server Error            |

---

## Rate Limiting

- General API: 100 requests/minute per IP
- Login/Register: 10 requests/minute per IP
- VIES Validation: 5 requests/minute per IP
- Checkout: 10 requests/minute per IP

---

## CORS

The API allows cross-origin requests from the configured `BASE_URL` (storefront origin).

Allowed methods: `GET, POST, PATCH, DELETE, OPTIONS`
Allowed headers: `Content-Type, Authorization`
