-- 010_product_vat_overrides.up.sql
-- Per-product per-country VAT category overrides
-- Allows a product to have a different VAT rate type in specific EU countries

CREATE TABLE product_vat_overrides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    country_code TEXT NOT NULL REFERENCES eu_countries(country_code) ON DELETE CASCADE,
    vat_category_id UUID NOT NULL REFERENCES vat_categories(id) ON DELETE CASCADE,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, country_code)
);

CREATE INDEX idx_product_vat_overrides_product_id ON product_vat_overrides(product_id);
CREATE INDEX idx_product_vat_overrides_country_code ON product_vat_overrides(country_code);
CREATE INDEX idx_product_vat_overrides_vat_category_id ON product_vat_overrides(vat_category_id);
