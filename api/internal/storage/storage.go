package storage

import (
	"context"
	"io"
	"time"
)

// Storage abstracts file storage operations. Implementations handle local
// filesystem or S3-compatible object storage (CEPH, MinIO, etc.).
type Storage interface {
	// Put uploads content and returns the publicly accessible URL.
	// key is the object path (e.g. "products/{id}/{uuid}-file.webp").
	Put(ctx context.Context, key string, body io.Reader, contentType string) (url string, err error)

	// Delete removes the object at key.
	Delete(ctx context.Context, key string) error

	// PresignGet returns a time-limited URL for private objects.
	// Implementations on public buckets may simply return the permanent URL.
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}
