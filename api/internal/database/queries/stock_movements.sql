-- name: CreateStockMovement :one
INSERT INTO stock_movements (
  id, entity_type, entity_id, movement_type,
  quantity_change, quantity_before, quantity_after,
  reference_type, reference_id, unit_cost, notes,
  created_by, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: ListStockMovements :many
SELECT * FROM stock_movements
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
