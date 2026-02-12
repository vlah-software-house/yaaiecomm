-- name: ListProductAttributes :many
SELECT * FROM product_attributes WHERE product_id = $1 ORDER BY position;

-- name: GetProductAttribute :one
SELECT * FROM product_attributes WHERE id = $1;

-- name: CreateProductAttribute :one
INSERT INTO product_attributes (id, product_id, name, display_name, attribute_type, position, affects_pricing, affects_shipping)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateProductAttribute :one
UPDATE product_attributes SET
  name = $2, display_name = $3, attribute_type = $4,
  position = $5, affects_pricing = $6, affects_shipping = $7
WHERE id = $1
RETURNING *;

-- name: DeleteProductAttribute :exec
DELETE FROM product_attributes WHERE id = $1;

-- name: ListAttributeOptions :many
SELECT * FROM product_attribute_options WHERE attribute_id = $1 ORDER BY position;

-- name: GetAttributeOption :one
SELECT * FROM product_attribute_options WHERE id = $1;

-- name: CreateAttributeOption :one
INSERT INTO product_attribute_options (
  id, attribute_id, value, display_value, color_hex, image_url,
  price_modifier, weight_modifier_grams, position, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateAttributeOption :one
UPDATE product_attribute_options SET
  value = $2, display_value = $3, color_hex = $4, image_url = $5,
  price_modifier = $6, weight_modifier_grams = $7, position = $8, is_active = $9
WHERE id = $1
RETURNING *;

-- name: DeleteAttributeOption :exec
DELETE FROM product_attribute_options WHERE id = $1;
