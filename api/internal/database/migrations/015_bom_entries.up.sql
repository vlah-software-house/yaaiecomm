-- 015_bom_entries.up.sql
-- Layered Bill of Materials (BOM) system for manufacturer product costing
-- Layer 1: product-level common materials
-- Layer 2a: per-attribute-option additional materials
-- Layer 2b: per-attribute-option modifiers on product BOM entries
-- Layer 3: per-variant overrides

-- Layer 1: Product-level BOM entries (common materials for all variants)
CREATE TABLE product_bom_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    raw_material_id UUID NOT NULL REFERENCES raw_materials(id) ON DELETE RESTRICT,
    quantity NUMERIC(12,4) NOT NULL,
    unit_of_measure TEXT NOT NULL DEFAULT 'unit',
    is_required BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, raw_material_id)
);

CREATE INDEX idx_product_bom_entries_product_id ON product_bom_entries(product_id);
CREATE INDEX idx_product_bom_entries_raw_material_id ON product_bom_entries(raw_material_id);

-- Layer 2a: Per-option additional materials
CREATE TABLE attribute_option_bom_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    option_id UUID NOT NULL REFERENCES product_attribute_options(id) ON DELETE CASCADE,
    raw_material_id UUID NOT NULL REFERENCES raw_materials(id) ON DELETE RESTRICT,
    quantity NUMERIC(12,4) NOT NULL,
    unit_of_measure TEXT NOT NULL DEFAULT 'unit',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (option_id, raw_material_id)
);

CREATE INDEX idx_attribute_option_bom_entries_option_id ON attribute_option_bom_entries(option_id);
CREATE INDEX idx_attribute_option_bom_entries_raw_material_id ON attribute_option_bom_entries(raw_material_id);

-- Layer 2b: Per-option modifiers on product BOM entries
-- Applied in attribute position order for deterministic results
CREATE TABLE attribute_option_bom_modifiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    option_id UUID NOT NULL REFERENCES product_attribute_options(id) ON DELETE CASCADE,
    product_bom_entry_id UUID NOT NULL REFERENCES product_bom_entries(id) ON DELETE CASCADE,
    modifier_type TEXT NOT NULL CHECK (modifier_type IN ('multiply', 'add', 'set')),
    modifier_value NUMERIC(12,4) NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (option_id, product_bom_entry_id)
);

CREATE INDEX idx_attribute_option_bom_modifiers_option_id ON attribute_option_bom_modifiers(option_id);
CREATE INDEX idx_attribute_option_bom_modifiers_product_bom_entry_id ON attribute_option_bom_modifiers(product_bom_entry_id);

-- Layer 3: Per-variant BOM overrides (replace, add, remove, or set quantity)
CREATE TABLE variant_bom_overrides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    raw_material_id UUID NOT NULL REFERENCES raw_materials(id) ON DELETE RESTRICT,
    override_type TEXT NOT NULL CHECK (override_type IN ('replace', 'add', 'remove', 'set_quantity')),
    replaces_material_id UUID REFERENCES raw_materials(id) ON DELETE RESTRICT,
    quantity NUMERIC(12,4),
    unit_of_measure TEXT DEFAULT 'unit',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_variant_bom_overrides_variant_id ON variant_bom_overrides(variant_id);
CREATE INDEX idx_variant_bom_overrides_raw_material_id ON variant_bom_overrides(raw_material_id);
CREATE INDEX idx_variant_bom_overrides_variant_material ON variant_bom_overrides(variant_id, raw_material_id);
CREATE INDEX idx_variant_bom_overrides_replaces_material_id ON variant_bom_overrides(replaces_material_id) WHERE replaces_material_id IS NOT NULL;
