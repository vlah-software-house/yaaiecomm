# CLAUDE.md — ForgeCommerce

> A proprietary e-commerce platform for EU small businesses and manufacturers.
> Built with Go (API + Admin), Nuxt 3 (Storefront), PostgreSQL, Stripe, and Playwright.
> EU-first: full VAT support, country restrictions, VIES validation, B2B reverse charge.

---

## Project Identity

- **Name**: ForgeCommerce (working title)
- **License**: Proprietary. All rights reserved. Not open source.
- **Purpose**: End-to-end e-commerce for EU small businesses, with first-class support for small manufacturers who manage raw materials, production, and finished goods inventory. EU VAT compliance built in.
- **Primary Market**: European Union. The platform is designed EU-first with proper VAT handling, country-based shipping restrictions, and B2B reverse charge support.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                      Clients                            │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │  Storefront  │  │ Admin Panel  │  │  Public API   │  │
│  │  (Nuxt 3)   │  │ (Go + HTMX)  │  │  (REST/JSON)  │  │
│  └──────┬──────┘  └──────┬───────┘  └───────┬───────┘  │
└─────────┼────────────────┼──────────────────┼───────────┘
          │                │                  │
          ▼                ▼                  ▼
┌─────────────────────────────────────────────────────────┐
│                   Go API Server                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐  │
│  │ Handlers │ │ Services │ │   Auth   │ │ Middleware │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐  │
│  │  Stripe  │ │ Shipping │ │   VAT    │ │ Webhooks  │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────┘  │
│  ┌──────────┐ ┌──────────┐                              │
│  │  Media   │ │ Scheduler│  (VAT sync, scheduled tasks) │
│  └──────────┘ └──────────┘                              │
└────────────────────────┬────────────────────────────────┘
                         │
     ┌───────────────────┼───────────────────┐
     ▼                   ▼                   ▼
┌──────────┐      ┌────────────┐      ┌────────────┐
│PostgreSQL│      │ File Store │      │  External  │
│          │      │ (local/S3) │      │  Services  │
└──────────┘      └────────────┘      │  - Stripe  │
                                      │  - EC TEDB │
                                      │  - VIES    │
                                      └────────────┘
```

### Component Responsibilities

| Component | Technology | Role |
|-----------|-----------|------|
| **API Server** | Go (chi or net/http) | All business logic, REST API, admin HTML rendering |
| **Admin Panel** | Go (templ) + HTMX + Alpine.js | Server-rendered admin with dynamic interactions |
| **Storefront** | Nuxt 3 (Vue 3, SSR) | Customer-facing store consuming the Go API |
| **Database** | PostgreSQL 16+ | Single database. No ORM — sqlc + pgx |
| **Payments** | Stripe (Checkout → Elements later) | Payment processing, webhook-driven |
| **VAT Engine** | Go service + scheduled sync | Rate management, calculation, VIES validation |
| **Media** | Local filesystem (S3-compatible interface) | Product images, assets |
| **Scheduler** | Go (internal cron) | Daily VAT rate sync, scheduled tasks |
| **Testing** | Playwright (E2E), Go testing (unit/integration) | Full test coverage |

---

## Tech Stack

- **Go API + Admin**: chi/net/http, templ, HTMX, Alpine.js, sqlc, pgx, golang-migrate
- **Storefront**: Nuxt 3, Tailwind CSS, Pinia
- **Database**: PostgreSQL 16+, plain SQL migrations, sqlc code generation
- **No ORM**: sqlc generates type-safe Go from SQL. Full query control.

---

## Project Structure

```
forgecommerce/
├── CLAUDE.md, ROADMAP.md
├── docker-compose.yml, docker-compose.test.yml
├── api/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── database/ (migrations/, queries/, gen/)
│   │   ├── auth/
│   │   ├── handlers/ (api/, admin/)
│   │   ├── middleware/
│   │   ├── models/
│   │   ├── services/ (product, variant, bom, inventory, order, shipping, discount, payment, report, media, vat)
│   │   ├── vat/               # VAT engine: rates sync, calculation, VIES client
│   │   │   ├── engine.go      # VAT calculation logic
│   │   │   ├── sync.go        # EC TEDB + fallback rate sync
│   │   │   ├── vies.go        # VIES VAT number validation client
│   │   │   └── scheduler.go   # Midnight sync scheduler
│   │   ├── stripe/
│   │   ├── shipping/
│   │   ├── discount/
│   │   ├── reports/
│   │   └── webhook/
│   ├── templates/ (layouts/, admin/, components/)
│   ├── static/ (css/, js/, img/)
│   ├── go.mod, sqlc.yaml, Dockerfile
├── storefront/ (Nuxt 3: pages/, components/, composables/, stores/)
├── tests/ (playwright: e2e/, fixtures/, helpers/)
├── scripts/ (seed.sql, migrate.sh, generate.sh)
└── docs/ (api.md, admin.md, vat-model.md, attributes-variants.md, deployment.md)
```

---

## EU VAT System — Complete Design

### Overview

VAT support is **optional but comprehensive**. A manufacturer who is not a VAT payer simply disables VAT and all prices are net. A VAT-registered business gets full EU VAT compliance: correct rates per country, per product category, automatic rate updates, B2B reverse charge, and VAT-compliant invoicing data.

### VAT Rate Types in the EU

The EU VAT Directive defines these rate types that member states can apply:

| Rate Type | Description | Example |
|-----------|-------------|---------|
| **Standard** | Default rate, minimum 15% | DE: 19%, FR: 20%, ES: 21%, HU: 27% |
| **Reduced** | First reduced rate, minimum 5% | DE: 7%, FR: 5.5%, ES: 10% |
| **Reduced Alt** | Second reduced rate | FR: 10%, BE: 12%, CZ: 10% |
| **Super Reduced** | Below 5%, only grandfathered countries | ES: 4%, FR: 2.1%, IE: 4.8% |
| **Parking** | Transitional rate (12%+), limited countries | BE: 12%, IE: 13.5%, LU: 12% |
| **Zero** | 0% with right of deduction | Some basic necessities |
| **Exempt** | No VAT charged, no input deduction | Medical, financial services |

Not all countries have all rate types. Denmark has only a standard rate (25%). Spain has standard + reduced + super_reduced. The system stores all types per country, with `null` for types the country doesn't use.

### Data Sources for VAT Rates

**Primary: European Commission TEDB SOAP Service**
- Endpoint: `https://ec.europa.eu/taxation_customs/tedb/ws/VatRetrievalService.wsdl`
- Action: `RetrieveVatRates` — returns rates per member state, filterable by date
- Official EU source, maintained by member states themselves
- SOAP/XML — requires XML parsing in Go

