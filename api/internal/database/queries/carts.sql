-- name: CreateCart :one
INSERT INTO carts (
  id, customer_id, email, country_code, vat_number, coupon_code, expires_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
RETURNING *;

-- name: GetCart :one
SELECT * FROM carts WHERE id = $1;

-- name: UpdateCart :one
UPDATE carts SET
  email = $2, country_code = $3, vat_number = $4, coupon_code = $5, updated_at = $6
WHERE id = $1
RETURNING *;

-- name: DeleteExpiredCarts :exec
DELETE FROM carts WHERE expires_at < NOW();

-- name: AddCartItem :one
INSERT INTO cart_items (id, cart_id, variant_id, quantity, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (cart_id, variant_id) DO UPDATE SET
  quantity = cart_items.quantity + EXCLUDED.quantity,
  updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: GetCartItems :many
SELECT
  ci.id, ci.cart_id, ci.variant_id, ci.quantity,
  ci.created_at, ci.updated_at,
  pv.sku AS variant_sku,
  pv.price AS variant_price,
  pv.stock_quantity AS variant_stock,
  pv.weight_grams AS variant_weight_grams,
  pv.is_active AS variant_is_active,
  p.id AS product_id,
  p.name AS product_name,
  p.slug AS product_slug,
  p.base_price AS product_base_price,
  p.status AS product_status,
  p.vat_category_id AS product_vat_category_id
FROM cart_items ci
JOIN product_variants pv ON pv.id = ci.variant_id
JOIN products p ON p.id = pv.product_id
WHERE ci.cart_id = $1
ORDER BY ci.created_at;

-- name: UpdateCartItemQuantity :one
UPDATE cart_items SET quantity = $2, updated_at = $3
WHERE id = $1
RETURNING *;

-- name: RemoveCartItem :exec
DELETE FROM cart_items WHERE id = $1;

-- name: ClearCart :exec
DELETE FROM cart_items WHERE cart_id = $1;
