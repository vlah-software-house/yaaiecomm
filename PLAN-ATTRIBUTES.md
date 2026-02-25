# Plan-Attributes.md — Global Attribute Templates & Metadata Schema

> Extension to ForgeCommerce's product attribute system.
> Adds reusable, store-wide attribute definitions with rich metadata,
> enabling massive variant generation with structured option data.

---

## Motivation

The current attribute system is **product-scoped**: each product defines its own attributes and options. This works for simple catalogs but breaks down when:

1. **Many products share the same attributes** (e.g., 50 wool products all need the same "Color Palette" with 7 colors)
2. **Options carry rich metadata** (e.g., a phone model needs dimensions, compatibility flags, camera layout info)
3. **Consistency matters** (e.g., "Forest Green" must be the same hex value everywhere)
4. **Combinatorial products** need structured data to drive pricing, BOM, and shipping calculations

### Real-World Example: Wool Phone Case Shop

A shop sells handmade wool phone cases with:
- **Base Wool** — 3 colors (from a palette of 7)
- **Interior Lining** — 7 colors (same palette)
- **Exterior Strap** — 7 colors (same palette, optional)
- **Phone Model** — 10+ models, each with physical dimensions

This creates **3 x 7 x 8 x 10 = 1,680 potential variants** per product.

Without global attributes:
- The admin must manually create "Color" 3 times per product (base, interior, strap)
- Phone model dimensions must be manually tracked outside the system
- Adding a new color requires editing every product individually

With global attributes:
- Define "Wool Color Palette" once with 7 colors
- Define "Phone Models" once with full dimension metadata
- Link to any product, selecting which options apply
- Add a new color → it's instantly available everywhere

---

## Implementation Scope

### Phase A: Global Attribute Templates [THIS ITERATION]

- [x] Database schema (migration 024)
- [x] SQL queries for CRUD operations
- [x] sqlc code generation
- [x] Global attribute service layer
- [x] Admin UI for managing global attributes
- [x] "Link Global Attribute" workflow on product attribute page
- [x] Variant generation from linked global attributes
- [x] Go unit tests
- [x] E2E Playwright tests

### Phase B: Rich Metadata Schema [THIS ITERATION]

- [x] Metadata schema definition table
- [x] JSONB metadata on global attribute options
- [x] Admin UI for defining metadata fields
- [x] Admin UI for entering metadata values per option
- [x] Metadata display in storefront API response

### Phase C: Attribute Constraints & Rules [FUTURE]

- [ ] Variant exclusion rules (e.g., "base_color != interior_color")
- [ ] Conditional pricing rules (e.g., "if phone > 160mm, +EUR 3")
- [ ] Optional/conditional attributes (e.g., strap is optional)
- [ ] Availability rules (date-gated options)

### Phase D: Advanced Features [FUTURE]

- [ ] Bulk option import (CSV) for global attributes
- [ ] Attribute groups / categories
- [ ] Cross-product variant matrix view
- [ ] Auto-BOM from metadata (e.g., calculate material from dimensions)

---

## Database Schema

### New Tables (Migration 024)

