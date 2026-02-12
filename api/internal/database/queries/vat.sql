-- name: ListVATCategories :many
SELECT * FROM vat_categories ORDER BY position;

-- name: GetVATCategory :one
SELECT * FROM vat_categories WHERE id = $1;

-- name: GetVATCategoryByName :one
SELECT * FROM vat_categories WHERE name = $1;

-- name: ListActiveVATRates :many
SELECT * FROM vat_rates WHERE valid_to IS NULL ORDER BY country_code, rate_type;

-- name: ListVATRatesByCountry :many
SELECT * FROM vat_rates WHERE country_code = $1 AND valid_to IS NULL ORDER BY rate_type;

-- name: GetVATRate :one
SELECT * FROM vat_rates WHERE country_code = $1 AND rate_type = $2 AND valid_to IS NULL;

-- name: ListEUCountries :many
SELECT * FROM eu_countries WHERE is_eu_member = true ORDER BY name;

-- name: GetEUCountry :one
SELECT * FROM eu_countries WHERE country_code = $1;

-- name: ListEnabledShippingCountries :many
SELECT ec.* FROM eu_countries ec
JOIN store_shipping_countries ssc ON ssc.country_code = ec.country_code
WHERE ssc.is_enabled = true
ORDER BY ec.name;

-- name: ListStoreShippingCountries :many
SELECT ssc.*, ec.name as country_name FROM store_shipping_countries ssc
JOIN eu_countries ec ON ec.country_code = ssc.country_code
ORDER BY ssc.position, ec.name;

-- name: SetShippingCountryEnabled :exec
UPDATE store_shipping_countries SET is_enabled = $2 WHERE country_code = $1;

-- name: GetStoreSettings :one
SELECT * FROM store_settings LIMIT 1;

-- name: UpdateStoreVATSettings :exec
UPDATE store_settings SET
  vat_enabled = $1, vat_number = $2, vat_country_code = $3,
  vat_prices_include_vat = $4, vat_default_category = $5,
  vat_b2b_reverse_charge_enabled = $6, updated_at = $7
WHERE id = (SELECT id FROM store_settings LIMIT 1);

-- name: ListProductVATOverrides :many
SELECT pvo.*, vc.name as category_name, vc.display_name as category_display_name,
       ec.name as country_name
FROM product_vat_overrides pvo
JOIN vat_categories vc ON vc.id = pvo.vat_category_id
JOIN eu_countries ec ON ec.country_code = pvo.country_code
WHERE pvo.product_id = $1
ORDER BY ec.name;

-- name: UpsertProductVATOverride :one
INSERT INTO product_vat_overrides (id, product_id, country_code, vat_category_id, notes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (product_id, country_code)
DO UPDATE SET vat_category_id = EXCLUDED.vat_category_id, notes = EXCLUDED.notes, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteProductVATOverride :exec
DELETE FROM product_vat_overrides WHERE id = $1;
