-- 009_products.up.sql
-- Core products table with variant support, VAT category, and SEO fields

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    short_description TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    sku_prefix TEXT,
    base_price NUMERIC(12,2) NOT NULL DEFAULT 0,
    compare_at_price NUMERIC(12,2),
    vat_category_id UUID REFERENCES vat_categories(id) ON DELETE SET NULL,
    base_weight_grams INTEGER NOT NULL DEFAULT 0,
    base_dimensions_mm JSONB,
    shipping_extra_fee_per_unit NUMERIC(12,2) NOT NULL DEFAULT 0,
    has_variants BOOLEAN NOT NULL DEFAULT false,
    seo_title TEXT,
    seo_description TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_slug ON products(slug);
CREATE INDEX idx_products_status ON products(status);
CREATE INDEX idx_products_vat_category_id ON products(vat_category_id);
CREATE INDEX idx_products_created_at ON products(created_at);

-- Now that products exists, add the FK constraint on product_categories.product_id
ALTER TABLE product_categories
    ADD CONSTRAINT fk_product_categories_product_id
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;