```sql
-- Global attribute definitions (store-wide, reusable across products)
CREATE TABLE global_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,            -- internal key: "wool_color_palette"
    display_name TEXT NOT NULL,           -- admin/customer label: "Wool Color"
    description TEXT,                     -- help text for admins
    attribute_type TEXT NOT NULL DEFAULT 'select'
        CHECK (attribute_type IN ('select', 'color_swatch', 'button_group', 'image_swatch')),
    category TEXT,                        -- grouping: "color", "size", "device", "material"
    position INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Metadata field definitions for a global attribute
-- Defines the STRUCTURE of metadata that options must conform to
CREATE TABLE global_attribute_metadata_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    global_attribute_id UUID NOT NULL REFERENCES global_attributes(id) ON DELETE CASCADE,
    field_name TEXT NOT NULL,             -- "width_mm", "fiber_composition"
    display_name TEXT NOT NULL,           -- "Width (mm)", "Fiber Composition"
    field_type TEXT NOT NULL DEFAULT 'text'
        CHECK (field_type IN ('text', 'number', 'boolean', 'select', 'url')),
    is_required BOOLEAN NOT NULL DEFAULT false,
    default_value TEXT,                   -- default for new options
    select_options TEXT[],                -- for field_type='select': allowed values
    help_text TEXT,                       -- tooltip for admins
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (global_attribute_id, field_name)
);

-- Options for global attributes (the actual values)
CREATE TABLE global_attribute_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    global_attribute_id UUID NOT NULL REFERENCES global_attributes(id) ON DELETE CASCADE,
    value TEXT NOT NULL,                  -- internal: "forest_green"
    display_value TEXT NOT NULL,          -- customer-facing: "Forest Green"
    color_hex TEXT,                       -- for color swatches: "#228B22"
    image_url TEXT,                       -- for image swatches
    metadata JSONB NOT NULL DEFAULT '{}', -- structured data per metadata fields
    position INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (global_attribute_id, value)
);

-- Links a product to a global attribute, with role and option filtering
CREATE TABLE product_global_attribute_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    global_attribute_id UUID NOT NULL REFERENCES global_attributes(id) ON DELETE CASCADE,
    role_name TEXT NOT NULL,              -- "Base Color", "Interior Color", "Phone Model"
    role_display_name TEXT NOT NULL,      -- customer-facing label
    position INTEGER NOT NULL DEFAULT 0,
    affects_pricing BOOLEAN NOT NULL DEFAULT false,
    affects_shipping BOOLEAN NOT NULL DEFAULT false,
    price_modifier_field TEXT,            -- metadata field to use as price modifier
    weight_modifier_field TEXT,           -- metadata field to use as weight modifier
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, global_attribute_id, role_name)
);

-- Which options from the global attribute are enabled for this product link
-- If NO rows exist for a link, ALL active options are included
CREATE TABLE product_global_option_selections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES product_global_attribute_links(id) ON DELETE CASCADE,
    global_option_id UUID NOT NULL REFERENCES global_attribute_options(id) ON DELETE CASCADE,
    price_modifier NUMERIC(12,2),         -- product-specific price override
    weight_modifier_grams INTEGER,        -- product-specific weight override
    position_override INTEGER,            -- product-specific ordering
    UNIQUE (link_id, global_option_id)
);
```

### How Variant Generation Changes

Currently variants are generated from `product_attributes` + `product_attribute_options`.

With the extension, the variant generator must ALSO consider `product_global_attribute_links`:

```
function resolveAttributeAxes(product_id):
    axes = []

    // 1. Product-specific attributes (existing behavior)
    for attr in product_attributes WHERE product_id:
        axes.append({
            source: "product",
            attribute_id: attr.id,
            name: attr.display_name,
            options: product_attribute_options WHERE attribute_id = attr.id AND is_active
        })

    // 2. Global attribute links (NEW)
    for link in product_global_attribute_links WHERE product_id ORDER BY position:
        selections = product_global_option_selections WHERE link_id = link.id
        if selections is empty:
            // Use ALL active global options
            options = global_attribute_options WHERE global_attribute_id AND is_active
        else:
            // Use only selected options
            options = global_attribute_options WHERE id IN selections.global_option_id

        axes.append({
            source: "global",
            link_id: link.id,
            global_attribute_id: link.global_attribute_id,
            name: link.role_display_name,
            options: options (with optional price/weight overrides from selections)
        })

    // Sort ALL axes by position
    return sort(axes, by=position)
```

The Cartesian product algorithm remains the same — it just operates on a unified list of axes.

### Variant-Option Junction for Global Attributes

The existing `product_variant_options` table links variants to `product_attribute_options`. For global attributes, we need to also link to `global_attribute_options`:

