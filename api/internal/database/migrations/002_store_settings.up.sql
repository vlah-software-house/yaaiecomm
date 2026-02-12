-- 002_store_settings.up.sql
-- Single-row store settings table with VAT configuration

CREATE TABLE store_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_name TEXT NOT NULL DEFAULT 'My Store',
    store_email TEXT,
    store_phone TEXT,
    store_address JSONB,
    default_currency TEXT NOT NULL DEFAULT 'EUR',

    -- VAT configuration
    vat_enabled BOOLEAN NOT NULL DEFAULT false,
    vat_number TEXT,                                               -- store's own VAT number, e.g., "ES12345678A"
    vat_country_code TEXT REFERENCES eu_countries(country_code),   -- store's country of registration
    vat_prices_include_vat BOOLEAN NOT NULL DEFAULT true,          -- whether entered prices include VAT
    vat_default_category TEXT NOT NULL DEFAULT 'standard',         -- default VAT category for new products
    vat_b2b_reverse_charge_enabled BOOLEAN NOT NULL DEFAULT true,  -- enable B2B reverse charge for valid EU VAT numbers

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index on the FK column
CREATE INDEX idx_store_settings_vat_country_code ON store_settings(vat_country_code);

-- Insert one default row
INSERT INTO store_settings (store_name) VALUES ('My Store');
