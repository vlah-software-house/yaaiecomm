-- name: GetDiscount :one
SELECT * FROM discounts WHERE id = $1;

-- name: ListActiveDiscounts :many
SELECT * FROM discounts
WHERE is_active = true
AND (starts_at IS NULL OR starts_at <= NOW())
AND (ends_at IS NULL OR ends_at > NOW())
ORDER BY priority DESC;

-- name: ListDiscounts :many
SELECT * FROM discounts ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CreateDiscount :one
INSERT INTO discounts (
  id, name, type, value, scope, minimum_amount, maximum_discount,
  starts_at, ends_at, is_active, priority, stackable, conditions,
  created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)
RETURNING *;

-- name: UpdateDiscount :one
UPDATE discounts SET
  name = $2, type = $3, value = $4, scope = $5,
  minimum_amount = $6, maximum_discount = $7,
  starts_at = $8, ends_at = $9, is_active = $10,
  priority = $11, stackable = $12, conditions = $13, updated_at = $14
WHERE id = $1
RETURNING *;

-- name: GetCoupon :one
SELECT * FROM coupons WHERE id = $1;

-- name: GetCouponByCode :one
SELECT * FROM coupons WHERE code = $1;

-- name: ListCoupons :many
SELECT c.*, d.name as discount_name FROM coupons c
JOIN discounts d ON d.id = c.discount_id
ORDER BY c.created_at DESC LIMIT $1 OFFSET $2;

-- name: CreateCoupon :one
INSERT INTO coupons (
  id, code, discount_id, usage_limit, usage_limit_per_customer,
  usage_count, starts_at, ends_at, is_active, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, 0, $6, $7, $8, $9, $9)
RETURNING *;

-- name: IncrementCouponUsage :exec
UPDATE coupons SET usage_count = usage_count + 1, updated_at = $2 WHERE id = $1;
