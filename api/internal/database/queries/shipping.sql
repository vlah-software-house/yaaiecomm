-- name: GetShippingConfig :one
SELECT * FROM shipping_config LIMIT 1;

-- name: UpdateShippingConfig :one
UPDATE shipping_config SET
  enabled = $1,
  calculation_method = $2,
  fixed_fee = $3,
  weight_rates = $4,
  size_rates = $5,
  free_shipping_threshold = $6,
  updated_at = $7
WHERE id = (SELECT id FROM shipping_config LIMIT 1)
RETURNING *;

-- name: ListShippingZones :many
SELECT * FROM shipping_zones ORDER BY position;

-- name: GetShippingZone :one
SELECT * FROM shipping_zones WHERE id = $1;

-- name: CreateShippingZone :one
INSERT INTO shipping_zones (
  id, name, countries, calculation_method, rates, position,
  created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING *;

-- name: UpdateShippingZone :one
UPDATE shipping_zones SET
  name = $2,
  countries = $3,
  calculation_method = $4,
  rates = $5,
  position = $6,
  updated_at = $7
WHERE id = $1
RETURNING *;

-- name: DeleteShippingZone :exec
DELETE FROM shipping_zones WHERE id = $1;

-- name: GetShippingZoneForCountry :one
SELECT sz.* FROM shipping_zones sz
JOIN store_shipping_countries ssc ON ssc.shipping_zone_id = sz.id
WHERE ssc.country_code = $1 AND ssc.is_enabled = true;

-- name: ListEnabledShippingCountriesWithZones :many
SELECT
  ssc.country_code,
  ssc.is_enabled,
  ssc.shipping_zone_id,
  ssc.position,
  ec.name AS country_name,
  ec.currency AS country_currency,
  sz.name AS zone_name
FROM store_shipping_countries ssc
JOIN eu_countries ec ON ec.country_code = ssc.country_code
LEFT JOIN shipping_zones sz ON sz.id = ssc.shipping_zone_id
WHERE ssc.is_enabled = true
ORDER BY ssc.position, ec.name;
