-- 024_global_attributes.up.sql
-- Global attribute templates: reusable attribute definitions shared across products
-- with structured metadata schema and option filtering per product

-- Global attribute definitions (store-wide, reusable across products)
CREATE TABLE global_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    attribute_type TEXT NOT NULL DEFAULT 'select'
        CHECK (attribute_type IN ('select', 'color_swatch', 'button_group', 'image_swatch')),
    category TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_global_attributes_category ON global_attributes(category);
CREATE INDEX idx_global_attributes_is_active ON global_attributes(is_active);

-- Metadata field definitions for a global attribute
CREATE TABLE global_attribute_metadata_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    global_attribute_id UUID NOT NULL REFERENCES global_attributes(id) ON DELETE CASCADE,
    field_name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    field_type TEXT NOT NULL DEFAULT 'text'
        CHECK (field_type IN ('text', 'number', 'boolean', 'select', 'url')),
    is_required BOOLEAN NOT NULL DEFAULT false,
    default_value TEXT,
    select_options TEXT[],
    help_text TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (global_attribute_id, field_name)
);

CREATE INDEX idx_global_attribute_metadata_fields_attr ON global_attribute_metadata_fields(global_attribute_id);

-- Options for global attributes
CREATE TABLE global_attribute_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    global_attribute_id UUID NOT NULL REFERENCES global_attributes(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    display_value TEXT NOT NULL,
    color_hex TEXT,
    image_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    position INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (global_attribute_id, value)
);

CREATE INDEX idx_global_attribute_options_attr ON global_attribute_options(global_attribute_id);

-- Links a product to a global attribute with a specific role
CREATE TABLE product_global_attribute_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    global_attribute_id UUID NOT NULL REFERENCES global_attributes(id) ON DELETE CASCADE,
    role_name TEXT NOT NULL,
    role_display_name TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    affects_pricing BOOLEAN NOT NULL DEFAULT false,
    affects_shipping BOOLEAN NOT NULL DEFAULT false,
    price_modifier_field TEXT,
    weight_modifier_field TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, global_attribute_id, role_name)
);

CREATE INDEX idx_product_global_attribute_links_product ON product_global_attribute_links(product_id);
CREATE INDEX idx_product_global_attribute_links_global ON product_global_attribute_links(global_attribute_id);

-- Which options from the global attribute are selected for this product link
CREATE TABLE product_global_option_selections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES product_global_attribute_links(id) ON DELETE CASCADE,
    global_option_id UUID NOT NULL REFERENCES global_attribute_options(id) ON DELETE CASCADE,
    price_modifier NUMERIC(12,2),
    weight_modifier_grams INTEGER,
    position_override INTEGER,
    UNIQUE (link_id, global_option_id)
);

CREATE INDEX idx_product_global_option_selections_link ON product_global_option_selections(link_id);

-- Variant-to-global-option junction
CREATE TABLE product_variant_global_options (
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES product_global_attribute_links(id) ON DELETE CASCADE,
    global_option_id UUID NOT NULL REFERENCES global_attribute_options(id) ON DELETE CASCADE,
    UNIQUE (variant_id, link_id)
);

CREATE INDEX idx_product_variant_global_options_variant ON product_variant_global_options(variant_id);
CREATE INDEX idx_product_variant_global_options_option ON product_variant_global_options(global_option_id);
