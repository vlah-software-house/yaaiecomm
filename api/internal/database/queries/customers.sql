-- name: GetCustomer :one
SELECT * FROM customers WHERE id = $1;

-- name: GetCustomerByEmail :one
SELECT * FROM customers WHERE email = $1;

-- name: ListCustomers :many
SELECT * FROM customers ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountCustomers :one
SELECT COUNT(*) FROM customers;

-- name: CreateCustomer :one
INSERT INTO customers (
  id, email, first_name, last_name, phone, password_hash,
  default_billing_address, default_shipping_address,
  accepts_marketing, stripe_customer_id, vat_number,
  notes, metadata, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)
RETURNING *;

-- name: UpdateCustomer :one
UPDATE customers SET
  first_name = $2, last_name = $3, phone = $4,
  default_billing_address = $5, default_shipping_address = $6,
  accepts_marketing = $7, vat_number = $8, notes = $9, updated_at = $10
WHERE id = $1
RETURNING *;
