-- name: ListProductBOMEntries :many
SELECT pbe.*, rm.name as material_name, rm.sku as material_sku, rm.unit_of_measure
FROM product_bom_entries pbe
JOIN raw_materials rm ON rm.id = pbe.raw_material_id
WHERE pbe.product_id = $1
ORDER BY rm.name;

-- name: CreateProductBOMEntry :one
INSERT INTO product_bom_entries (id, product_id, raw_material_id, quantity, unit_of_measure, is_required, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateProductBOMEntry :one
UPDATE product_bom_entries SET quantity = $2, unit_of_measure = $3, is_required = $4, notes = $5
WHERE id = $1
RETURNING *;

-- name: DeleteProductBOMEntry :exec
DELETE FROM product_bom_entries WHERE id = $1;

-- name: ListOptionBOMEntries :many
SELECT aobe.*, rm.name as material_name, rm.sku as material_sku
FROM attribute_option_bom_entries aobe
JOIN raw_materials rm ON rm.id = aobe.raw_material_id
WHERE aobe.option_id = $1
ORDER BY rm.name;

-- name: CreateOptionBOMEntry :one
INSERT INTO attribute_option_bom_entries (id, option_id, raw_material_id, quantity, unit_of_measure, notes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteOptionBOMEntry :exec
DELETE FROM attribute_option_bom_entries WHERE id = $1;

-- name: ListOptionBOMModifiers :many
SELECT aobm.*, rm.name as material_name
FROM attribute_option_bom_modifiers aobm
JOIN product_bom_entries pbe ON pbe.id = aobm.product_bom_entry_id
JOIN raw_materials rm ON rm.id = pbe.raw_material_id
WHERE aobm.option_id = $1
ORDER BY rm.name;

-- name: CreateOptionBOMModifier :one
INSERT INTO attribute_option_bom_modifiers (id, option_id, product_bom_entry_id, modifier_type, modifier_value, notes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteOptionBOMModifier :exec
DELETE FROM attribute_option_bom_modifiers WHERE id = $1;

-- name: ListVariantBOMOverrides :many
SELECT vbo.*, rm.name as material_name, rm2.name as replaces_material_name
FROM variant_bom_overrides vbo
JOIN raw_materials rm ON rm.id = vbo.raw_material_id
LEFT JOIN raw_materials rm2 ON rm2.id = vbo.replaces_material_id
WHERE vbo.variant_id = $1
ORDER BY rm.name;

-- name: CreateVariantBOMOverride :one
INSERT INTO variant_bom_overrides (id, variant_id, raw_material_id, override_type, replaces_material_id, quantity, unit_of_measure, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: DeleteVariantBOMOverride :exec
DELETE FROM variant_bom_overrides WHERE id = $1;
