-- ForgeCommerce Seed Data
-- Idempotent: uses INSERT ... ON CONFLICT DO NOTHING throughout.
-- Run with: psql $DATABASE_URL -f scripts/seed.sql

BEGIN;

-- =============================================================================
-- 1. EU Countries (all 27 member states)
-- =============================================================================

INSERT INTO eu_countries (country_code, name, local_vat_name, local_vat_abbreviation, is_eu_member, currency)
VALUES
  ('AT', 'Austria',        'Umsatzsteuer',                         'USt.',   true, 'EUR'),
  ('BE', 'Belgium',        'Taxe sur la valeur ajoutée',           'TVA',    true, 'EUR'),
  ('BG', 'Bulgaria',       'Данък върху добавената стойност',      'ДДС',    true, 'BGN'),
  ('HR', 'Croatia',        'Porez na dodanu vrijednost',           'PDV',    true, 'EUR'),
  ('CY', 'Cyprus',         'Φόρος Προστιθέμενης Αξίας',           'ΦΠΑ',    true, 'EUR'),
  ('CZ', 'Czech Republic', 'Daň z přidané hodnoty',               'DPH',    true, 'CZK'),
  ('DK', 'Denmark',        'Merværdiafgift',                       'moms',   true, 'DKK'),
  ('EE', 'Estonia',        'Käibemaks',                            'km',     true, 'EUR'),
  ('FI', 'Finland',        'Arvonlisävero',                        'ALV',    true, 'EUR'),
  ('FR', 'France',         'Taxe sur la valeur ajoutée',           'TVA',    true, 'EUR'),
  ('DE', 'Germany',        'Mehrwertsteuer',                       'MwSt.',  true, 'EUR'),
  ('GR', 'Greece',         'Φόρος Προστιθέμενης Αξίας',           'ΦΠΑ',    true, 'EUR'),
  ('HU', 'Hungary',        'Általános forgalmi adó',               'ÁFA',    true, 'HUF'),
  ('IE', 'Ireland',        'Value-Added Tax',                      'VAT',    true, 'EUR'),
  ('IT', 'Italy',          'Imposta sul valore aggiunto',          'IVA',    true, 'EUR'),
  ('LV', 'Latvia',         'Pievienotās vērtības nodoklis',       'PVN',    true, 'EUR'),
  ('LT', 'Lithuania',      'Pridėtinės vertės mokestis',          'PVM',    true, 'EUR'),
  ('LU', 'Luxembourg',     'Taxe sur la valeur ajoutée',           'TVA',    true, 'EUR'),
  ('MT', 'Malta',          'Taxxa tal-Valur Miżjud',              'TVA',    true, 'EUR'),
  ('NL', 'Netherlands',    'Belasting over de toegevoegde waarde', 'BTW',    true, 'EUR'),
  ('PL', 'Poland',         'Podatek od towarów i usług',          'PTU',    true, 'PLN'),
  ('PT', 'Portugal',       'Imposto sobre o Valor Acrescentado',  'IVA',    true, 'EUR'),
  ('RO', 'Romania',        'Taxa pe valoarea adăugată',            'TVA',    true, 'RON'),
  ('SK', 'Slovakia',       'Daň z pridanej hodnoty',              'DPH',    true, 'EUR'),
  ('SI', 'Slovenia',       'Davek na dodano vrednost',             'DDV',    true, 'EUR'),
  ('ES', 'Spain',          'Impuesto sobre el Valor Añadido',     'IVA',    true, 'EUR'),
  ('SE', 'Sweden',         'Mervärdesskatt',                       'moms',   true, 'SEK')
ON CONFLICT (country_code) DO NOTHING;


-- =============================================================================
-- 2. VAT Categories
-- =============================================================================

