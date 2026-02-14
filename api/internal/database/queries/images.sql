-- images.sql — queries for product images and media assets

-- name: CreateMediaAsset :one
INSERT INTO media_assets (
    id, filename, original_filename, content_type, size_bytes, url, width, height, metadata, created_at
) VALUES (
    @id, @filename, @original_filename, @content_type, @size_bytes, @url, @width, @height, @metadata, @created_at
)
RETURNING *;

-- name: GetMediaAsset :one
SELECT * FROM media_assets WHERE id = @id;

-- name: DeleteMediaAsset :exec
DELETE FROM media_assets WHERE id = @id;

-- name: CreateProductImage :one
INSERT INTO product_images (
    id, product_id, variant_id, option_id, url, alt_text, position, is_primary, created_at
) VALUES (
    @id, @product_id, @variant_id, @option_id, @url, @alt_text, @position, @is_primary, @created_at
)
RETURNING *;

-- name: GetProductImage :one
SELECT * FROM product_images WHERE id = @id;

-- name: ListProductImagesByProduct :many
SELECT * FROM product_images
WHERE product_id = @product_id
ORDER BY position ASC, created_at ASC;

-- name: ListProductImagesByVariant :many
SELECT * FROM product_images
WHERE variant_id = @variant_id
ORDER BY position ASC, created_at ASC;

-- name: GetPrimaryImageByProduct :one
SELECT * FROM product_images
WHERE product_id = @product_id AND is_primary = true
LIMIT 1;

-- name: UpdateProductImage :exec
UPDATE product_images
SET alt_text = @alt_text,
    position = @position,
    is_primary = @is_primary
WHERE id = @id;

-- name: UpdateProductImageVariant :exec
UPDATE product_images
SET variant_id = @variant_id,
    option_id = @option_id
WHERE id = @id;

-- name: DeleteProductImage :exec
DELETE FROM product_images WHERE id = @id;

-- name: UnsetPrimaryProductImages :exec
UPDATE product_images
SET is_primary = false
WHERE product_id = @product_id AND is_primary = true;

-- name: SetPrimaryProductImage :exec
UPDATE product_images
SET is_primary = true
WHERE id = @id;

-- name: CountProductImagesByProduct :one
SELECT COUNT(*) FROM product_images WHERE product_id = @product_id;

-- name: UpdateProductImagePosition :exec
UPDATE product_images
SET position = @position
WHERE id = @id;
