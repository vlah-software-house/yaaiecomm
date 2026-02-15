-- ForgeCommerce Extended Seed Data for Admin Documentation Screenshots
-- Depends on: scripts/seed.sql (must be run first)
-- Idempotent: uses INSERT ... ON CONFLICT DO NOTHING throughout.
-- Run with: psql $DATABASE_URL -f scripts/seed-docs.sql

BEGIN;

-- =============================================================================
-- 1. Raw Material Categories
-- =============================================================================

INSERT INTO raw_material_categories (id, name, slug, position, created_at)
VALUES
  ('bb000000-0000-0000-0000-000000000001', 'Leather',            'leather',            1, NOW()),
  ('bb000000-0000-0000-0000-000000000002', 'Hardware',           'hardware',           2, NOW()),
  ('bb000000-0000-0000-0000-000000000003', 'Thread & Supplies',  'thread-and-supplies', 3, NOW())
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 2. Raw Materials
-- =============================================================================

INSERT INTO raw_materials (id, name, sku, description, category_id, unit_of_measure, cost_per_unit, stock_quantity, low_stock_threshold, supplier_name, supplier_sku, lead_time_days, is_active, created_at, updated_at)
VALUES
  (
    'bc000000-0000-0000-0000-000000000001',
    'Full-Grain Leather (Black)',
    'RM-LEATHER-BLK',
    'Premium full-grain cowhide leather, black finish. Sourced from Valencian tanneries.',
    'bb000000-0000-0000-0000-000000000001',
    'm2',
    45.0000,
    120.0000,
    20.0000,
    'Curtidos Levante S.L.',
    'CL-FG-BLK-01',
    14,
    true,
    NOW(),
    NOW()
  ),
  (
    'bc000000-0000-0000-0000-000000000002',
    'Brass Buckle (25mm)',
    'RM-BUCKLE-BRASS-25',
    'Solid brass roller buckle, 25mm width. Polished finish.',
    'bb000000-0000-0000-0000-000000000002',
    'unit',
    3.5000,
    500.0000,
    100.0000,
    'MetalCraft EU',
    'MC-BB-25-POL',
    7,
    true,
    NOW(),
    NOW()
  ),
  (
    'bc000000-0000-0000-0000-000000000003',
    'Waxed Thread (Black)',
    'RM-THREAD-WAX-BLK',
    'Heavy-duty waxed polyester thread, 0.8mm, black. For saddle stitching.',
    'bb000000-0000-0000-0000-000000000003',
    'm',
    0.1500,
    2000.0000,
    500.0000,
    'Thread Masters',
    'TM-WPT-08-BLK',
    5,
    true,
    NOW(),
    NOW()
  ),
  (
    'bc000000-0000-0000-0000-000000000004',
    'Magnetic Clasp',
    'RM-CLASP-MAG-18',
    '18mm magnetic snap clasp, nickel-free brass plated.',
    'bb000000-0000-0000-0000-000000000002',
    'unit',
    2.8000,
    300.0000,
    50.0000,
    'MetalCraft EU',
    'MC-MC-18-BRS',
    7,
    true,
    NOW(),
    NOW()
  )
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 3. BOM Entries for Leather Messenger Bag (product f0000000-...-01)
-- =============================================================================

INSERT INTO product_bom_entries (id, product_id, raw_material_id, quantity, unit_of_measure, is_required, notes, created_at)
VALUES
  ('bd000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001', 'bc000000-0000-0000-0000-000000000001', 0.5000, 'm2',   true, 'Main body panels + flap',   NOW()),
  ('bd000000-0000-0000-0000-000000000002', 'f0000000-0000-0000-0000-000000000001', 'bc000000-0000-0000-0000-000000000002', 1.0000, 'unit', true, 'Strap adjustment buckle',   NOW()),
  ('bd000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000001', 'bc000000-0000-0000-0000-000000000003', 3.0000, 'm',    true, 'Saddle stitching all seams', NOW()),
  ('bd000000-0000-0000-0000-000000000004', 'f0000000-0000-0000-0000-000000000001', 'bc000000-0000-0000-0000-000000000004', 1.0000, 'unit', true, 'Front flap closure',        NOW())
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 4. Shipping Configuration
-- =============================================================================

-- Update the existing default shipping_config row (inserted by migration 020)
-- or insert with our known ID if the default was removed.
-- Use a DO block to handle the update-or-insert cleanly.
DO $$
BEGIN
  -- Try to update the existing single row
  UPDATE shipping_config
  SET enabled = true,
      calculation_method = 'fixed',
      fixed_fee = 8.50,
      free_shipping_threshold = 150.00,
      default_currency = 'EUR',
      updated_at = NOW()
  WHERE id = (SELECT id FROM shipping_config LIMIT 1);

  -- If no rows were updated (table is empty), insert our known row
  IF NOT FOUND THEN
    INSERT INTO shipping_config (id, enabled, calculation_method, fixed_fee, free_shipping_threshold, default_currency, created_at, updated_at)
    VALUES ('be000000-0000-0000-0000-000000000001', true, 'fixed', 8.50, 150.00, 'EUR', NOW(), NOW());
  END IF;
