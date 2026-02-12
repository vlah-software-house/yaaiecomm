-- 019_discounts.down.sql

-- Remove FK constraints from orders before dropping discount tables
ALTER TABLE orders DROP CONSTRAINT IF EXISTS fk_orders_discount_id;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS fk_orders_coupon_id;

DROP TABLE IF EXISTS coupon_usage;
DROP TABLE IF EXISTS coupons;
DROP TABLE IF EXISTS discounts;
