-- ============================================================
-- Global Attributes
-- ============================================================

-- name: ListGlobalAttributes :many
SELECT * FROM global_attributes ORDER BY position, name;

-- name: ListGlobalAttributesByCategory :many
SELECT * FROM global_attributes WHERE category = $1 ORDER BY position, name;

-- name: GetGlobalAttribute :one
SELECT * FROM global_attributes WHERE id = $1;

-- name: GetGlobalAttributeByName :one
SELECT * FROM global_attributes WHERE name = $1;

-- name: CreateGlobalAttribute :one
INSERT INTO global_attributes (id, name, display_name, description, attribute_type, category, position, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateGlobalAttribute :one
UPDATE global_attributes SET
  name = $2, display_name = $3, description = $4,
  attribute_type = $5, category = $6, position = $7,
  is_active = $8, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteGlobalAttribute :exec
DELETE FROM global_attributes WHERE id = $1;

-- name: CountGlobalAttributeUsage :one
SELECT COUNT(*) FROM product_global_attribute_links WHERE global_attribute_id = $1;

-- ============================================================
-- Global Attribute Metadata Fields
-- ============================================================

-- name: ListMetadataFields :many
SELECT * FROM global_attribute_metadata_fields
WHERE global_attribute_id = $1
ORDER BY position, field_name;

-- name: GetMetadataField :one
SELECT * FROM global_attribute_metadata_fields WHERE id = $1;

-- name: CreateMetadataField :one
INSERT INTO global_attribute_metadata_fields (
  id, global_attribute_id, field_name, display_name, field_type,
  is_required, default_value, select_options, help_text, position
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateMetadataField :one
UPDATE global_attribute_metadata_fields SET
  field_name = $2, display_name = $3, field_type = $4,
  is_required = $5, default_value = $6, select_options = $7,
  help_text = $8, position = $9
WHERE id = $1
RETURNING *;

-- name: DeleteMetadataField :exec
DELETE FROM global_attribute_metadata_fields WHERE id = $1;

-- ============================================================
-- Global Attribute Options
-- ============================================================

-- name: ListGlobalAttributeOptions :many
SELECT * FROM global_attribute_options
WHERE global_attribute_id = $1
ORDER BY position;

-- name: GetGlobalAttributeOption :one
SELECT * FROM global_attribute_options WHERE id = $1;

-- name: CreateGlobalAttributeOption :one
INSERT INTO global_attribute_options (
  id, global_attribute_id, value, display_value, color_hex,
  image_url, metadata, position, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateGlobalAttributeOption :one
UPDATE global_attribute_options SET
  value = $2, display_value = $3, color_hex = $4, image_url = $5,
  metadata = $6, position = $7, is_active = $8, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteGlobalAttributeOption :exec
DELETE FROM global_attribute_options WHERE id = $1;

-- ============================================================
-- Product Global Attribute Links
-- ============================================================

-- name: ListProductGlobalAttributeLinks :many
SELECT * FROM product_global_attribute_links
WHERE product_id = $1
ORDER BY position;

-- name: GetProductGlobalAttributeLink :one
SELECT * FROM product_global_attribute_links WHERE id = $1;

-- name: CreateProductGlobalAttributeLink :one
INSERT INTO product_global_attribute_links (
  id, product_id, global_attribute_id, role_name, role_display_name,
  position, affects_pricing, affects_shipping,
  price_modifier_field, weight_modifier_field
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateProductGlobalAttributeLink :one
UPDATE product_global_attribute_links SET
  role_name = $2, role_display_name = $3, position = $4,
  affects_pricing = $5, affects_shipping = $6,
  price_modifier_field = $7, weight_modifier_field = $8
WHERE id = $1
RETURNING *;

-- name: DeleteProductGlobalAttributeLink :exec
DELETE FROM product_global_attribute_links WHERE id = $1;

-- name: ListProductsUsingGlobalAttribute :many
SELECT DISTINCT p.id, p.name, p.slug, p.status
FROM products p
JOIN product_global_attribute_links pgal ON pgal.product_id = p.id
WHERE pgal.global_attribute_id = $1
ORDER BY p.name;

-- ============================================================
-- Product Global Option Selections
-- ============================================================

-- name: ListOptionSelections :many
SELECT * FROM product_global_option_selections
WHERE link_id = $1
ORDER BY position_override NULLS LAST;

-- name: CreateOptionSelection :one
INSERT INTO product_global_option_selections (
  id, link_id, global_option_id, price_modifier,
  weight_modifier_grams, position_override
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteOptionSelection :exec
DELETE FROM product_global_option_selections WHERE id = $1;

-- name: DeleteAllOptionSelections :exec
DELETE FROM product_global_option_selections WHERE link_id = $1;

-- ============================================================
-- Variant Global Options
-- ============================================================

-- name: ListVariantGlobalOptions :many
SELECT pvgo.variant_id, pvgo.link_id, pvgo.global_option_id,
       ga.name AS attribute_name, ga.display_name AS attribute_display_name,
       gao.value AS option_value, gao.display_value AS option_display_value,
       pgal.role_name, pgal.role_display_name
FROM product_variant_global_options pvgo
JOIN product_global_attribute_links pgal ON pgal.id = pvgo.link_id
JOIN global_attributes ga ON ga.id = pgal.global_attribute_id
JOIN global_attribute_options gao ON gao.id = pvgo.global_option_id
WHERE pvgo.variant_id = $1
ORDER BY pgal.position;

-- name: SetVariantGlobalOption :exec
INSERT INTO product_variant_global_options (variant_id, link_id, global_option_id)
VALUES ($1, $2, $3)
ON CONFLICT (variant_id, link_id) DO UPDATE SET global_option_id = EXCLUDED.global_option_id;

-- name: DeleteVariantGlobalOptions :exec
DELETE FROM product_variant_global_options WHERE variant_id = $1;