**Fallback: euvatrates.com JSON (MIT licensed)**
- Endpoint: `https://euvatrates.com/rates.json`
- Simple JSON with standard_rate, reduced_rate, reduced_rate_alt, super_reduced_rate, parking_rate per country
- Community-maintained, sourced from EC data
- Used as fallback if TEDB is unavailable

**VAT Number Validation: VIES (official EU service)**
- Endpoint: `https://ec.europa.eu/taxation_customs/vies/checkVatService.wsdl`
- Validates EU VAT numbers against national databases in real-time
- Returns: valid/invalid, company name, address
- Used for B2B reverse charge eligibility

### VAT Rate Sync Schedule

```
ON APPLICATION STARTUP:
  1. Load VAT rates from database (cached from last sync)
  2. If no rates in DB (first run) OR last_synced > 24h ago:
     a. Try EC TEDB SOAP service -> parse XML -> update DB
     b. If TEDB fails: try euvatrates.com/rates.json -> parse JSON -> update DB
     c. If both fail: log error, continue with cached DB rates (or hardcoded seed)
  3. Load rates into in-memory cache (map[country_code]VATRates)

DAILY AT MIDNIGHT (UTC):
  1. Same sync logic as startup step 2
  2. Compare fetched rates with DB rates
  3. If any rate changed:
     a. Update DB with new rates + timestamp
     b. Refresh in-memory cache
     c. Log the change with before/after values
     d. Create AdminAuditLog entry: "vat_rates.auto_updated"
  4. If sync fails: log warning, keep existing rates, retry in 1 hour (max 3 retries)

MANUAL TRIGGER (admin):
  Admin can trigger a rate sync from Settings > VAT > "Sync VAT Rates Now"
  Same logic as above, with immediate feedback in the admin UI.
```

### Database Schema — VAT & Country Configuration

