-- name: GetRawMaterial :one
SELECT * FROM raw_materials WHERE id = $1;

-- name: ListRawMaterials :many
SELECT * FROM raw_materials
WHERE ($1::uuid IS NULL OR category_id = $1)
AND ($2::bool IS NULL OR is_active = $2)
ORDER BY name
LIMIT $3 OFFSET $4;

-- name: CountRawMaterials :one
SELECT COUNT(*) FROM raw_materials
WHERE ($1::uuid IS NULL OR category_id = $1)
AND ($2::bool IS NULL OR is_active = $2);

-- name: CreateRawMaterial :one
INSERT INTO raw_materials (
  id, name, sku, description, category_id, unit_of_measure,
  cost_per_unit, stock_quantity, low_stock_threshold,
  supplier_name, supplier_sku, lead_time_days,
  metadata, is_active, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $15)
RETURNING *;

-- name: UpdateRawMaterial :one
UPDATE raw_materials SET
  name = $2, sku = $3, description = $4, category_id = $5,
  unit_of_measure = $6, cost_per_unit = $7, stock_quantity = $8,
  low_stock_threshold = $9, supplier_name = $10, supplier_sku = $11,
  lead_time_days = $12, metadata = $13, is_active = $14, updated_at = $15
WHERE id = $1
RETURNING *;

-- name: DeleteRawMaterial :exec
DELETE FROM raw_materials WHERE id = $1;

-- name: ListRawMaterialCategories :many
SELECT * FROM raw_material_categories ORDER BY position, name;

-- name: CreateRawMaterialCategory :one
INSERT INTO raw_material_categories (id, name, slug, parent_id, position, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListLowStockRawMaterials :many
SELECT * FROM raw_materials
WHERE stock_quantity <= low_stock_threshold AND is_active = true
ORDER BY stock_quantity ASC
LIMIT $1;

-- name: SearchRawMaterials :many
SELECT * FROM raw_materials
WHERE (name ILIKE '%' || $1 || '%' OR sku ILIKE '%' || $1 || '%')
ORDER BY name
LIMIT $2 OFFSET $3;
