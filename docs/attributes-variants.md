# Product Attributes, Variants & Bill of Materials

> How ForgeCommerce models configurable products with a layered BOM system
> designed for small manufacturers.

---

## Concepts

### Products

A **product** is the top-level item (e.g., "Leather Messenger Bag"). It has a
base price, weight, description, and optional attributes.

### Attributes

**Attributes** are the axes of variation (e.g., Color, Size, Material). Each
attribute has **options** — the specific values a customer can choose:

```
Product: Leather Messenger Bag
  Attribute: Color
    Options: Black, Tan, Brown
  Attribute: Size
    Options: Standard, Large
```

Attribute types control how options are displayed in the storefront:

| Type           | Display                          |
|---------------|----------------------------------|
| `select`      | Dropdown menu                    |
| `color_swatch`| Colored circles                  |
| `button_group`| Clickable buttons                |
| `image_swatch`| Small image thumbnails           |

### Variants

**Variants** are the purchasable combinations of attribute options. They are
generated as the Cartesian product of all active options:

```
Black/Standard, Black/Large,
Tan/Standard, Tan/Large,
Brown/Standard, Brown/Large
→ 6 variants total (3 colors x 2 sizes)
```

Each variant has its own SKU, stock quantity, and optional price/weight override.

### Simple Products

Products without attributes have a **single variant** (no linked options).
This is used for simple, non-configurable products.

---

## Pricing

### Effective Price

Variant pricing follows this priority:

```
1. variant.price (if explicitly set)
   ↓ (if null)
2. product.base_price + SUM(option.price_modifier for each selected option)
```

**Example:**
- Base price: EUR 99.00
- Color "Black": no modifier
- Size "Large": +EUR 15.00
- Black/Large effective price: EUR 114.00

### Effective Weight

Same logic applies to weight:

```
1. variant.weight_grams (if explicitly set)
   ↓ (if null)
2. product.base_weight_grams + SUM(option.weight_modifier_grams)
```

---

## SKU Generation

Variant SKUs are auto-generated from the product's `sku_prefix` and option
abbreviations:

```
Product SKU Prefix: LMB
Color Black → BLK, Tan → TAN
Size Standard → STD, Large → LRG

Generated SKUs: LMB-BLK-STD, LMB-BLK-LRG, LMB-TAN-STD, etc.
```

When regenerating variants (after adding/removing options), existing variants
are preserved by matching their option combinations.

---

## Bill of Materials (BOM)

The BOM system tracks which raw materials are needed to produce each variant.
It uses a **3-layer resolution** system:

### Layer 1: Product BOM (Common Materials)

Materials needed by **all variants** of a product.

```
Leather Messenger Bag:
  - 1x Brass buckle
  - 3m Thread
  - 1x Magnetic clasp
```

### Layer 2a: Option Additional Materials

Materials added when a specific option is selected.

```
Color = Black:
  + 0.5 m2 Black leather
  + 1x Black dye

Color = Tan:
  + 0.5 m2 Tan leather

Size = Large:
  + 1x Wide strap
```

### Layer 2b: Option Quantity Modifiers

Modifiers that adjust Layer 1 quantities when a specific option is selected.

```
Size = Large:
  - Leather: multiply by 1.4 (40% more material)
  - Thread: multiply by 1.3 (30% more thread)
```

Modifier types:
- `multiply` — multiply existing quantity
- `add` — add to existing quantity
- `set` — replace existing quantity

Modifiers are applied in attribute position order (deterministic).

### Layer 3: Variant Overrides (Rare)

Direct overrides for specific variant combinations.

```
Brown/Large:
  - REPLACE brass buckle with antique brass buckle
  - ADD 1x special finish coating
```

Override types:
- `replace` — swap one material for another
- `add` — add a new material
- `remove` — remove a material
- `set_quantity` — override the resolved quantity

### BOM Resolution Algorithm

For a given variant (e.g., Black/Large):

```
1. Start with product BOM entries (Layer 1)
   → {brass_buckle: 1, thread: 3m, magnetic_clasp: 1}

2a. Add option materials (Layer 2a)
   → + {black_leather: 0.5m2, black_dye: 1, wide_strap: 1}

2b. Apply option modifiers (Layer 2b, in attribute position order)
   → thread: 3m * 1.3 = 3.9m (Size=Large modifier)
   → (if leather were in Layer 1, it would be multiplied by 1.4)

3. Apply variant overrides (Layer 3)
   → (none for Black/Large)

Final BOM:
   brass_buckle: 1, thread: 3.9m, magnetic_clasp: 1,
   black_leather: 0.5m2, black_dye: 1, wide_strap: 1
```

---

## Producibility

**Producibility** answers: "How many units of this variant can I produce with
current raw material stock?"

```
For each material in the resolved BOM:
  possible_units = floor(material.stock_quantity / required_quantity)

Producibility = min(possible_units) across all materials
```

**Example:**
```
Black/Large BOM:
  brass_buckle: 1 needed, 50 in stock → 50 possible
  thread: 3.9m needed, 100m in stock → 25 possible
  magnetic_clasp: 1 needed, 30 in stock → 30 possible
  black_leather: 0.5m2 needed, 10m2 in stock → 20 possible
  black_dye: 1 needed, 15 in stock → 15 possible
  wide_strap: 1 needed, 8 in stock → 8 possible

Producibility = 8 (limited by wide_strap)
```

The admin shows producibility per variant with the limiting material highlighted.

---

## Stock Management

### Raw Materials

Raw materials track:
- Current stock quantity (with decimal precision for continuous materials)
- Unit of measure (piece, meter, square_meter, kilogram, liter, etc.)
- Cost per unit
- Low stock threshold (triggers alerts)
- Supplier information

### Stock Movements

Every change to raw material or variant stock is recorded:

| Movement Type        | Description                        |
|---------------------|------------------------------------|
| `purchase`          | Stock received from supplier       |
| `sale`              | Stock deducted from customer order |
| `adjustment`        | Manual correction                  |
| `production_consume`| Materials used in production batch  |
| `production_output` | Finished goods from production     |
| `return`            | Customer return                    |
| `damage`            | Stock lost to damage               |

Movements record quantity before/after, unit cost, reference (order/batch ID),
and the admin user who made the change.

---

## Admin Workflow

### Creating a Configurable Product

1. **Create product** with basic info, price, weight
2. **Add attributes** (e.g., Color with Black/Tan/Brown, Size with Standard/Large)
3. **Generate variants** — system creates all combinations
4. **Review variants** — adjust prices, weights, stock, deactivate unwanted combos
5. **Set up BOM** — add product-level materials, option materials, modifiers
6. **Verify producibility** — check which variants can be produced

### Adding a New Option

When you add a new option (e.g., Color "Navy"):

1. Add the option to the Color attribute
2. Regenerate variants — new combinations are added
3. Existing variants (Black/*, Tan/*, Brown/*) are preserved
4. Set up BOM entries for the new option (Navy leather, etc.)
5. Set stock for new variants