```
StoreSettings (single row or key-value settings table)
  ...existing fields...
  vat_enabled (bool, default false) -- master toggle
  vat_number (text, nullable) -- store's own VAT number (e.g., "ES12345678A")
  vat_country_code (text) -- store's country (ISO 3166-1 alpha-2, e.g., "ES")
  vat_prices_include_vat (bool, default true) -- prices entered with VAT included
  vat_default_category (text, default "standard") -- default VAT category for new products
  vat_b2b_reverse_charge_enabled (bool, default true) -- enable B2B reverse charge
  ...

EUCountry (reference table — all 27 EU member states)
  country_code (PK, text) -- ISO 3166-1 alpha-2 (DE, FR, ES, etc.)
  name (text) -- "Germany", "France", "Spain"
  local_vat_name (text) -- "MwSt.", "TVA", "IVA"
  local_vat_abbreviation (text) -- "MwSt.", "TVA", "IVA"
  is_eu_member (bool, default true) -- for future non-EU support
  currency (text, default "EUR") -- some EU countries use EUR, some don't

StoreShippingCountry (which countries the store sells/ships to)
  country_code (FK -> EUCountry)
  is_enabled (bool, default false)
  shipping_zone_id (FK -> ShippingZone, nullable -- for zone-based shipping rates)
  position (int -- display ordering)
  UNIQUE(country_code)
  -- Only enabled countries appear in storefront checkout country selector
  -- Only enabled countries are considered for VAT calculations

VATRate (synced from EC TEDB / euvatrates.com)
  id (uuid)
  country_code (FK -> EUCountry)
  rate_type (enum: standard, reduced, reduced_alt, super_reduced, parking, zero)
  rate (decimal) -- percentage, e.g., 21.00
  description (text, nullable) -- e.g., "Standard rate", "Reduced rate for food"
  valid_from (date) -- when this rate became effective
  valid_to (date, nullable) -- null = currently active
  source (text) -- "ec_tedb", "euvatrates_json", "manual", "seed"
  synced_at (timestamptz)
  UNIQUE(country_code, rate_type, valid_from)
  -- Historical rates kept for order reference

VATCategory (defines what "type" of VAT a product falls under)
  id (uuid)
  name (text, unique) -- "standard", "reduced", "reduced_alt", "super_reduced", "zero", "exempt"
  display_name (text) -- "Standard Rate", "Reduced Rate", "Zero Rate"
  description (text) -- "Default rate for most goods and services"
  maps_to_rate_type (text) -- which VATRate.rate_type this category maps to
  is_default (bool) -- one category is the system default
  position (int)
  -- These are the categories admins choose from when setting VAT on products.
  -- "standard" maps to VATRate where rate_type = "standard" for the destination country.
  -- "reduced" maps to VATRate where rate_type = "reduced", etc.

ProductVATOverride (per-product per-country VAT category override)
  id (uuid)
  product_id (FK -> Product)
  country_code (FK -> EUCountry)
  vat_category_id (FK -> VATCategory)
  notes (text, nullable) -- "Foodstuffs qualify for reduced rate in DE"
  created_at, updated_at
  UNIQUE(product_id, country_code)
  -- This is the key table for handling "this product is reduced-rate in France
  -- but standard-rate in Germany". Only create entries where the product DIFFERS
  -- from its default VAT category for a specific country.
```

**Also modify the Product table:**

```
Product (additions)
  ...existing fields...
  vat_category_id (FK -> VATCategory, nullable)
    -- Product-level VAT category. Overrides the store default.
    -- NULL = use store default VAT category.
    -- Can be further overridden per-country via ProductVATOverride.
```

### VAT Calculation Algorithm

```
function calculateVAT(product, destination_country_code, customer_vat_number):

    // Step 0: Is VAT enabled?
    if NOT store.vat_enabled:
        return { rate: 0, amount: 0, exempt_reason: "vat_disabled" }

    // Step 1: Check B2B reverse charge
    if store.vat_b2b_reverse_charge_enabled
       AND customer_vat_number IS NOT NULL
       AND destination_country_code != store.vat_country_code:

        // Validate via VIES
        vies_result = validateVIES(customer_vat_number)
        if vies_result.valid:
            return { rate: 0, amount: 0, exempt_reason: "reverse_charge",
                     customer_vat_number: customer_vat_number,
                     customer_company: vies_result.company_name }

    // Step 2: Determine VAT category for this product + destination
    vat_category = resolveVATCategory(product, destination_country_code)

    // Step 3: Look up the rate for destination country + category
    rate_type = vat_category.maps_to_rate_type
    vat_rate = lookupCurrentRate(destination_country_code, rate_type)

    if vat_rate IS NULL:
        // Country doesn't have this rate type (e.g., Denmark has no reduced rate)
        // Fall back to standard rate
        vat_rate = lookupCurrentRate(destination_country_code, "standard")

    // Step 4: Calculate VAT amount
    if store.vat_prices_include_vat:
        // Price includes VAT — extract VAT from the price
        // price = net + (net * rate/100)
        // net = price / (1 + rate/100)
        // vat = price - net
        net_price = product_price / (1 + vat_rate.rate / 100)
        vat_amount = product_price - net_price
    else:
        // Price is net — add VAT on top
        vat_amount = product_price * (vat_rate.rate / 100)

    return {
        rate: vat_rate.rate,
        rate_type: rate_type,
        amount: vat_amount,
        country_code: destination_country_code,
    }


function resolveVATCategory(product, country_code):
    // Priority: product+country override > product default > store default
    override = ProductVATOverride.find(product_id, country_code)
    if override:
        return override.vat_category

    if product.vat_category_id IS NOT NULL:
        return product.vat_category

    return store.default_vat_category
```

