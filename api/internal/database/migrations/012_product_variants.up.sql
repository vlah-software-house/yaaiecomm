-- 012_product_variants.up.sql
-- Purchasable product variants (Cartesian product of attribute options)
-- Simple products have a single variant with no linked options

CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku TEXT UNIQUE NOT NULL,
    price NUMERIC(12,2),                              -- NULL = calculated from base_price + option modifiers
    compare_at_price NUMERIC(12,2),
    weight_grams INTEGER,                             -- NULL = calculated from base_weight + option modifiers
    dimensions_mm JSONB,
    stock_quantity INTEGER NOT NULL DEFAULT 0,
    low_stock_threshold INTEGER NOT NULL DEFAULT 5,
    barcode TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_product_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_product_variants_sku ON product_variants(sku);
CREATE INDEX idx_product_variants_is_active ON product_variants(is_active);

-- Junction: variant <-> attribute option (which options define this variant)
CREATE TABLE product_variant_options (
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    attribute_id UUID NOT NULL REFERENCES product_attributes(id) ON DELETE CASCADE,
    option_id UUID NOT NULL REFERENCES product_attribute_options(id) ON DELETE CASCADE,
    UNIQUE (variant_id, attribute_id)
);

CREATE INDEX idx_product_variant_options_variant_id ON product_variant_options(variant_id);
CREATE INDEX idx_product_variant_options_option_id ON product_variant_options(option_id);