END $$;


-- =============================================================================
-- 5. Shipping Zone
-- =============================================================================

INSERT INTO shipping_zones (id, name, countries, calculation_method, rates, position, created_at, updated_at)
VALUES (
  'be100000-0000-0000-0000-000000000001',
  'Iberian Peninsula',
  ARRAY['ES','PT'],
  'fixed',
  '{"fixed_fee": 6.50}'::jsonb,
  1,
  NOW(),
  NOW()
)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 6. Discount
-- =============================================================================

INSERT INTO discounts (id, name, type, value, scope, minimum_amount, maximum_discount, starts_at, ends_at, is_active, priority, stackable, conditions, created_at, updated_at)
VALUES (
  'bf000000-0000-0000-0000-000000000001',
  'Summer Sale 2026',
  'percentage',
  15.00,
  'subtotal',
  50.00,
  NULL,
  '2026-06-01 00:00:00+00',
  '2026-08-31 23:59:59+00',
  true,
  1,
  false,
  '{}'::jsonb,
  NOW(),
  NOW()
)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 7. Coupon
-- =============================================================================

INSERT INTO coupons (id, code, discount_id, usage_limit, usage_limit_per_customer, usage_count, starts_at, ends_at, is_active, created_at, updated_at)
VALUES (
  'c0000000-0000-0000-0000-000000000010',
  'SUMMER15',
  'bf000000-0000-0000-0000-000000000001',
  100,
  1,
  3,
  '2026-06-01 00:00:00+00',
  '2026-08-31 23:59:59+00',
  true,
  NOW(),
  NOW()
)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 8. Orders
-- =============================================================================

-- Order 1: Domestic ES, delivered
INSERT INTO orders (
  id, order_number, customer_id, status, email,
  billing_address, shipping_address,
  subtotal, shipping_fee, shipping_extra_fees, discount_amount, vat_total, total,
  vat_country_code, vat_number, vat_company_name, vat_reverse_charge,
  payment_status, shipping_method, tracking_number, shipped_at, delivered_at,
  created_at, updated_at
)
VALUES (
  'aa000000-0000-0000-0000-000000000001',
  1001,
  NULL,
  'delivered',
  'maria@example.es',
  '{"first_name":"María","last_name":"García","line1":"Calle Mayor 15","line2":"Piso 3","city":"Madrid","postal_code":"28013","country_code":"ES","phone":"+34612345678"}'::jsonb,
  '{"first_name":"María","last_name":"García","line1":"Calle Mayor 15","line2":"Piso 3","city":"Madrid","postal_code":"28013","country_code":"ES","phone":"+34612345678"}'::jsonb,
  89.00,
  8.50,
  0.00,
  0.00,
  15.46,
  97.50,
  'ES',
  NULL,
  NULL,
  false,
  'paid',
  'standard',
  'ES2026021012345',
  NOW() - INTERVAL '3 days',
  NOW() - INTERVAL '2 days',
  NOW() - INTERVAL '5 days',
  NOW() - INTERVAL '2 days'
)
ON CONFLICT (id) DO NOTHING;

-- Order 2: B2B reverse charge, DE, confirmed
INSERT INTO orders (
  id, order_number, customer_id, status, email,
  billing_address, shipping_address,
  subtotal, shipping_fee, shipping_extra_fees, discount_amount, vat_total, total,
  vat_country_code, vat_number, vat_company_name, vat_reverse_charge,
  payment_status,
  created_at, updated_at
)
VALUES (
  'aa000000-0000-0000-0000-000000000002',
  1002,
  NULL,
  'confirmed',
  'hans@example.de',
  '{"first_name":"Hans","last_name":"Müller","line1":"Friedrichstraße 43","line2":"2. OG","city":"Berlin","postal_code":"10117","country_code":"DE","phone":"+4930123456"}'::jsonb,
  '{"first_name":"Hans","last_name":"Müller","line1":"Friedrichstraße 43","line2":"2. OG","city":"Berlin","postal_code":"10117","country_code":"DE","phone":"+4930123456"}'::jsonb,
  104.00,
  12.00,
  0.00,
  0.00,
  0.00,
  116.00,
  'DE',
  'DE123456789',
  'Beispiel GmbH',
  true,
  'paid',
  NOW() - INTERVAL '1 day',
  NOW() - INTERVAL '1 day'
)
ON CONFLICT (id) DO NOTHING;

-- Order 3: France, processing
INSERT INTO orders (
  id, order_number, customer_id, status, email,
  billing_address, shipping_address,
  subtotal, shipping_fee, shipping_extra_fees, discount_amount, vat_total, total,
  vat_country_code, vat_number, vat_company_name, vat_reverse_charge,
  payment_status,
  created_at, updated_at
)
VALUES (
  'aa000000-0000-0000-0000-000000000003',
  1003,
  NULL,
  'processing',
  'jean@example.fr',
  '{"first_name":"Jean","last_name":"Dupont","line1":"12 Rue de Rivoli","line2":"Apt 4B","city":"Paris","postal_code":"75001","country_code":"FR","phone":"+33612345678"}'::jsonb,
  '{"first_name":"Jean","last_name":"Dupont","line1":"12 Rue de Rivoli","line2":"Apt 4B","city":"Paris","postal_code":"75001","country_code":"FR","phone":"+33612345678"}'::jsonb,
  69.00,
  10.00,
  0.00,
  0.00,
  11.50,
  90.50,
  'FR',
  NULL,
  NULL,
  false,
  'paid',
  NOW() - INTERVAL '12 hours',
  NOW() - INTERVAL '12 hours'
)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 9. Order Items
-- =============================================================================

