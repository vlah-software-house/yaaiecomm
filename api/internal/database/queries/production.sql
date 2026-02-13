-- name: CreateProductionBatch :one
INSERT INTO production_batches (batch_number, product_id, variant_id, planned_quantity, status, scheduled_date, notes, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetProductionBatch :one
SELECT pb.*,
       p.name as product_name,
       pv.sku as variant_sku
FROM production_batches pb
JOIN products p ON p.id = pb.product_id
LEFT JOIN product_variants pv ON pv.id = pb.variant_id
WHERE pb.id = $1;

-- name: ListProductionBatches :many
SELECT pb.*,
       p.name as product_name,
       pv.sku as variant_sku
FROM production_batches pb
JOIN products p ON p.id = pb.product_id
LEFT JOIN product_variants pv ON pv.id = pb.variant_id
ORDER BY pb.created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListProductionBatchesByStatus :many
SELECT pb.*,
       p.name as product_name,
       pv.sku as variant_sku
FROM production_batches pb
JOIN products p ON p.id = pb.product_id
LEFT JOIN product_variants pv ON pv.id = pb.variant_id
WHERE pb.status = $1
ORDER BY pb.created_at DESC;

-- name: UpdateProductionBatchStatus :one
UPDATE production_batches
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: StartProductionBatch :one
UPDATE production_batches
SET status = 'in_progress', started_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status IN ('draft', 'scheduled')
RETURNING *;

-- name: CompleteProductionBatch :one
UPDATE production_batches
SET status = 'completed', actual_quantity = $2, completed_at = NOW(), cost_total = $3, updated_at = NOW()
WHERE id = $1 AND status = 'in_progress'
RETURNING *;

-- name: CancelProductionBatch :one
UPDATE production_batches
SET status = 'cancelled', updated_at = NOW()
WHERE id = $1 AND status IN ('draft', 'scheduled')
RETURNING *;

-- name: NextBatchNumber :one
SELECT COALESCE(MAX(CAST(SUBSTRING(batch_number FROM 'PB-(\d+)') AS INT)), 0) + 1 AS next_num
FROM production_batches;

-- name: CreateBatchMaterial :one
INSERT INTO production_batch_materials (batch_id, raw_material_id, required_quantity, unit_cost)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListBatchMaterials :many
SELECT pbm.*, rm.name as material_name, rm.sku as material_sku, rm.unit_of_measure, rm.stock_quantity as available_stock
FROM production_batch_materials pbm
JOIN raw_materials rm ON rm.id = pbm.raw_material_id
WHERE pbm.batch_id = $1
ORDER BY rm.name;

-- name: UpdateBatchMaterialConsumed :exec
UPDATE production_batch_materials
SET consumed_quantity = $2
WHERE id = $1;

-- name: CountProductionBatchesByStatus :one
SELECT COUNT(*) FROM production_batches WHERE status = $1;