```sql
-- Extend variant generation to track global option links
CREATE TABLE product_variant_global_options (
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES product_global_attribute_links(id) ON DELETE CASCADE,
    global_option_id UUID NOT NULL REFERENCES global_attribute_options(id) ON DELETE CASCADE,
    UNIQUE (variant_id, link_id)
);
```

---

## Admin UI Design

### New Admin Section: Settings > Global Attributes

```
Settings > Global Attributes

[+ Create Global Attribute]

┌─────────────────────────────────────────────────────────────────────┐
│ Wool Color Palette                                    color_swatch │
│ 7 options | Used by 12 products | Category: color                 │
│                                                                     │
│ Options: Forest Green, Ocean Blue, Charcoal, Cream, Rust,         │
│          Midnight Navy, Dusty Rose                                  │
│                                                                     │
│ Metadata fields: fiber_type, pantone_code                          │
│                                                                     │
│ [Edit] [Duplicate]                                                 │
├─────────────────────────────────────────────────────────────────────┤
│ Phone Models                                              select   │
│ 10 options | Used by 3 products | Category: device                │
│                                                                     │
│ Options: iPhone 16 Pro, iPhone 16, iPhone 15 Pro, Pixel 9, ...    │
│                                                                     │
│ Metadata fields: width_mm, height_mm, depth_mm, has_magsafe,     │
│                  camera_layout, release_year                       │
│                                                                     │
│ [Edit] [Duplicate]                                                 │
└─────────────────────────────────────────────────────────────────────┘
```

### Global Attribute Edit Page

```
Edit Global Attribute: Wool Color Palette

Name:         [wool_color_palette     ]
Display Name: [Wool Color             ]
Description:  [Color palette for wool products                    ]
Type:         [Color Swatch       ▼]
Category:     [color              ]

── Metadata Fields ──────────────────────────── [+ Add Field] ──
│ Field Name    │ Display    │ Type   │ Required │ Actions │
│ fiber_type    │ Fiber Type │ text   │ No       │ [x]     │
│ pantone_code  │ Pantone    │ text   │ No       │ [x]     │

── Options ──────────────────────────────────── [+ Add Option] ──
│ Value         │ Display       │ Hex     │ fiber_type   │ pantone │ Active │
│ forest_green  │ Forest Green  │ #228B22 │ Merino       │ 17-6153 │ [x]    │
│ ocean_blue    │ Ocean Blue    │ #006994 │ Merino       │ 19-4052 │ [x]    │
│ charcoal      │ Charcoal      │ #36454F │ Merino       │ 19-3906 │ [x]    │
│ cream         │ Cream         │ #FFFDD0 │ Merino       │ 11-0604 │ [x]    │
│ rust          │ Rust          │ #B7410E │ Merino       │ 18-1248 │ [x]    │
│ midnight_navy │ Midnight Navy │ #003366 │ Merino       │ 19-3933 │ [x]    │
│ dusty_rose    │ Dusty Rose    │ #DCAE96 │ Alpaca blend │ 15-1516 │ [x]    │

[Save Changes]
```

### Product Attributes Tab — Enhanced

```
Product: Merino Wool Phone Case > Attributes

[Link Global Attribute ▼]    [+ Add Custom Attribute]
  ├─ Wool Color Palette
  ├─ Phone Models
  └─ (no more available)

── Linked Global Attributes ────────────────────────────────────────
┌─ Wool Color Palette as "Base Color" ──────────────────────────────┐
│ Role: [Base Color         ]                                       │
│ Pricing: [x] Affects pricing                                     │
│                                                                   │
│ Options (3 of 7 selected):                                       │
│ [x] Charcoal   [ ] Ocean Blue   [ ] Cream                       │
│ [x] Rust       [ ] Midnight Navy [ ] Dusty Rose                 │
│ [x] Forest Green                                                 │
│                                                                   │
│ [Unlink from product]                                             │
├───────────────────────────────────────────────────────────────────┤
│ Wool Color Palette as "Interior Color" ───────────────────────── │
│ Role: [Interior Color     ]                                       │
│ All 7 options enabled (no filtering)                              │
│ [Unlink from product]                                             │
├───────────────────────────────────────────────────────────────────┤
│ Phone Models as "Phone Model" ────────────────────────────────── │
│ Role: [Phone Model        ]                                       │
│ Weight modifier field: [weight_grams ▼]                          │
│ 10 of 10 options enabled                                          │
│ [Unlink from product]                                             │
└───────────────────────────────────────────────────────────────────┘

── Custom Attributes (product-specific) ────────────────────────────
  (none defined for this product)
```

