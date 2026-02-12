-- 020_shipping.up.sql
-- Global shipping configuration, shipping zones, and FK to store_shipping_countries

-- Single-row global shipping configuration
CREATE TABLE shipping_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enabled BOOLEAN NOT NULL DEFAULT true,
    calculation_method TEXT NOT NULL DEFAULT 'fixed' CHECK (calculation_method IN ('fixed', 'weight_based', 'size_based')),
    fixed_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
    weight_rates JSONB NOT NULL DEFAULT '[]',         -- array of { min_weight_g, max_weight_g, fee }
    size_rates JSONB NOT NULL DEFAULT '[]',            -- array of { max_volume_cm3, fee }
    free_shipping_threshold NUMERIC(12,2),
    default_currency TEXT NOT NULL DEFAULT 'EUR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Insert one default row
INSERT INTO shipping_config (enabled, calculation_method, fixed_fee) VALUES (true, 'fixed', 0);

-- Shipping zones for per-zone rate overrides
CREATE TABLE shipping_zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    countries TEXT[] NOT NULL DEFAULT '{}',
    calculation_method TEXT NOT NULL DEFAULT 'fixed' CHECK (calculation_method IN ('fixed', 'weight_based', 'size_based')),
    rates JSONB NOT NULL DEFAULT '{}',
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Now add the FK from store_shipping_countries.shipping_zone_id -> shipping_zones(id)
ALTER TABLE store_shipping_countries
    ADD CONSTRAINT fk_store_shipping_countries_zone
    FOREIGN KEY (shipping_zone_id) REFERENCES shipping_zones(id) ON DELETE SET NULL;
