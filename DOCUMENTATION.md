# ForgeCommerce Admin Documentation

> Complete guide for shop administrators.
> Covers product setup, attributes, variants, VAT, orders, and day-to-day operations.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Products](#products)
3. [Attributes & Variants](#attributes--variants)
4. [Global Attribute Templates](#global-attribute-templates)
5. [Bill of Materials (BOM)](#bill-of-materials-bom)
6. [Product Images](#product-images)
7. [Categories](#categories)
8. [Raw Materials & Inventory](#raw-materials--inventory)
9. [Orders](#orders)
10. [Customers](#customers)
11. [Discounts & Coupons](#discounts--coupons)
12. [Shipping Configuration](#shipping-configuration)
13. [VAT Configuration](#vat-configuration)
14. [Reports](#reports)
15. [Import / Export (CSV)](#import--export-csv)
16. [Webhooks](#webhooks)
17. [User Management](#user-management)

---

## Getting Started

### Logging In

1. Navigate to `https://admin.yourstore.com` (or `http://localhost:8081` in development)
2. Enter your email and password
3. On first login, you'll be prompted to set up **Two-Factor Authentication (2FA)**:
   - Scan the QR code with an authenticator app (Google Authenticator, Authy, etc.)
   - Enter the 6-digit code to verify
   - Save your recovery codes in a safe place
4. Subsequent logins require your 2FA code after password

### Admin Dashboard

The dashboard shows:
- **Today's sales** (revenue, order count)
- **Recent orders** (last 10)
- **Low stock alerts** (products and raw materials below threshold)
- **Quick links** to common tasks

### Navigation

The left sidebar provides access to all admin sections:
- Dashboard
- Products
- Categories
- Raw Materials
- Orders
- Customers
- Reports
- Settings (VAT, Shipping, Countries, Global Attributes)
- Discounts
- Import/Export
- Webhooks

---

## Products

### Creating a Product

1. Go to **Products** and click **Create Product**
2. Fill in the **Details** tab:
   - **Name**: Product title displayed to customers
   - **SKU Prefix**: Used for auto-generating variant SKUs (e.g., `BAG`)
   - **Base Price**: Starting price in EUR (before any variant modifiers)
   - **Compare At Price**: Optional "was" price for showing discounts
   - **Status**: Draft (not visible), Active (on sale), Archived (hidden)
   - **Description**: Full product description (supports rich text)
   - **Short Description**: Summary shown in product listings
3. Click **Save**

### Product Tabs

Each product has multiple tabs for different aspects:

| Tab | Purpose |
|-----|---------|
| **Details** | Name, price, description, status, SEO |
| **Attributes** | Define variant axes (Color, Size, etc.) |
| **Variants** | Generated purchasable SKUs with stock |
| **BOM** | Raw materials needed to produce this product |
| **Images** | Product photos, variant-specific images |
| **VAT** | Tax category and per-country overrides |
| **SEO** | Meta title, description for search engines |

---

## Attributes & Variants

### Understanding Attributes

**Attributes** define the axes along which a product varies. Common examples:
- **Color**: Black, Brown, Tan
- **Size**: Small, Medium, Large
- **Material**: Cotton, Wool, Silk

Each attribute has **options** — the specific values a customer can choose.

### Types of Attributes

| Type | Display | Best For |
|------|---------|----------|
| **Select** | Dropdown menu | Sizes, phone models, materials |
| **Color Swatch** | Colored circles | Colors (requires hex code) |
| **Button Group** | Clickable buttons | Sizes, styles |
| **Image Swatch** | Thumbnail images | Patterns, materials |

### Two Ways to Define Attributes

ForgeCommerce supports two approaches:

#### 1. Custom Attributes (Product-Specific)

Define attributes directly on a product. These are unique to that product.

**When to use**: One-off attributes that only apply to a single product.

**How to add**:
1. Go to product > **Attributes** tab
2. Click **+ Add Custom Attribute**
3. Enter the attribute name, display name, and type
4. Add options one by one with values and optional modifiers

#### 2. Global Attribute Templates (Shared)

Define attributes once in **Settings > Global Attributes**, then link them to multiple products.

**When to use**: Attributes shared across many products (colors, sizes, phone models).

**How to add to a product**:
1. Go to product > **Attributes** tab
2. Click **Link Global Attribute**
3. Select the global attribute from the dropdown
4. Assign a **role name** (e.g., "Base Color" vs "Interior Color" when using the same color palette twice)
5. Optionally filter which options apply to this product

See [Global Attribute Templates](#global-attribute-templates) for full details.

### Option Modifiers

Options can modify the base product price and weight:

- **Price Modifier**: Added to the base price (e.g., +EUR 5 for "Large")
- **Weight Modifier**: Added to the base weight in grams (e.g., +200g for "Large")

These modifiers stack when multiple attributes have them.

### Generating Variants

Once you've defined attributes with options, ForgeCommerce can automatically generate all variant combinations:

1. Go to product > **Variants** tab
2. Click **Generate Variants**
3. The system creates the Cartesian product of all active options

**Example**: Color [Black, Brown] x Size [S, M, L] = 6 variants:
- Black/S, Black/M, Black/L, Brown/S, Brown/M, Brown/L

**Important behaviors**:
- Existing variants are **never deleted** during regeneration
- Only new combinations are added
- Manual edits to price, stock, SKU are preserved
- Deactivated options are excluded from generation

### Managing Variants

After generation, each variant has:
- **SKU**: Auto-generated from prefix + option abbreviations (e.g., `BAG-BLA-LAR`)
- **Price**: NULL = calculated (base + modifiers), or set a manual override
- **Stock Quantity**: Current inventory level
- **Low Stock Threshold**: Alert level (default: 5)
- **Barcode**: Optional EAN/UPC code
- **Active**: Toggle visibility on storefront

### Effective Price Calculation

```
If variant has an explicit price set:
    Use that price directly

Otherwise (price is NULL):
    effective_price = product.base_price
                    + SUM(option.price_modifier for each selected option)
```

---

## Global Attribute Templates

### Overview

Global Attribute Templates let you define reusable attribute definitions that can be shared across multiple products. This is especially powerful when:

- Many products share the same color palette
- You need to track rich metadata (dimensions, certifications, material composition)
- You want consistency across your catalog (same "Forest Green" hex everywhere)
- Products use the same attribute in different roles (e.g., "Wool Color" as both "Base Color" and "Interior Color")

### Creating a Global Attribute

1. Go to **Settings > Global Attributes**
2. Click **Create Global Attribute**
3. Fill in:
   - **Name**: Internal identifier (lowercase, underscores OK, e.g., `wool_color_palette`)
   - **Display Name**: Human-readable label (e.g., "Wool Color")
   - **Description**: Help text for other admins
   - **Type**: Select, Color Swatch, Button Group, or Image Swatch
   - **Category**: Optional grouping (e.g., "color", "size", "device")

### Defining Metadata Fields

Metadata fields define **structured data** that each option can carry. This goes beyond simple labels — it lets options contain rich, queryable information.

**How to add metadata fields**:

1. On the global attribute edit page, scroll to **Metadata Fields**
2. Click **+ Add Field**
3. Define each field:

| Setting | Purpose | Example |
|---------|---------|---------|
| **Field Name** | Internal key | `width_mm` |
| **Display Name** | Admin-visible label | "Width (mm)" |
| **Field Type** | Data type | text, number, boolean, select, url |
| **Required** | Must be filled for every option | Yes/No |
| **Default Value** | Pre-filled for new options | `0` |
| **Select Options** | Allowed values (for select type) | `["hand_wash", "machine_wash"]` |
| **Help Text** | Tooltip for admins | "Measurement in millimeters" |

**Example: Phone Models metadata fields**:

| Field Name | Display | Type | Required |
|-----------|---------|------|----------|
| `width_mm` | Width (mm) | number | Yes |
| `height_mm` | Height (mm) | number | Yes |
| `depth_mm` | Depth (mm) | number | Yes |
| `has_magsafe` | MagSafe Compatible | boolean | No |
| `camera_layout` | Camera Layout | select | No |
| `release_year` | Release Year | number | No |

### Adding Options

Options are the actual values for the attribute. Each option can include metadata values:

1. Scroll to **Options** section
2. Click **+ Add Option**
3. Fill in:
   - **Value**: Internal identifier (e.g., `iphone_16_pro`)
   - **Display Value**: Customer-facing label (e.g., "iPhone 16 Pro")
   - **Color Hex**: For color swatches (e.g., `#228B22`)
   - **Metadata values**: Fill in each defined metadata field

**Example: Adding "iPhone 16 Pro" to Phone Models**:

```
Value:        iphone_16_pro
Display:      iPhone 16 Pro
Metadata:
  width_mm:       76.7
  height_mm:      159.9
  depth_mm:       8.25
  has_magsafe:    true
  camera_layout:  triple_diagonal
  release_year:   2024
```

### Linking to Products

Once a global attribute exists, link it to products:

1. Go to product > **Attributes** tab
2. Click **Link Global Attribute**
3. Select the attribute from the dropdown
4. Configure the link:

| Setting | Purpose | Example |
|---------|---------|---------|
| **Role Name** | Internal identifier for this usage | `base_color` |
| **Role Display Name** | What the customer sees | "Base Color" |
| **Affects Pricing** | Options can modify price | Yes/No |
| **Affects Shipping** | Options can modify weight | Yes/No |
| **Price Modifier Field** | Which metadata field contains price adjustment | `price_premium` |
| **Weight Modifier Field** | Which metadata field contains weight adjustment | `weight_grams` |

### Filtering Options Per Product

Not every product needs every option from a global attribute. You can select a subset:

1. After linking, you'll see all options with checkboxes
2. Check only the options that apply to this product
3. If no options are checked, **all active options** are included by default

**Example**: The "Wool Color Palette" has 7 colors, but your "Classic Phone Case" only comes in 3:
- [x] Charcoal
- [x] Rust
- [x] Forest Green
- [ ] Ocean Blue
- [ ] Cream
- [ ] Midnight Navy
- [ ] Dusty Rose

### Using the Same Attribute Multiple Times

A powerful feature: you can link the **same** global attribute multiple times with different roles.

**Example**: A wool phone case uses the same "Wool Color Palette" for three separate attributes:

| Role | Selected Options | Purpose |
|------|-----------------|---------|
| **Base Color** | 3 of 7 | Outer case color |
| **Interior Color** | All 7 | Interior lining color |
| **Strap Color** | All 7 | Optional strap color |

Each role becomes a separate axis for variant generation:
- 3 base colors x 7 interior colors x 7 strap colors = **147 variants**

### Variant Generation with Global Attributes

When you click **Generate Variants**, the system combines:
1. All **custom attributes** (product-specific)
2. All **linked global attributes** (with filtered options)

Both sources are treated equally in the Cartesian product generation. The resulting variants reference both product-specific and global option IDs.

### Changing Global Attribute Options

When you add or modify options in a global attribute:
- **Adding a new option**: Does NOT auto-regenerate variants. You must go to each linked product and click "Generate Variants" to include the new option.
- **Editing an option's display value or metadata**: Changes are reflected everywhere immediately (display names, metadata).
- **Deactivating an option**: Existing variants using that option remain, but new variant generation will exclude it.
- **Deleting an option**: Cascades to remove all variant-option links using it.

---

## Bill of Materials (BOM)

### Overview

The BOM system tracks which raw materials are needed to manufacture each product variant. It uses a three-layer system for maximum flexibility.

### Layer 1: Product-Level Materials

Materials common to **all variants** of a product.

**Example** for a leather bag:
- 1x Brass buckle
- 3m Waxed thread
- 1x Magnetic clasp

### Layer 2: Option-Specific Materials

#### 2a: Additional Materials

Extra materials needed when a specific option is selected.

**Example**: When Color = "Black":
- +0.5 m2 Black leather
- +1x Black dye packet

#### 2b: Quantity Modifiers

Modify quantities from Layer 1 based on option selection.

**Example**: When Size = "Large":
- Leather quantity x 1.4 (40% more material)
- Thread quantity x 1.3 (30% more)

Modifier types:
- **Multiply**: `base_quantity * modifier_value`
- **Add**: `base_quantity + modifier_value`
- **Set**: `modifier_value` (replaces base quantity)

### Layer 3: Variant Overrides

Exceptional cases where a specific variant combination has unique material needs.

**Example**: Brown/Large variant:
- **Replace** brass buckle with antique brass buckle
- **Add** extra reinforcement strap

Override types: replace, add, remove, set_quantity

### BOM Resolution Order

For any variant, the final material list is calculated:

1. Start with Product-level materials (Layer 1)
2. For each option selected in the variant (in attribute position order):
   - Add any option-specific materials (Layer 2a)
   - Apply quantity modifiers to existing materials (Layer 2b)
3. Apply variant-specific overrides (Layer 3)

### Producibility

The system calculates how many units of each variant can be produced based on current raw material stock:

```
producibility = MIN(floor(available_stock / required_quantity))
               across all required materials
```

---

## Product Images

### Uploading Images

1. Go to product > **Images** tab
2. Drag and drop files or click to browse
3. Supported formats: JPEG, PNG, WebP, GIF (max 10MB per file)
4. Upload multiple files at once

### Managing Images

- **Primary Image**: Click the star icon to set the main product image. The first uploaded image is automatically set as primary.
- **Alt Text**: Add descriptive text for accessibility and SEO
- **Reorder**: Drag and drop to change display order
- **Variant Assignment**: Link an image to a specific variant using the dropdown
- **Delete**: Remove images you no longer need

### Image Priority

Images are displayed in this priority order:
1. **Variant-specific images** (linked to a particular variant)
2. **Option-specific images** (linked to an attribute option)
3. **Product-level images** (general product photos)

---

## Categories

### Creating Categories

1. Go to **Categories**
2. Click **Create Category**
3. Fill in: Name, Description, Parent Category (optional for nesting)
4. Set position for display ordering

### Category Hierarchy

Categories support unlimited nesting:
- Clothing
  - Men's Clothing
    - T-Shirts
    - Jackets
  - Women's Clothing
    - Dresses
    - Blouses

### Assigning Products to Categories

On the product **Details** tab, select one or more categories.

---

## Raw Materials & Inventory

### Managing Raw Materials

1. Go to **Raw Materials**
2. Click **Create Raw Material**
3. Fill in:
   - **Name & SKU**: Identification
   - **Category**: Material type (Fabric, Thread, Hardware, etc.)
   - **Unit of Measure**: meters, square_meters, grams, pieces, liters
   - **Cost Per Unit**: Purchase cost
   - **Stock Quantity**: Current inventory level
   - **Low Stock Threshold**: Alert level
   - **Supplier**: Name, SKU, lead time

### Stock Movements

All inventory changes are tracked with a full audit trail:
- **Purchase**: Stock received from supplier
- **Sale**: Stock consumed by order
- **Adjustment**: Manual correction
- **Production Consume**: Used in manufacturing
- **Production Output**: Finished goods created
- **Return**: Stock returned by customer
- **Damage**: Stock written off

---

## Orders

### Order Lifecycle

```
Pending → Confirmed → Processing → Shipped → Delivered
                                        └──→ Cancelled
```

### Viewing Orders

The orders list shows:
- Order number (sequential)
- Customer email
- Status
- Total (with VAT breakdown)
- Date

### Order Details

Each order shows:
- **Line items** with product name, SKU, quantity, price
- **VAT breakdown** per item (rate, amount, net/gross)
- **Shipping information** (method, tracking, fees)
- **Customer details** (billing/shipping address)
- **Payment status** (Stripe reference)
- **B2B details** (VAT number, company name if reverse charge)

### Processing Orders

1. Click on an order to view details
2. Update status as you fulfill it
3. Add tracking number when shipped
4. Add notes for internal reference

---

## Customers

### Customer Accounts

Customers can:
- Register accounts with email/password
- Save billing and shipping addresses
- View order history
- Store their VAT number for B2B purchases

### Guest Checkout

Customers can also check out without creating an account.

---

## Discounts & Coupons

### Creating Discounts

1. Go to **Discounts**
2. Click **Create Discount**
3. Configure:
   - **Type**: Percentage or Fixed Amount
   - **Value**: e.g., 10% or EUR 5
   - **Scope**: Subtotal, Shipping, or Total
   - **Minimum Amount**: Minimum order value to qualify
   - **Maximum Discount**: Cap on discount value
   - **Date Range**: When the discount is active
   - **Priority**: Order of application (lower = first)
   - **Stackable**: Whether it can combine with other discounts

### Creating Coupon Codes

1. On the discount page, create a **Coupon**
2. Set a unique code (e.g., `SUMMER20`)
3. Configure usage limits:
   - **Total Usage Limit**: Maximum uses across all customers
   - **Per Customer Limit**: Maximum uses per customer
   - **Date Range**: Validity period

### How Discounts Apply

1. Active discounts are collected and filtered by conditions
2. Applied in priority order within each scope (subtotal, shipping, total)
3. Stackable discounts accumulate; non-stackable stops the chain
4. Coupon discounts only apply if customer enters the code

---

## Shipping Configuration

### Global Shipping Settings

Go to **Settings > Shipping** to configure:

- **Calculation Method**:
  - **Fixed Fee**: Same shipping cost for all orders
  - **Weight-Based**: Rates by weight brackets
  - **Size-Based**: Rates by volumetric dimensions
- **Free Shipping Threshold**: Orders above this amount ship free

### Shipping Zones

Group countries with similar shipping rates:

1. Create zones (e.g., "Iberian Peninsula", "Central Europe", "Nordics")
2. Assign countries to each zone
3. Set zone-specific rates

### Per-Product Extra Fees

Products with unusual shipping requirements can add a per-unit surcharge in the product **Details** tab.

### Selling Countries

Go to **Settings > Countries** to enable/disable which EU countries you ship to. Only enabled countries appear in the storefront checkout country selector.

---

## VAT Configuration

### Overview

ForgeCommerce provides full EU VAT compliance:
- Automatic VAT rate sync from the European Commission
- Per-country rate application
- B2B reverse charge for valid EU VAT numbers
- VAT-inclusive or VAT-exclusive pricing modes
- Per-country VAT overrides for products in different tax categories

### Enabling VAT

1. Go to **Settings > VAT**
2. Enable VAT
3. Enter your store's VAT number (validated via VIES)
4. Set your store's country
5. Choose pricing mode:
   - **Prices include VAT**: Entered prices contain VAT (VAT is extracted for display)
   - **Prices are net**: VAT is added on top at checkout

### VAT Categories

Products are assigned to VAT categories that map to EU rate types:

| Category | Maps To | Use Case |
|----------|---------|----------|
| Standard | Standard rate | Most goods (19-27% depending on country) |
| Reduced | Reduced rate | Food, books, medical supplies |
| Reduced Alt | 2nd reduced rate | Country-specific reduced categories |
| Super Reduced | Super reduced rate | Basic necessities (select countries) |
| Zero | Zero rate | Exports, specific exemptions |
| Exempt | Exempt | Medical, financial services |

### Per-Product VAT Overrides

Some products may have different VAT rates in different countries:

1. Go to product > **VAT** tab
2. Set the product's default VAT category
3. Add country-specific overrides where needed

**Example**: A food product is "Standard" in most countries but "Reduced" in France and Germany.

### B2B Reverse Charge

When enabled:
1. B2B customers enter their EU VAT number at checkout
2. The number is validated in real-time via the EU VIES service
3. If valid and the customer is in a different EU country than your store:
   - VAT is charged at 0%
   - The invoice notes "Reverse charge applies"
   - The customer is responsible for declaring VAT in their country

### VAT Rate Sync

VAT rates are automatically synced from the European Commission TEDB service:
- **Automatic**: Daily at midnight UTC
- **Manual**: Click "Sync Now" in Settings > VAT
- **Fallback**: If TEDB is unavailable, rates are fetched from euvatrates.com

---

## Reports

### Sales Reports

Go to **Reports > Sales** to view:
- **Daily revenue** chart (bar + cumulative line)
- **Comparison overlays** (this month vs last month, vs same month last year)
- **Key metrics**: Total revenue, order count, average order value
- **Top products** by revenue
- **Export to CSV** for external analysis

### VAT Report

Go to **Reports > VAT** to view:
- **VAT collected per country** per rate type
- Useful for quarterly/monthly VAT filing
- **Export to CSV** for submission to tax authorities

### Sales Predictions

The system predicts future sales using:
- **Weighted Moving Average** (28-day, day-of-week adjusted)
- **Year-over-Year comparison** (when historical data is available)
- Blended formula: 60% YoY-adjusted + 40% WMA (when YoY data exists)

---

## Import / Export (CSV)

### Exporting Data

Go to **Import/Export** to download CSV files for:
- Products (with all details)
- Raw Materials (with stock levels)
- Orders (with line items)

### Importing Data

Upload CSV files to bulk-create or update:
- Products
- Raw Materials

CSV format must match the export format. Review the preview before confirming import.

---

## Webhooks

### Overview

Webhooks notify external systems when events occur in your store. ForgeCommerce sends HTTP POST requests with JSON payloads to your configured endpoints.

### Setting Up Webhooks

1. Go to **Settings > Webhooks**
2. Click **Create Webhook**
3. Configure:
   - **URL**: The endpoint to receive events
   - **Events**: Which events trigger the webhook
   - **Secret**: Used to sign payloads for verification

### Available Events

- `order.created`, `order.updated`, `order.cancelled`
- `product.created`, `product.updated`, `product.deleted`
- `stock.low` (when stock falls below threshold)
- `customer.registered`

### Payload Verification

Each webhook includes an `X-Forge-Signature` header containing an HMAC-SHA256 signature. Verify this against your webhook secret to ensure the payload is authentic.

---

## User Management

### Admin Users

Go to **Settings > Users** to manage admin accounts:
- Create new admin users
- Set roles and permissions
- Require 2FA setup
- Deactivate accounts (without deleting)

### Security

- All admin actions are logged in the **Audit Log**
- 2FA is mandatory for all admin users
- Sessions expire after 8 hours of inactivity
- Login is rate-limited (5 attempts per minute) to prevent brute-force attacks
