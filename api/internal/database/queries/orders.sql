-- name: GetOrder :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByNumber :one
SELECT * FROM orders WHERE order_number = $1;

-- name: ListOrders :many
SELECT * FROM orders
WHERE ($1::text IS NULL OR status = $1::text)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountOrders :one
SELECT COUNT(*) FROM orders
WHERE ($1::text IS NULL OR status = $1::text);

-- name: CreateOrder :one
INSERT INTO orders (
  id, order_number, customer_id, status, email,
  billing_address, shipping_address,
  subtotal, shipping_fee, shipping_extra_fees, discount_amount,
  vat_total, total,
  vat_number, vat_company_name, vat_reverse_charge, vat_country_code,
  stripe_payment_intent_id, stripe_checkout_session_id, payment_status,
  discount_id, coupon_id, discount_breakdown,
  shipping_method, notes, customer_notes, metadata,
  created_at, updated_at
) VALUES (
  $1, nextval('order_number_seq'), $2, $3, $4,
  $5, $6,
  $7, $8, $9, $10,
  $11, $12,
  $13, $14, $15, $16,
  $17, $18, $19,
  $20, $21, $22,
  $23, $24, $25, $26,
  $27, $27
)
RETURNING *;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = $3 WHERE id = $1 RETURNING *;

-- name: UpdateOrderTracking :exec
UPDATE orders SET tracking_number = $2, shipped_at = $3, updated_at = $4 WHERE id = $1;

-- name: ListOrderItems :many
SELECT * FROM order_items WHERE order_id = $1 ORDER BY id;

-- name: CreateOrderItem :one
INSERT INTO order_items (
  id, order_id, product_id, variant_id,
  product_name, variant_name, variant_options, sku,
  quantity, unit_price, total_price,
  vat_rate, vat_rate_type, vat_amount,
  price_includes_vat, net_unit_price, gross_unit_price,
  weight_grams, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING *;

-- name: CreateOrderEvent :exec
INSERT INTO order_events (id, order_id, event_type, from_status, to_status, data, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListOrderEvents :many
SELECT * FROM order_events WHERE order_id = $1 ORDER BY created_at DESC;

-- name: CountOrdersToday :one
SELECT COUNT(*) FROM orders WHERE created_at >= CURRENT_DATE;

-- name: SumRevenueMonth :one
SELECT COALESCE(SUM(total), 0) FROM orders
WHERE created_at >= date_trunc('month', CURRENT_DATE) AND payment_status = 'paid';

-- name: CountPendingOrders :one
SELECT COUNT(*) FROM orders WHERE status = 'pending';