INSERT INTO vat_categories (id, name, display_name, description, maps_to_rate_type, is_default, position)
VALUES
  ('a0000000-0000-0000-0000-000000000001', 'standard',      'Standard Rate',      'Default rate for most goods and services',                   'standard',      true,  1),
  ('a0000000-0000-0000-0000-000000000002', 'reduced',       'Reduced Rate',       'First reduced rate for qualifying goods (food, books, etc.)', 'reduced',       false, 2),
  ('a0000000-0000-0000-0000-000000000003', 'reduced_alt',   'Reduced Rate (Alt)', 'Second reduced rate, available in some countries',            'reduced_alt',   false, 3),
  ('a0000000-0000-0000-0000-000000000004', 'super_reduced', 'Super Reduced Rate', 'Below 5%, only available in grandfathered countries',         'super_reduced', false, 4),
  ('a0000000-0000-0000-0000-000000000005', 'parking',       'Parking Rate',       'Transitional rate (12%+), limited countries',                 'parking',       false, 5),
  ('a0000000-0000-0000-0000-000000000006', 'zero',          'Zero Rate',          '0% rate with right of input VAT deduction',                  'zero',          false, 6),
  ('a0000000-0000-0000-0000-000000000007', 'exempt',        'Exempt',             'No VAT charged, no input VAT deduction',                     'exempt',        false, 7)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 3. Store Settings
-- =============================================================================

INSERT INTO store_settings (
  id,
  store_name,
  vat_enabled,
  vat_number,
  vat_country_code,
  vat_prices_include_vat,
  vat_default_category,
  vat_b2b_reverse_charge_enabled
)
VALUES (
  'b0000000-0000-0000-0000-000000000001',
  'ForgeCommerce Demo Store',
  true,
  'ESB12345678',
  'ES',
  true,
  'standard',
  true
)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 4. VAT Rates (sample rates for major countries)
-- =============================================================================

-- Germany (DE): Standard 19%, Reduced 7%
INSERT INTO vat_rates (id, country_code, rate_type, rate, description, valid_from, valid_to, source, synced_at)
VALUES
  ('c0000000-0000-0000-0000-000000000001', 'DE', 'standard', 19.00, 'Standard rate',       '2007-01-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000002', 'DE', 'reduced',   7.00, 'Reduced rate',        '2007-01-01', NULL, 'seed', NOW())
ON CONFLICT (id) DO NOTHING;

-- France (FR): Standard 20%, Reduced 10%, Reduced Alt 5.5%, Super Reduced 2.1%
INSERT INTO vat_rates (id, country_code, rate_type, rate, description, valid_from, valid_to, source, synced_at)
VALUES
  ('c0000000-0000-0000-0000-000000000003', 'FR', 'standard',       20.00, 'Standard rate',       '2014-01-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000004', 'FR', 'reduced',        10.00, 'Reduced rate',        '2014-01-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000005', 'FR', 'reduced_alt',     5.50, 'Second reduced rate', '2014-01-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000006', 'FR', 'super_reduced',   2.10, 'Super reduced rate',  '1992-01-01', NULL, 'seed', NOW())
ON CONFLICT (id) DO NOTHING;

-- Spain (ES): Standard 21%, Reduced 10%, Super Reduced 4%
INSERT INTO vat_rates (id, country_code, rate_type, rate, description, valid_from, valid_to, source, synced_at)
VALUES
  ('c0000000-0000-0000-0000-000000000007', 'ES', 'standard',      21.00, 'Standard rate',      '2012-09-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000008', 'ES', 'reduced',       10.00, 'Reduced rate',       '2012-09-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000009', 'ES', 'super_reduced',  4.00, 'Super reduced rate', '1995-01-01', NULL, 'seed', NOW())
ON CONFLICT (id) DO NOTHING;

-- Italy (IT): Standard 22%, Reduced 10%, Super Reduced 4%
INSERT INTO vat_rates (id, country_code, rate_type, rate, description, valid_from, valid_to, source, synced_at)
VALUES
  ('c0000000-0000-0000-0000-000000000010', 'IT', 'standard',      22.00, 'Standard rate',      '2013-10-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000011', 'IT', 'reduced',       10.00, 'Reduced rate',       '1995-01-01', NULL, 'seed', NOW()),
  ('c0000000-0000-0000-0000-000000000012', 'IT', 'super_reduced',  4.00, 'Super reduced rate', '1995-01-01', NULL, 'seed', NOW())
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 5. Store Shipping Countries (enable ES, DE, FR, IT, PT, NL, BE)
-- =============================================================================

INSERT INTO store_shipping_countries (country_code, is_enabled, position)
VALUES
  ('ES', true,  1),
  ('DE', true,  2),
  ('FR', true,  3),
  ('IT', true,  4),
  ('PT', true,  5),
  ('NL', true,  6),
  ('BE', true,  7)
ON CONFLICT (country_code) DO NOTHING;


