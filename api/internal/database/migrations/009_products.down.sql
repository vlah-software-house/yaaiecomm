-- 009_products.down.sql

-- Remove the FK constraint added to product_categories before dropping products
ALTER TABLE product_categories
    DROP CONSTRAINT IF EXISTS fk_product_categories_product_id;

DROP TABLE IF EXISTS products;
