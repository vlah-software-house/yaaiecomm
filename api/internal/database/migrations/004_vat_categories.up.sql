-- 004_vat_categories.up.sql
-- VAT categories that products can be assigned to

CREATE TABLE vat_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,                -- internal name: "standard", "reduced", etc.
    display_name TEXT NOT NULL,               -- human-readable: "Standard Rate", "Reduced Rate"
    description TEXT,
    maps_to_rate_type TEXT NOT NULL,           -- which vat_rates.rate_type this category maps to
    is_default BOOLEAN NOT NULL DEFAULT false,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for ordering and default lookup
CREATE INDEX idx_vat_categories_position ON vat_categories(position);
CREATE INDEX idx_vat_categories_default ON vat_categories(is_default) WHERE is_default = true;

-- Insert the standard set of VAT categories
INSERT INTO vat_categories (name, display_name, description, maps_to_rate_type, is_default, position) VALUES
    ('standard',      'Standard Rate',       'Default rate for most goods and services',                             'standard',      true,  1),
    ('reduced',       'Reduced Rate',        'First reduced rate for qualifying goods (food, books, etc.)',          'reduced',       false, 2),
    ('reduced_alt',   'Reduced Rate (Alt)',   'Second reduced rate available in some EU countries',                  'reduced_alt',   false, 3),
    ('super_reduced', 'Super Reduced Rate',  'Rate below 5%, only available in grandfathered EU countries',         'super_reduced', false, 4),
    ('parking',       'Parking Rate',        'Transitional rate (12%+), available in limited EU countries',         'parking',       false, 5),
    ('zero',          'Zero Rate',           'Zero-rated goods with right of input VAT deduction',                  'zero',          false, 6),
    ('exempt',        'Exempt',              'VAT-exempt goods and services, no input VAT deduction',               'exempt',        false, 7);