### VAT on Orders — What Gets Stored

When an order is placed, the VAT calculation is **snapshotted** on the order. This is critical because rates can change, and the order must always reflect what was charged at the time.

```
Order (additions to existing schema)
  ...existing fields...
  vat_number (text, nullable) -- customer's VAT number if B2B
  vat_company_name (text, nullable) -- from VIES validation
  vat_reverse_charge (bool, default false)
  vat_country_code (text) -- destination country used for VAT calc
  vat_total (decimal) -- total VAT amount on the order

OrderItem (additions)
  ...existing fields...
  vat_rate (decimal) -- snapshot: the rate applied (e.g., 21.00)
  vat_rate_type (text) -- snapshot: "standard", "reduced", etc.
  vat_amount (decimal) -- snapshot: VAT amount for this line
  price_includes_vat (bool) -- snapshot: whether price was VAT-inclusive
  net_unit_price (decimal) -- price without VAT
  gross_unit_price (decimal) -- price with VAT
```

### VIES VAT Number Validation

```
function validateVIES(vat_number):
    // Parse: first 2 chars = country code, rest = number
    country_code = vat_number[0:2]
    number = vat_number[2:]

    // Call EC VIES SOAP service
    response = callVIES(country_code, number)

    // Cache result for 24 hours (avoid hammering VIES)
    cacheVIESResult(vat_number, response, ttl=24h)

    return {
        valid: response.valid,
        company_name: response.name,
        company_address: response.address,
        consultation_number: response.requestIdentifier,
    }

VIESValidationCache (database table)
  vat_number (text, PK)
  is_valid (bool)
  company_name (text, nullable)
  company_address (text, nullable)
  consultation_number (text, nullable) -- for audit
  validated_at (timestamptz)
  expires_at (timestamptz) -- validated_at + 24h
```

### Admin UI — VAT Configuration