-- Order 1 item: Leather Messenger Bag, Black/Standard
INSERT INTO order_items (
  id, order_id, product_id, variant_id,
  product_name, variant_name, variant_options, sku,
  quantity, unit_price, total_price,
  vat_rate, vat_rate_type, vat_amount, price_includes_vat, net_unit_price, gross_unit_price,
  weight_grams
)
VALUES (
  'ab000000-0000-0000-0000-000000000001',
  'aa000000-0000-0000-0000-000000000001',
  'f0000000-0000-0000-0000-000000000001',
  'f3000000-0000-0000-0000-000000000001',
  'Leather Messenger Bag',
  'Black / Standard',
  '[{"attribute":"Color","value":"Black"},{"attribute":"Size","value":"Standard"}]'::jsonb,
  'LMB-BLK-STD',
  1,
  89.00,
  89.00,
  21.00,
  'standard',
  15.46,
  true,
  73.55,
  89.00,
  850
)
ON CONFLICT (id) DO NOTHING;

-- Order 2 item: Leather Messenger Bag, Tan/Large (B2B reverse charge)
INSERT INTO order_items (
  id, order_id, product_id, variant_id,
  product_name, variant_name, variant_options, sku,
  quantity, unit_price, total_price,
  vat_rate, vat_rate_type, vat_amount, price_includes_vat, net_unit_price, gross_unit_price,
  weight_grams
)
VALUES (
  'ab000000-0000-0000-0000-000000000002',
  'aa000000-0000-0000-0000-000000000002',
  'f0000000-0000-0000-0000-000000000001',
  'f3000000-0000-0000-0000-000000000004',
  'Leather Messenger Bag',
  'Tan / Large',
  '[{"attribute":"Color","value":"Tan"},{"attribute":"Size","value":"Large"}]'::jsonb,
  'LMB-TAN-LRG',
  1,
  104.00,
  104.00,
  0.00,
  'standard',
  0.00,
  true,
  104.00,
  104.00,
  1050
)
ON CONFLICT (id) DO NOTHING;

-- Order 3 item: Waxed Canvas Tote, Default
INSERT INTO order_items (
  id, order_id, product_id, variant_id,
  product_name, variant_name, variant_options, sku,
  quantity, unit_price, total_price,
  vat_rate, vat_rate_type, vat_amount, price_includes_vat, net_unit_price, gross_unit_price,
  weight_grams
)
VALUES (
  'ab000000-0000-0000-0000-000000000003',
  'aa000000-0000-0000-0000-000000000003',
  'f0000000-0000-0000-0000-000000000002',
  'f3000000-0000-0000-0000-000000000010',
  'Waxed Canvas Tote',
  'Default',
  NULL,
  'WCT-DEFAULT',
  1,
  69.00,
  69.00,
  20.00,
  'standard',
  11.50,
  true,
  57.50,
  69.00,
  520
)
ON CONFLICT (id) DO NOTHING;


-- =============================================================================
-- 10. Order Events (status trail for Order #1001)
-- =============================================================================

INSERT INTO order_events (id, order_id, event_type, from_status, to_status, data, created_by, created_at)
VALUES
  (
    'ae000000-0000-0000-0000-000000000001',
    'aa000000-0000-0000-0000-000000000001',
    'status_changed',
    'pending',
    'confirmed',
    '{"note":"Payment confirmed via Stripe"}'::jsonb,
    'd0000000-0000-0000-0000-000000000001',
    NOW() - INTERVAL '5 days'
  ),
  (
    'ae000000-0000-0000-0000-000000000002',
    'aa000000-0000-0000-0000-000000000001',
    'status_changed',
    'confirmed',
    'processing',
    '{"note":"Preparing for shipment"}'::jsonb,
    'd0000000-0000-0000-0000-000000000001',
    NOW() - INTERVAL '4 days'
  ),
  (
    'ae000000-0000-0000-0000-000000000003',
    'aa000000-0000-0000-0000-000000000001',
    'status_changed',
    'processing',
    'shipped',
    '{"note":"Shipped via Correos Express","tracking_number":"ES2026021012345"}'::jsonb,
    'd0000000-0000-0000-0000-000000000001',
    NOW() - INTERVAL '3 days'
  ),
  (
    'ae000000-0000-0000-0000-000000000004',
    'aa000000-0000-0000-0000-000000000001',
    'status_changed',
    'shipped',
    'delivered',
    '{"note":"Delivered — signed by recipient"}'::jsonb,
    'd0000000-0000-0000-0000-000000000001',
    NOW() - INTERVAL '2 days'
  )
ON CONFLICT (id) DO NOTHING;


COMMIT;
