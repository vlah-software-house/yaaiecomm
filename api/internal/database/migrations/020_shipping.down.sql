-- 020_shipping.down.sql

-- Remove the FK constraint from store_shipping_countries before dropping shipping_zones
ALTER TABLE store_shipping_countries
    DROP CONSTRAINT IF EXISTS fk_store_shipping_countries_zone;

DROP TABLE IF EXISTS shipping_zones;
DROP TABLE IF EXISTS shipping_config;
