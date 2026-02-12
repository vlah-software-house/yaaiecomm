-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT * FROM categories WHERE slug = $1;

-- name: ListCategories :many
SELECT * FROM categories WHERE is_active = true ORDER BY position, name;

-- name: ListAllCategories :many
SELECT * FROM categories ORDER BY position, name;

-- name: ListTopCategories :many
SELECT * FROM categories WHERE parent_id IS NULL AND is_active = true ORDER BY position, name;

-- name: ListChildCategories :many
SELECT * FROM categories WHERE parent_id = $1 AND is_active = true ORDER BY position, name;

-- name: CreateCategory :one
INSERT INTO categories (id, name, slug, description, parent_id, position, image_url, seo_title, seo_description, is_active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories SET
  name = $2, slug = $3, description = $4, parent_id = $5,
  position = $6, image_url = $7, seo_title = $8, seo_description = $9,
  is_active = $10, updated_at = $11
WHERE id = $1
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;

-- name: CountProductsInCategory :one
SELECT COUNT(*) FROM product_categories WHERE category_id = $1;
