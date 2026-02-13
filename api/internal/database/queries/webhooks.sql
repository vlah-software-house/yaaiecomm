-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (url, secret, events, description, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWebhookEndpoint :one
SELECT * FROM webhook_endpoints WHERE id = $1;

-- name: ListWebhookEndpoints :many
SELECT * FROM webhook_endpoints ORDER BY created_at DESC;

-- name: ListActiveWebhookEndpoints :many
SELECT * FROM webhook_endpoints WHERE is_active = true ORDER BY created_at DESC;

-- name: UpdateWebhookEndpoint :one
UPDATE webhook_endpoints
SET url = $2, events = $3, description = $4, is_active = $5, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteWebhookEndpoint :exec
DELETE FROM webhook_endpoints WHERE id = $1;

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (endpoint_id, event_type, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateWebhookDeliverySuccess :exec
UPDATE webhook_deliveries
SET response_status = $2, response_body = $3, delivered_at = NOW(), attempt = attempt + 1
WHERE id = $1;

-- name: UpdateWebhookDeliveryFailed :exec
UPDATE webhook_deliveries
SET response_status = $2, response_body = $3, attempt = attempt + 1, next_retry_at = $4
WHERE id = $1;

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE endpoint_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPendingWebhookDeliveries :many
SELECT wd.*, we.url, we.secret
FROM webhook_deliveries wd
JOIN webhook_endpoints we ON we.id = wd.endpoint_id
WHERE wd.delivered_at IS NULL
  AND (wd.next_retry_at IS NULL OR wd.next_retry_at <= NOW())
  AND wd.attempt < 5
ORDER BY wd.created_at ASC
LIMIT $1;

-- name: CountWebhookDeliveries :one
SELECT COUNT(*) FROM webhook_deliveries WHERE endpoint_id = $1;
