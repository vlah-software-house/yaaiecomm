-- name: ListProductVariants :many
SELECT * FROM product_variants WHERE product_id = $1 ORDER BY position;

-- name: GetProductVariant :one
SELECT * FROM product_variants WHERE id = $1;

-- name: GetProductVariantBySKU :one
SELECT * FROM product_variants WHERE sku = $1;

-- name: CreateProductVariant :one
INSERT INTO product_variants (
  id, product_id, sku, price, compare_at_price,
  weight_grams, dimensions_mm, stock_quantity, low_stock_threshold,
  barcode, is_active, position
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateProductVariant :one
UPDATE product_variants SET
  sku = $2, price = $3, compare_at_price = $4,
  weight_grams = $5, dimensions_mm = $6, stock_quantity = $7,
  low_stock_threshold = $8, barcode = $9, is_active = $10, position = $11
WHERE id = $1
RETURNING *;

-- name: DeleteProductVariant :exec
DELETE FROM product_variants WHERE id = $1;

-- name: UpdateVariantStock :exec
UPDATE product_variants SET stock_quantity = $2 WHERE id = $1;

-- name: ListVariantOptions :many
SELECT pvo.*, pa.name as attribute_name, pao.value as option_value, pao.display_value as option_display_value
FROM product_variant_options pvo
JOIN product_attributes pa ON pa.id = pvo.attribute_id
JOIN product_attribute_options pao ON pao.id = pvo.option_id
WHERE pvo.variant_id = $1
ORDER BY pa.position;

-- name: SetVariantOption :exec
INSERT INTO product_variant_options (variant_id, attribute_id, option_id)
VALUES ($1, $2, $3)
ON CONFLICT (variant_id, attribute_id) DO UPDATE SET option_id = EXCLUDED.option_id;

-- name: DeleteVariantOptions :exec
DELETE FROM product_variant_options WHERE variant_id = $1;

-- name: ListLowStockVariants :many
SELECT pv.*, p.name as product_name FROM product_variants pv
JOIN products p ON p.id = pv.product_id
WHERE pv.stock_quantity <= pv.low_stock_threshold AND pv.is_active = true
ORDER BY pv.stock_quantity ASC
LIMIT $1;
