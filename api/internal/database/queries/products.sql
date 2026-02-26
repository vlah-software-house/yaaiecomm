-- name: GetProduct :one
SELECT * FROM products WHERE id = $1;

-- name: GetProductBySlug :one
SELECT * FROM products WHERE slug = $1;

-- name: ListProducts :many
SELECT * FROM products
WHERE ($1::text = '' OR status = $1::text)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountProducts :one
SELECT COUNT(*) FROM products
WHERE ($1::text = '' OR status = $1::text);

-- name: CreateProduct :one
INSERT INTO products (
  id, name, slug, description, short_description, status, sku_prefix,
  base_price, compare_at_price, vat_category_id,
  base_weight_grams, base_dimensions_mm,
  shipping_extra_fee_per_unit, has_variants,
  seo_title, seo_description, metadata,
  created_at, updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10,
  $11, $12,
  $13, $14,
  $15, $16, $17,
  $18, $18
)
RETURNING *;

-- name: UpdateProduct :one
UPDATE products SET
  name = $2, slug = $3, description = $4, short_description = $5,
  status = $6, sku_prefix = $7,
  base_price = $8, compare_at_price = $9, vat_category_id = $10,
  base_weight_grams = $11, base_dimensions_mm = $12,
  shipping_extra_fee_per_unit = $13, has_variants = $14,
  seo_title = $15, seo_description = $16, metadata = $17,
  updated_at = $18
WHERE id = $1
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products WHERE id = $1;

-- name: ListProductCategories :many
SELECT c.* FROM categories c
JOIN product_categories pc ON pc.category_id = c.id
WHERE pc.product_id = $1
ORDER BY pc.position;

-- name: SetProductCategories :exec
DELETE FROM product_categories WHERE product_id = $1;

-- name: AddProductCategory :exec
INSERT INTO product_categories (product_id, category_id, position)
VALUES ($1, $2, $3)
ON CONFLICT (product_id, category_id) DO UPDATE SET position = EXCLUDED.position;

-- name: SearchProducts :many
SELECT * FROM products
WHERE (name ILIKE '%' || $1 || '%' OR sku_prefix ILIKE '%' || $1 || '%')
AND ($2::text = '' OR status = $2::text)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