---

## Public API Response

When a product has global attribute links, the API response includes them alongside product-specific attributes:

```json
{
  "id": "uuid",
  "name": "Merino Wool Phone Case",
  "attributes": [
    {
      "name": "base_color",
      "display_name": "Base Color",
      "source": "global",
      "attribute_type": "color_swatch",
      "global_attribute_id": "uuid",
      "options": [
        {
          "id": "uuid",
          "value": "forest_green",
          "display_value": "Forest Green",
          "color_hex": "#228B22",
          "metadata": {
            "fiber_type": "Merino",
            "pantone_code": "17-6153"
          },
          "price_modifier": 0.00
        }
      ]
    },
    {
      "name": "phone_model",
      "display_name": "Phone Model",
      "source": "global",
      "attribute_type": "select",
      "options": [
        {
          "id": "uuid",
          "value": "iphone_16_pro",
          "display_value": "iPhone 16 Pro",
          "metadata": {
            "width_mm": 76.7,
            "height_mm": 159.9,
            "depth_mm": 8.25,
            "has_magsafe": true,
            "camera_layout": "triple_diagonal",
            "release_year": 2024
          }
        }
      ]
    }
  ],
  "variants": [ ... ]
}
```

---

## Test Coverage Plan

### Go Unit Tests

1. **Global Attribute Service**
   - CRUD: create, read, update, delete global attributes
   - Option CRUD with metadata validation
   - Metadata field CRUD and validation
   - List with filtering by category
   - Duplicate detection (unique name)

2. **Product Link Service**
   - Link global attribute to product with role
   - Duplicate role detection
   - Option selection (subset vs all)
   - Price/weight modifier overrides
   - Unlink and cascade behavior

3. **Extended Variant Generation**
   - Generate from global-only attributes
   - Generate from mixed (global + product-specific)
   - Option filtering (subset selection)
   - SKU generation with global attribute values
   - Preserve existing variants on regeneration
   - Metadata passed through to variant context

4. **Metadata Validation**
   - Required field enforcement
   - Type validation (number, boolean, select enum)
   - Default value application
   - Invalid metadata rejection

### E2E Playwright Tests

1. **Admin: Global Attribute Management**
   - Create global attribute with metadata fields
   - Add/edit/delete options with metadata values
   - Edit metadata field definitions
   - Delete global attribute

2. **Admin: Product Global Attribute Linking**
   - Link global attribute to product
   - Set role name
   - Filter options (select subset)
   - Unlink global attribute
   - Generate variants from global attributes

3. **Storefront: Global Attribute Display**
   - Product page shows global attribute options
   - Variant picker works with global attributes
   - Metadata accessible in product data

---

## File Map

```
api/internal/database/migrations/
  024_global_attributes.up.sql       # Schema
  024_global_attributes.down.sql     # Rollback

api/internal/database/queries/
  global_attributes.sql              # sqlc queries

api/internal/database/gen/
  global_attributes.sql.go           # Generated code

api/internal/services/globalattr/
  service.go                         # Business logic

api/internal/handlers/admin/
  global_attributes.go               # Admin HTTP handlers

api/templates/admin/
  global_attributes.templ            # Templ templates

api/internal/services/variant/
  service.go                         # Modified: support global attrs in generation

api/internal/handlers/api/
  public.go                          # Modified: include global attrs in API response

docs/
  Plan-Attributes.md                 # This file
  DOCUMENTATION.md                   # Admin user guide
```
