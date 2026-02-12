-- seed.sql — Sample data for ForgeCommerce development
-- Run after migrations: docker compose exec -T postgres psql -U forge -d forgecommerce -f /dev/stdin < scripts/seed.sql

-- ============================================================
-- VAT Rates (current EU rates as of 2026, source: seed)
-- ============================================================

INSERT INTO vat_rates (country_code, rate_type, rate, description, valid_from, source) VALUES
-- Austria
('AT', 'standard',      20.00, 'Standard rate',       '2016-01-01', 'seed'),
('AT', 'reduced',       10.00, 'Reduced rate',        '2016-01-01', 'seed'),
('AT', 'reduced_alt',   13.00, 'Second reduced rate', '2016-01-01', 'seed'),
('AT', 'parking',       12.00, 'Parking rate',        '2016-01-01', 'seed'),
-- Belgium
('BE', 'standard',      21.00, 'Standard rate',       '1996-01-01', 'seed'),
('BE', 'reduced',       12.00, 'Reduced rate',        '1996-01-01', 'seed'),
('BE', 'reduced_alt',    6.00, 'Second reduced rate', '1996-01-01', 'seed'),
('BE', 'parking',       12.00, 'Parking rate',        '1996-01-01', 'seed'),
-- Bulgaria
('BG', 'standard',      20.00, 'Standard rate',       '2007-01-01', 'seed'),
('BG', 'reduced',        9.00, 'Reduced rate',        '2007-01-01', 'seed'),
-- Croatia
('HR', 'standard',      25.00, 'Standard rate',       '2013-07-01', 'seed'),
('HR', 'reduced',       13.00, 'Reduced rate',        '2013-07-01', 'seed'),
('HR', 'reduced_alt',    5.00, 'Second reduced rate', '2013-07-01', 'seed'),
-- Cyprus
('CY', 'standard',      19.00, 'Standard rate',       '2014-01-13', 'seed'),
('CY', 'reduced',        9.00, 'Reduced rate',        '2014-01-13', 'seed'),
('CY', 'reduced_alt',    5.00, 'Second reduced rate', '2014-01-13', 'seed'),
-- Czech Republic
('CZ', 'standard',      21.00, 'Standard rate',       '2015-01-01', 'seed'),
('CZ', 'reduced',       12.00, 'Reduced rate',        '2015-01-01', 'seed'),
('CZ', 'reduced_alt',   10.00, 'Second reduced rate', '2015-01-01', 'seed'),
-- Denmark
('DK', 'standard',      25.00, 'Standard rate',       '1992-01-01', 'seed'),
-- Estonia
('EE', 'standard',      22.00, 'Standard rate',       '2024-01-01', 'seed'),
('EE', 'reduced',        9.00, 'Reduced rate',        '2024-01-01', 'seed'),
-- Finland
('FI', 'standard',      25.50, 'Standard rate',       '2024-09-01', 'seed'),
('FI', 'reduced',       14.00, 'Reduced rate',        '2024-09-01', 'seed'),
('FI', 'reduced_alt',   10.00, 'Second reduced rate', '2024-09-01', 'seed'),
-- France
('FR', 'standard',      20.00, 'Standard rate',       '2014-01-01', 'seed'),
('FR', 'reduced',       10.00, 'Reduced rate',        '2014-01-01', 'seed'),
('FR', 'reduced_alt',    5.50, 'Second reduced rate', '2014-01-01', 'seed'),
('FR', 'super_reduced',  2.10, 'Super reduced rate',  '2014-01-01', 'seed'),
-- Germany
('DE', 'standard',      19.00, 'Standard rate',       '2021-01-01', 'seed'),
('DE', 'reduced',        7.00, 'Reduced rate',        '2021-01-01', 'seed'),
-- Greece
('GR', 'standard',      24.00, 'Standard rate',       '2016-06-01', 'seed'),
('GR', 'reduced',       13.00, 'Reduced rate',        '2016-06-01', 'seed'),
('GR', 'reduced_alt',    6.00, 'Second reduced rate', '2016-06-01', 'seed'),
-- Hungary
('HU', 'standard',      27.00, 'Standard rate',       '2012-01-01', 'seed'),
('HU', 'reduced',       18.00, 'Reduced rate',        '2012-01-01', 'seed'),
('HU', 'reduced_alt',    5.00, 'Second reduced rate', '2012-01-01', 'seed'),
-- Ireland
('IE', 'standard',      23.00, 'Standard rate',       '2012-01-01', 'seed'),
('IE', 'reduced',       13.50, 'Reduced rate',        '2012-01-01', 'seed'),
('IE', 'reduced_alt',    9.00, 'Second reduced rate', '2012-01-01', 'seed'),
('IE', 'super_reduced',  4.80, 'Super reduced rate',  '2012-01-01', 'seed'),
('IE', 'parking',       13.50, 'Parking rate',        '2012-01-01', 'seed'),
-- Italy
('IT', 'standard',      22.00, 'Standard rate',       '2013-10-01', 'seed'),
('IT', 'reduced',       10.00, 'Reduced rate',        '2013-10-01', 'seed'),
('IT', 'reduced_alt',    5.00, 'Second reduced rate', '2013-10-01', 'seed'),
('IT', 'super_reduced',  4.00, 'Super reduced rate',  '2013-10-01', 'seed'),
-- Latvia
('LV', 'standard',      21.00, 'Standard rate',       '2012-07-01', 'seed'),
('LV', 'reduced',       12.00, 'Reduced rate',        '2012-07-01', 'seed'),
('LV', 'reduced_alt',    5.00, 'Second reduced rate', '2012-07-01', 'seed'),
-- Lithuania
('LT', 'standard',      21.00, 'Standard rate',       '2009-09-01', 'seed'),
('LT', 'reduced',        9.00, 'Reduced rate',        '2009-09-01', 'seed'),
('LT', 'reduced_alt',    5.00, 'Second reduced rate', '2009-09-01', 'seed'),
-- Luxembourg
('LU', 'standard',      17.00, 'Standard rate',       '2024-01-01', 'seed'),
('LU', 'reduced',        8.00, 'Reduced rate',        '2024-01-01', 'seed'),
('LU', 'reduced_alt',    3.00, 'Second reduced rate', '2024-01-01', 'seed'),
('LU', 'super_reduced',  3.00, 'Super reduced rate',  '2024-01-01', 'seed'),
('LU', 'parking',       12.00, 'Parking rate',        '2024-01-01', 'seed'),
-- Malta
('MT', 'standard',      18.00, 'Standard rate',       '2004-05-01', 'seed'),
('MT', 'reduced',        7.00, 'Reduced rate',        '2004-05-01', 'seed'),
('MT', 'reduced_alt',    5.00, 'Second reduced rate', '2004-05-01', 'seed'),
-- Netherlands
('NL', 'standard',      21.00, 'Standard rate',       '2019-01-01', 'seed'),
('NL', 'reduced',        9.00, 'Reduced rate',        '2019-01-01', 'seed'),
-- Poland
('PL', 'standard',      23.00, 'Standard rate',       '2011-01-01', 'seed'),
('PL', 'reduced',        8.00, 'Reduced rate',        '2011-01-01', 'seed'),
('PL', 'reduced_alt',    5.00, 'Second reduced rate', '2011-01-01', 'seed'),
-- Portugal
('PT', 'standard',      23.00, 'Standard rate',       '2011-01-01', 'seed'),
('PT', 'reduced',       13.00, 'Reduced rate',        '2011-01-01', 'seed'),
('PT', 'reduced_alt',    6.00, 'Second reduced rate', '2011-01-01', 'seed'),
('PT', 'parking',       13.00, 'Parking rate',        '2011-01-01', 'seed'),
-- Romania
('RO', 'standard',      19.00, 'Standard rate',       '2017-01-01', 'seed'),
('RO', 'reduced',        9.00, 'Reduced rate',        '2017-01-01', 'seed'),
('RO', 'reduced_alt',    5.00, 'Second reduced rate', '2017-01-01', 'seed'),
-- Slovakia
('SK', 'standard',      23.00, 'Standard rate',       '2025-01-01', 'seed'),
('SK', 'reduced',       19.00, 'Reduced rate',        '2025-01-01', 'seed'),
('SK', 'reduced_alt',    5.00, 'Second reduced rate', '2025-01-01', 'seed'),
-- Slovenia
('SI', 'standard',      22.00, 'Standard rate',       '2013-07-01', 'seed'),
('SI', 'reduced',        9.50, 'Reduced rate',        '2013-07-01', 'seed'),
('SI', 'reduced_alt',    5.00, 'Second reduced rate', '2013-07-01', 'seed'),
-- Spain
('ES', 'standard',      21.00, 'Standard rate',       '2012-09-01', 'seed'),
('ES', 'reduced',       10.00, 'Reduced rate',        '2012-09-01', 'seed'),
('ES', 'super_reduced',  4.00, 'Super reduced rate',  '2012-09-01', 'seed'),
-- Sweden
('SE', 'standard',      25.00, 'Standard rate',       '1995-01-01', 'seed'),
('SE', 'reduced',       12.00, 'Reduced rate',        '1995-01-01', 'seed'),
('SE', 'reduced_alt',    6.00, 'Second reduced rate', '1995-01-01', 'seed')
ON CONFLICT (country_code, rate_type, valid_from) DO NOTHING;

-- ============================================================
-- Enable a few selling countries for development
-- ============================================================

UPDATE store_shipping_countries SET is_enabled = true WHERE country_code IN ('ES', 'FR', 'DE', 'IT', 'PT', 'NL', 'BE', 'AT', 'IE');

-- ============================================================
-- Update store settings for development
-- ============================================================

UPDATE store_settings SET
    store_name = 'ForgeCommerce Dev Store',
    store_email = 'store@forgecommerce.local',
    vat_enabled = true,
    vat_country_code = 'ES',
    vat_prices_include_vat = true,
    vat_default_category = 'standard',
    vat_b2b_reverse_charge_enabled = true,
    updated_at = now();