-- =============================================================================
-- 6. Admin User
--    Email: admin@forgecommerce.local
--    Password: admin123 (bcrypt cost 12)
-- =============================================================================

-- Pre-computed bcrypt hash for 'admin123' at cost 12:
-- $2a$12$MJIaL5VIKmVDGD4.qiC3OumyGXw4ESB4oV8dK8NmdkbvfbuEBYcwm
INSERT INTO admin_users (id, email, name, password_hash, role, is_active, totp_verified, force_2fa_setup, created_at, updated_at)
VALUES (
  'd0000000-0000-0000-0000-000000000001',
  'admin@forgecommerce.local',
  'Admin',
  '$2a$12$MJIaL5VIKmVDGD4.qiC3OumyGXw4ESB4oV8dK8NmdkbvfbuEBYcwm',
  'super_admin',
  true,
  false,
  false,
  NOW(),
  NOW()
)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 7. Categories
-- =============================================================================

INSERT INTO categories (id, name, slug, description, parent_id, position, is_active, created_at, updated_at)
VALUES
  ('e0000000-0000-0000-0000-000000000001', 'Bags',        'bags',        'Handcrafted leather and canvas bags',  NULL, 1, true, NOW(), NOW()),
  ('e0000000-0000-0000-0000-000000000002', 'Accessories', 'accessories', 'Belts, wallets, and small goods',      NULL, 2, true, NOW(), NOW()),
  ('e0000000-0000-0000-0000-000000000003', 'Materials',   'materials',   'Raw materials and craft supplies',     NULL, 3, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 8. Sample Products
-- =============================================================================

-- Product 1: Leather Messenger Bag (with variants)
INSERT INTO products (
  id, name, slug, description, short_description, status,
  sku_prefix, base_price, compare_at_price, vat_category_id,
  base_weight_grams, has_variants, created_at, updated_at
)
VALUES (
  'f0000000-0000-0000-0000-000000000001',
  'Leather Messenger Bag',
  'leather-messenger-bag',
  'Premium full-grain leather messenger bag, handcrafted in Spain. Features brass hardware, adjustable shoulder strap, and magnetic closure. Perfect for daily carry or business use.',
  'Handcrafted full-grain leather messenger bag with brass hardware.',
  'active',
  'LMB',
  89.00,
  109.00,
  'a0000000-0000-0000-0000-000000000001', -- standard VAT
  850,
  true,
  NOW(),
  NOW()
)
ON CONFLICT (id) DO NOTHING;

-- Product 1: Attributes
INSERT INTO product_attributes (id, product_id, name, display_name, attribute_type, position, affects_pricing, affects_shipping)
VALUES
  ('f1000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'color', 'Color', 'color_swatch', 1, false, false),
  ('f1000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000001', 'size',  'Size',  'button_group', 2, true,  true)
ON CONFLICT (id) DO NOTHING;

-- Product 1: Attribute Options
INSERT INTO product_attribute_options (id, attribute_id, value, display_value, color_hex, price_modifier, weight_modifier_grams, position, is_active)
VALUES
  -- Colors
  ('f2000000-0000-0000-0000-000000000001', 'f1000000-0000-0000-0000-000000000001', 'black', 'Black', '#1a1a1a', 0,     0,   1, true),
  ('f2000000-0000-0000-0000-000000000002', 'f1000000-0000-0000-0000-000000000001', 'tan',   'Tan',   '#d2a679', 0,     0,   2, true),
  ('f2000000-0000-0000-0000-000000000003', 'f1000000-0000-0000-0000-000000000001', 'brown', 'Brown', '#6b4226', 0,     0,   3, true),
  -- Sizes
  ('f2000000-0000-0000-0000-000000000004', 'f1000000-0000-0000-0000-000000000002', 'standard', 'Standard', NULL, 0,     0,   1, true),
  ('f2000000-0000-0000-0000-000000000005', 'f1000000-0000-0000-0000-000000000002', 'large',    'Large',    NULL, 15.00, 200, 2, true)
ON CONFLICT (id) DO NOTHING;

-- Product 1: Variants (6 = 3 colors x 2 sizes)
INSERT INTO product_variants (id, product_id, sku, price, weight_grams, stock_quantity, low_stock_threshold, is_active, position)
VALUES
  ('f3000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'LMB-BLK-STD', NULL, NULL, 25, 5, true, 1),
  ('f3000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000001', 'LMB-BLK-LRG', NULL, NULL, 18, 5, true, 2),
  ('f3000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000001', 'LMB-TAN-STD', NULL, NULL, 30, 5, true, 3),
  ('f3000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000001', 'LMB-TAN-LRG', NULL, NULL, 12, 5, true, 4),
  ('f3000000-0000-0000-0000-000000000005', 'f0000000-0000-0000-0000-000000000001', 'LMB-BRN-STD', NULL, NULL, 20, 5, true, 5),
  ('f3000000-0000-0000-0000-000000000006', 'f0000000-0000-0000-0000-000000000001', 'LMB-BRN-LRG', NULL, NULL,  8, 5, true, 6)
ON CONFLICT (id) DO NOTHING;

-- Product 1: Variant-Option junctions
INSERT INTO product_variant_options (variant_id, attribute_id, option_id)
VALUES
  -- Black/Standard
  ('f3000000-0000-0000-0000-000000000001', 'f1000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001'),
  ('f3000000-0000-0000-0000-000000000001', 'f1000000-0000-0000-0000-000000000002', 'f2000000-0000-0000-0000-000000000004'),
  -- Black/Large
  ('f3000000-0000-0000-0000-000000000002', 'f1000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001'),
  ('f3000000-0000-0000-0000-000000000002', 'f1000000-0000-0000-0000-000000000002', 'f2000000-0000-0000-0000-000000000005'),
  -- Tan/Standard
  ('f3000000-0000-0000-0000-000000000003', 'f1000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000002'),
  ('f3000000-0000-0000-0000-000000000003', 'f1000000-0000-0000-0000-000000000002', 'f2000000-0000-0000-0000-000000000004'),
  -- Tan/Large
  ('f3000000-0000-0000-0000-000000000004', 'f1000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000002'),
  ('f3000000-0000-0000-0000-000000000004', 'f1000000-0000-0000-0000-000000000002', 'f2000000-0000-0000-0000-000000000005'),
  -- Brown/Standard
  ('f3000000-0000-0000-0000-000000000005', 'f1000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000003'),
  ('f3000000-0000-0000-0000-000000000005', 'f1000000-0000-0000-0000-000000000002', 'f2000000-0000-0000-0000-000000000004'),
  -- Brown/Large
  ('f3000000-0000-0000-0000-000000000006', 'f1000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000003'),
  ('f3000000-0000-0000-0000-000000000006', 'f1000000-0000-0000-0000-000000000002', 'f2000000-0000-0000-0000-000000000005')
ON CONFLICT DO NOTHING;

-- Product 1: Category assignment
INSERT INTO product_categories (product_id, category_id, position)
VALUES ('f0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000001', 1)
ON CONFLICT DO NOTHING;


-- Product 2: Waxed Canvas Tote (simple product, single variant)
INSERT INTO products (
  id, name, slug, description, short_description, status,
  sku_prefix, base_price, compare_at_price, vat_category_id,
  base_weight_grams, has_variants, created_at, updated_at
)
VALUES (
  'f0000000-0000-0000-0000-000000000002',
  'Waxed Canvas Tote',
  'waxed-canvas-tote',
  'Durable waxed canvas tote bag with leather handles and reinforced bottom. Water-resistant and built to last. Made from premium European canvas and vegetable-tanned leather.',
  'Water-resistant waxed canvas tote with leather handles.',
  'active',
  'WCT',
  69.00,
  NULL,
  'a0000000-0000-0000-0000-000000000001', -- standard VAT
  520,
  false,
  NOW(),
  NOW()
)
ON CONFLICT (id) DO NOTHING;

-- Product 2: Single default variant (simple product)
INSERT INTO product_variants (id, product_id, sku, price, weight_grams, stock_quantity, low_stock_threshold, is_active, position)
VALUES (
  'f3000000-0000-0000-0000-000000000010',
  'f0000000-0000-0000-0000-000000000002',
  'WCT-DEFAULT',
  NULL,
  NULL,
  45,
  10,
  true,
  1
)
ON CONFLICT (id) DO NOTHING;

-- Product 2: Category assignment
INSERT INTO product_categories (product_id, category_id, position)
VALUES ('f0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000001', 2)
ON CONFLICT DO NOTHING;


COMMIT;
