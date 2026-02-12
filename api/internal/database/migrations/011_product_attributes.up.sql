-- 011_product_attributes.up.sql
-- Product attributes (e.g., Color, Size, Material) and their selectable options
-- Used to define variant axes for products with has_variants = true

CREATE TABLE product_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    attribute_type TEXT NOT NULL DEFAULT 'select' CHECK (attribute_type IN ('select', 'color_swatch', 'button_group', 'image_swatch')),
    position INTEGER NOT NULL DEFAULT 0,
    affects_pricing BOOLEAN NOT NULL DEFAULT false,
    affects_shipping BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, name)
);

CREATE INDEX idx_product_attributes_product_id ON product_attributes(product_id);

CREATE TABLE product_attribute_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attribute_id UUID NOT NULL REFERENCES product_attributes(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    display_value TEXT NOT NULL,
    color_hex TEXT,
    image_url TEXT,
    price_modifier NUMERIC(12,2),
    weight_modifier_grams INTEGER,
    position INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (attribute_id, value)
);

CREATE INDEX idx_product_attribute_options_attribute_id ON product_attribute_options(attribute_id);