```
Settings > VAT

┌─ VAT Configuration ──────────────────────────────────────────────┐
│                                                                   │
│  VAT Enabled: [x] Yes                                            │
│  (Disable if you are not a VAT-registered business)              │
│                                                                   │
│  Store VAT Number: [ES12345678A        ] [Validate via VIES]     │
│  Store Country:    [Spain (ES)     ▼]                            │
│                                                                   │
│  Prices Include VAT: (●) Yes, entered prices include VAT         │
│                      ( ) No, entered prices are net               │
│                                                                   │
│  Default VAT Category: [Standard Rate ▼]                         │
│  (Applied to all products unless overridden)                      │
│                                                                   │
│  B2B Reverse Charge: [x] Enable for valid EU VAT numbers         │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘

┌─ Selling Countries ──────────────────────────────────────────────┐
│                                                                   │
│  Select which EU countries you sell and ship to:                  │
│                                                                   │
│  [x] Austria (AT)        [ ] Latvia (LV)                         │
│  [x] Belgium (BE)        [ ] Lithuania (LT)                      │
│  [ ] Bulgaria (BG)       [x] Luxembourg (LU)                     │
│  [ ] Croatia (HR)        [ ] Malta (MT)                          │
│  [ ] Cyprus (CY)         [x] Netherlands (NL)                    │
│  [ ] Czech Republic (CZ) [ ] Poland (PL)                         │
│  [x] Denmark (DK)        [x] Portugal (PT)                      │
│  [ ] Estonia (EE)        [ ] Romania (RO)                        │
│  [ ] Finland (FI)        [ ] Slovakia (SK)                       │
│  [x] France (FR)         [ ] Slovenia (SI)                       │
│  [x] Germany (DE)        [x] Spain (ES) ← Your country          │
│  [ ] Greece (GR)         [ ] Sweden (SE)                         │
│  [ ] Hungary (HU)                                                │
│  [x] Ireland (IE)        [Select All] [Deselect All]            │
│  [x] Italy (IT)                                                  │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘

┌─ Current VAT Rates ──────────────────────── [Sync Now] ──────────┐
│                                                                   │
│  Last synced: 2026-02-12 00:00:15 UTC (source: EC TEDB)         │
│                                                                   │
│  Country     Standard  Reduced  Red.Alt  SuperRed  Parking       │
│  Austria      20.0%    10.0%    13.0%     —        12.0%         │
│  Belgium      21.0%    12.0%     6.0%     —        12.0%         │
│  Germany      19.0%     7.0%     —        —         —            │
│  Spain        21.0%    10.0%     —        4.0%      —            │
│  France       20.0%    10.0%     5.5%     2.1%      —            │
│  ...                                                              │
│                                                                   │
│  Showing rates for enabled selling countries only.                │
│  Rates are synced automatically every midnight (UTC).             │
│  Manual edits are overwritten on next sync unless rate source     │
│  is set to "manual".                                             │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

### Admin UI — Product VAT Settings

On the product edit form (Details tab or dedicated VAT tab):

```
┌─ Product VAT Settings ───────────────────────────────────────────┐
│                                                                   │
│  VAT Category: [Standard Rate ▼]                                 │
│  (Store default is "Standard Rate". Override here if this         │
│   product needs a different rate in most countries.)              │
│                                                                   │
│  Country-Specific Overrides:                        [+ Add]      │
│                                                                   │
│  Country          VAT Category       Notes              Actions  │
│  France (FR)      Reduced Rate       Food products       [x]    │
│  Germany (DE)     Reduced Rate       Qualifying goods    [x]    │
│                                                                   │
│  (Only needed when a product has a different VAT category in     │
│   specific countries. Most products won't need overrides.)       │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

### Storefront Checkout — VAT Display

```
Cart / Checkout Summary:

  Leather Messenger Bag (Black/Large)    x1    €104.00
  Waxed Canvas Tote                      x2    €138.00
  ─────────────────────────────────────────────────────
  Subtotal (incl. VAT)                         €242.00
  VAT (21% IVA)                                 €41.98
  Shipping                                      €8.50
  ─────────────────────────────────────────────────────
  Total                                        €250.50

  Shipping to: [Spain (ES) ▼]  ← only enabled countries shown

  ── B2B Purchase? ──────────────────────────────────
  EU VAT Number: [                ] [Validate]
  (Enter your VAT number for reverse charge on
   intra-EU B2B purchases)
  ───────────────────────────────────────────────────

When VAT number validated and reverse charge applies:

  Subtotal (net)                               €200.02
  VAT (Reverse Charge — 0%)                      €0.00
  Shipping                                      €8.50
  ─────────────────────────────────────────────────────
  Total                                        €208.52
  Note: VAT reverse charge applies. Buyer is liable
  for VAT in their country of registration.
```

---

## Domain Model — Products, Attributes, Variants, BOM

(Unchanged from previous version — included here for completeness)

### Conceptual Overview

Products have arbitrary attribute axes (Color, Size, Material) that combine into purchasable variants. Materials are tracked through a layered BOM.

```
PRODUCT: Leather Messenger Bag
  ATTRIBUTES: Color [Black, Tan, Brown], Size [Standard, Large]
  VARIANTS: Black/Standard, Black/Large, Tan/Standard, Tan/Large, Brown/Standard, Brown/Large

  LAYERED BOM:
    Layer 1 (Product-level, ALL variants): 1x Brass buckle, 3m Thread, 1x Magnetic clasp
    Layer 2 (Per attribute option):
      Color=Black -> +0.5 m2 Black leather, +1x Black dye
      Color=Tan   -> +0.5 m2 Tan leather
      Size=Large  -> +1x Wide strap; MODIFIERS: leather x1.4, thread x1.3
    Layer 3 (Variant overrides, rare): Brown/Large -> REPLACE brass buckle with antique buckle
```

### Core Entities

```
Product
  id (uuid), name, slug, description, short_description
  status (draft | active | archived), sku_prefix
  base_price (decimal), compare_at_price (nullable)
  vat_category_id (FK -> VATCategory, nullable -- NULL = store default)
  base_weight_grams (int), base_dimensions_mm (JSONB)
  shipping_extra_fee_per_unit (decimal, default 0)
  has_variants (bool), seo_title, seo_description, metadata (JSONB)
  created_at, updated_at

ProductAttribute
  id, product_id (FK), name, display_name
  attribute_type (select | color_swatch | button_group | image_swatch)
  position, affects_pricing (bool), affects_shipping (bool)
  UNIQUE(product_id, name)

ProductAttributeOption
  id, attribute_id (FK), value, display_value
  color_hex (nullable), image_id (nullable)
  price_modifier (decimal, nullable), weight_modifier_grams (int, nullable)
  position, is_active (bool)
  UNIQUE(attribute_id, value)

ProductVariant
  id, product_id (FK), sku (unique)
  price (nullable -- NULL = calculated), compare_at_price (nullable)
  weight_grams (nullable -- NULL = calculated), dimensions_mm (nullable)
  stock_quantity (int), low_stock_threshold (int), barcode (nullable)
  is_active (bool), position
  -- Simple products: one variant, no linked options

ProductVariantOption (junction)
  variant_id, attribute_id, option_id
  UNIQUE(variant_id, attribute_id)

ProductVATOverride (per-product per-country VAT)
  id, product_id (FK), country_code (FK -> EUCountry)
  vat_category_id (FK -> VATCategory)
  notes (nullable)
  UNIQUE(product_id, country_code)

ProductImage
  id, product_id (FK), variant_id (nullable), option_id (nullable)
  url, alt_text, position, is_primary
  -- Priority: variant > option > product-level
```

**Variant generation**: Cartesian product of active options. Auto-SKU from prefix + abbreviations. Preserve existing variants on regeneration.

**Effective price**: `variant.price ?? (product.base_price + SUM(option.price_modifier))`
**Effective weight**: `variant.weight_grams ?? (product.base_weight_grams + SUM(option.weight_modifier_grams))`

### Raw Materials & Inventory

```
RawMaterial
  id, name, sku, description, category_id (FK), unit_of_measure (enum)
  cost_per_unit, stock_quantity (decimal), low_stock_threshold
  supplier_name, supplier_sku, lead_time_days, metadata, is_active

RawMaterialCategory: id, name, slug, parent_id, position
RawMaterialAttribute: id, raw_material_id, attribute_name, attribute_value
  UNIQUE(raw_material_id, attribute_name)

StockMovement (audit all inventory changes)
  id, entity_type (product_variant | raw_material), entity_id
  movement_type (purchase | sale | adjustment | production_consume | production_output | return | damage)
  quantity_change, quantity_before, quantity_after
  reference_type, reference_id, unit_cost, notes, created_by, created_at
```

### Layered BOM

```
Resolution order:
  1. Product BOM entries (common)
  2a. Apply option quantity modifiers (multiply/add/set on product entries)
  2b. Add option additional materials
  3. Apply variant overrides (replace/add/remove/set_quantity)

ProductBOMEntry (Layer 1): id, product_id, raw_material_id, quantity, uom, is_required, notes
  UNIQUE(product_id, raw_material_id)

AttributeOptionBOMEntry (Layer 2a): id, option_id, raw_material_id, quantity, uom, notes
  UNIQUE(option_id, raw_material_id)

AttributeOptionBOMModifier (Layer 2b): id, option_id, product_bom_entry_id
  modifier_type (multiply | add | set), modifier_value, notes
  UNIQUE(option_id, product_bom_entry_id)
  -- Applied in attribute position order for deterministic results

VariantBOMOverride (Layer 3): id, variant_id, raw_material_id
  override_type (replace | add | remove | set_quantity)
  replaces_material_id (nullable), quantity (nullable), uom, notes
```

**Producibility**: For each variant, resolve BOM, then `min(floor(stock / required))` across all materials.

### Orders, Customers, Discounts, Shipping, Admin

```
Order
  id, order_number (sequential), customer_id (nullable)
  status, email, billing_address (JSONB), shipping_address (JSONB)
  subtotal, shipping_fee, shipping_extra_fees, discount_amount
  vat_total (decimal), total
  vat_number (nullable), vat_company_name (nullable), vat_reverse_charge (bool)
  vat_country_code (text)
  stripe_payment_intent_id, stripe_checkout_session_id, payment_status
  discount_id, coupon_id, discount_breakdown (JSONB)
  shipping_method, tracking_number, shipped_at, delivered_at
  notes, customer_notes, metadata, created_at, updated_at

OrderItem
  id, order_id, product_id, variant_id
  product_name, variant_name, variant_options (JSONB snapshot)
  sku, quantity, unit_price, total_price
  vat_rate (decimal snapshot), vat_rate_type (text), vat_amount (decimal)
  price_includes_vat (bool), net_unit_price, gross_unit_price
  weight_grams, metadata

OrderEvent: id, order_id, event_type, from_status, to_status, data (JSONB), created_by, created_at

Customer: id, email, first_name, last_name, phone, password_hash
  default_billing/shipping_address, accepts_marketing, stripe_customer_id
  vat_number (nullable -- for B2B customers), notes, metadata

Discount
  id, name, type (percentage | fixed_amount), value, scope (subtotal | shipping | total)
  minimum_amount, maximum_discount, starts_at, ends_at, is_active, priority, stackable
  conditions (JSONB), created_at, updated_at

Coupon: id, code (unique), discount_id (FK), usage_limit, usage_limit_per_customer
  usage_count, starts_at, ends_at, is_active

ShippingConfig (global)
  enabled (bool), calculation_method (fixed | weight_based | size_based)
  fixed_fee, weight_rates (JSONB brackets), size_rates (JSONB)
  free_shipping_threshold, default_currency (default "EUR")

ShippingZone (optional, for per-zone rates)
  id, name (e.g., "Iberian Peninsula", "Central Europe", "Nordics")
  countries (text[] -- country codes in this zone)
  calculation_method, rates (JSONB), position

AdminUser: id, email, name, password_hash (bcrypt), role, permissions (text[])
  totp_secret, totp_verified, recovery_codes, force_2fa_setup, is_active

AdminAuditLog: id, admin_user_id, action, entity_type, entity_id, changes (JSONB), ip_address, created_at

Category: id, name, slug, description, parent_id, position, image_id, seo fields, is_active
ProductCategory (many-to-many): product_id, category_id, position
```

---

## Shipping — EU Country Restrictions

The shipping system is country-aware:

1. Admin selects which EU countries the store ships to (StoreShippingCountry)
2. Only enabled countries appear in storefront checkout country dropdown
3. Shipping calculation considers the destination country
4. Optional: shipping zones group countries with similar rates

```
Shipping fee calculation:
  1. Check destination country is enabled -> reject if not
  2. Base fee: fixed | weight_based (bracket) | size_based (volumetric)
     -- If shipping zones enabled, use zone-specific rates for destination country
  3. Per-product extra: SUM(product.shipping_extra_fee_per_unit * qty)
  4. Total = base + extra
  5. Apply free shipping threshold
  6. Apply shipping discounts
```

---

## Discount Engine

1. Collect active, date-valid discounts
2. Filter by conditions (min amount, products, categories, customer)
3. Evaluate by scope: subtotal -> shipping -> total
4. Stackable discounts in priority order; stop at first non-stackable
5. Coupons only if code provided and valid
6. Discount operates on net amounts (before VAT) or gross (including VAT) based on store setting
7. Record breakdown on order

---

## Reports & Predictions

### Sales Reports
- Current month: daily bar chart, cumulative line, comparison overlays
- Metrics: revenue (net and gross), VAT collected, order count, AOV, top products
- **VAT report**: total VAT collected per country per rate type (for tax filing)
- Export: CSV

### Prediction Algorithm
```
IF has_previous_year_data:
    prediction = 0.6 * yoy_adjusted + 0.4 * weighted_moving_average_28d
ELSE:
    prediction = weighted_moving_average_28d_day_of_week_adjusted
```

---

## API Design

All routes: `/api/v1/...`

### Public API

```
GET    /api/v1/products, /api/v1/products/:slug, /api/v1/products/:slug/variants
GET    /api/v1/categories
GET    /api/v1/countries              # Enabled selling countries (for checkout)

POST   /api/v1/cart, GET /api/v1/cart/:id
POST   /api/v1/cart/:id/items, PATCH/DELETE /api/v1/cart/:id/items/:item_id
POST   /api/v1/cart/:id/coupon, DELETE /api/v1/cart/:id/coupon
POST   /api/v1/cart/:id/vat-number    # Validate & apply B2B VAT number

POST   /api/v1/checkout                # Create Stripe session
POST   /api/v1/checkout/calculate      # Preview totals incl. VAT per country

POST   /api/v1/customers/register, login, GET /me, /me/orders

POST   /api/v1/webhooks/stripe
```

**Checkout calculate** accepts destination country and optional VAT number, returns:
- Subtotal (net), VAT breakdown per rate, shipping, discounts, total
- If B2B reverse charge: shows 0% VAT with explanation

### Admin routes

```
POST   /admin/login, /admin/login/2fa, /admin/setup-2fa
GET    /admin/dashboard
CRUD   /admin/products (tabbed: Details, Attributes, Variants, BOM, Images, VAT, SEO)
CRUD   /admin/inventory/raw-materials
GET    /admin/orders, /admin/orders/:id
GET    /admin/reports/sales, /admin/reports/vat  # VAT report for filing
GET    /admin/settings/vat                        # VAT configuration
PUT    /admin/settings/vat                        # Update VAT settings
PUT    /admin/settings/countries                  # Enable/disable selling countries
POST   /admin/settings/vat/sync                   # Manual VAT rate sync
POST   /admin/settings/vat/validate-number        # Validate store's own VAT number

HTMX partials for product attributes, variants, BOM, VAT overrides, etc.
```

---

## Authentication & Sessions

**Customer**: JWT (15min access + 7day refresh httpOnly cookie). Guest checkout.
**Admin**: Session-based (httpOnly, secure, SameSite=Strict). PostgreSQL storage. 8hr sliding. 2FA mandatory. CSRF on mutations.

---

## Testing Strategy

### E2E (Playwright) — Critical Paths

1. Storefront: Browse -> variant select -> cart -> checkout (Stripe) -> confirmation
2. Storefront: Country selection at checkout, VAT display updates
3. Storefront: B2B VAT number entry -> reverse charge applied -> totals update
4. Storefront: Attempt to checkout to disabled country -> rejected
5. Admin: Login with 2FA (first setup + subsequent)
6. Admin: Create product with attributes, generate variants
7. Admin: Set product VAT category, add per-country overrides
8. Admin: Configure VAT settings (enable, country selection, default category)
9. Admin: VAT rate sync (manual trigger, verify rates updated)
10. Admin: BOM at all three layers, producibility preview
11. Admin: Process orders, discount/coupon flows
12. Admin: VAT report shows correct per-country breakdown
13. API: VAT calculation for different countries, rate types, reverse charge
14. API: Shipping to enabled vs disabled countries

### Go Tests

- Unit: VAT calculation engine (all scenarios), BOM resolution, shipping calc, discount engine, prediction
- Integration: handlers + real PostgreSQL (testcontainers)
- VAT sync: mock EC TEDB + euvatrates.com responses, test fallback logic
- VIES: mock SOAP responses, test caching

### CI Pipeline

```
1. Go unit + integration tests (testcontainers)
2. Build Go binary + Nuxt storefront
3. docker-compose up (API, storefront, PostgreSQL, stripe-mock)
4. Playwright E2E
5. Coverage report
```

---

## Coding Conventions

### Go
- Wrap errors: `fmt.Errorf("...: %w", err)`. Validate at handler. Structured errors.
- sqlc for queries, templ for templates. No globals, constructor injection, Context everywhere.
- Money: `shopspring/decimal` or integer cents. NEVER float64.
- VAT calculations: always use `decimal` type, round to 2 decimal places at the end.
- Logging: `slog`. Linting: `golangci-lint`.

### SQL / Migrations
- Numbered: `001_name.up.sql` + `.down.sql`. Never modify applied.
- Money: `NUMERIC(12,2)`. VAT rates: `NUMERIC(5,2)`. IDs: UUID. Timestamps: TIMESTAMPTZ.
- Index FKs and filter/sort columns.

### Nuxt / TypeScript
- Strict TypeScript. SSR. Variant picker: reactive. VAT display: updates on country change.

### Playwright
- Page Object Model. API-based data setup. Clean state per test. `@critical` tags.

---

## Configuration

```env
PORT=8080
ADMIN_PORT=8081
BASE_URL=https://store.example.com
ADMIN_URL=https://admin.store.example.com
DATABASE_URL=postgres://...
JWT_SECRET=..., SESSION_SECRET=..., TOTP_ISSUER=ForgeCommerce
STRIPE_SECRET_KEY=sk_test_..., STRIPE_WEBHOOK_SECRET=whsec_..., STRIPE_PUBLIC_KEY=pk_test_...
MEDIA_STORAGE=local, MEDIA_PATH=./media
SMTP_HOST=localhost, SMTP_PORT=1025, SMTP_FROM=store@example.com

# VAT
VAT_SYNC_ENABLED=true                # Enable automatic VAT rate sync
VAT_SYNC_CRON=0 0 * * *              # Midnight UTC daily
VAT_TEDB_TIMEOUT=30s                 # EC TEDB SOAP service timeout
VAT_EUVATRATES_FALLBACK_URL=https://euvatrates.com/rates.json
VIES_TIMEOUT=10s                     # VIES validation timeout
VIES_CACHE_TTL=24h                   # Cache validation results
```

---

## Security Checklist

- [ ] Admin routes behind auth + 2FA middleware
- [ ] CSRF on all admin mutations
- [ ] Rate limiting (login, API, checkout, VIES validation)
- [ ] SQL injection prevention (sqlc/pgx parameterized)
- [ ] XSS prevention (templ auto-escapes, CSP)
- [ ] CORS for storefront origin only
- [ ] Stripe webhook signature verification
- [ ] bcrypt (cost 12+), TOTP secrets encrypted at rest
- [ ] Admin audit log for all writes
- [ ] Session fixation prevention, secure cookies
- [ ] File upload validation (type, size, magic bytes)
- [ ] Variant stock validation at checkout (race condition prevention)
- [ ] VAT number input sanitization (strip spaces, uppercase)
- [ ] VIES response validation (don't trust raw SOAP responses blindly)
- [ ] Country restriction enforcement at API level (not just UI)
- [ ] VAT rate changes logged with audit trail
