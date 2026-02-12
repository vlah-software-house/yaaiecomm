-- 003_store_shipping_countries.up.sql
-- Which EU countries the store sells/ships to

CREATE TABLE store_shipping_countries (
    country_code TEXT PRIMARY KEY REFERENCES eu_countries(country_code),
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    shipping_zone_id UUID,                     -- FK added later when shipping_zones table is created
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for looking up enabled countries and ordering
CREATE INDEX idx_store_shipping_countries_enabled ON store_shipping_countries(is_enabled) WHERE is_enabled = true;
CREATE INDEX idx_store_shipping_countries_position ON store_shipping_countries(position);
CREATE INDEX idx_store_shipping_countries_zone ON store_shipping_countries(shipping_zone_id) WHERE shipping_zone_id IS NOT NULL;

-- Insert all 27 EU countries, disabled by default, ordered alphabetically by country_code
INSERT INTO store_shipping_countries (country_code, is_enabled, position) VALUES
    ('AT', false,  1),
    ('BE', false,  2),
    ('BG', false,  3),
    ('CY', false,  4),
    ('CZ', false,  5),
    ('DE', false,  6),
    ('DK', false,  7),
    ('EE', false,  8),
    ('ES', false,  9),
    ('FI', false, 10),
    ('FR', false, 11),
    ('GR', false, 12),
    ('HR', false, 13),
    ('HU', false, 14),
    ('IE', false, 15),
    ('IT', false, 16),
    ('LT', false, 17),
    ('LU', false, 18),
    ('LV', false, 19),
    ('MT', false, 20),
    ('NL', false, 21),
    ('PL', false, 22),
    ('PT', false, 23),
    ('RO', false, 24),
    ('SE', false, 25),
    ('SI', false, 26),
    ('SK', false, 27);
