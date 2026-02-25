package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Local stores files on the local filesystem and serves them via a URL prefix.
// Suitable for development. In production, use S3.
type Local struct {
	basePath  string // filesystem root, e.g. "./media"
	urlPrefix string // URL prefix for served files, e.g. "/media"
}

// NewLocal creates a local filesystem storage.
// basePath is the directory to write files to.
// urlPrefix is the HTTP path prefix used to serve them (e.g. "/media").
func NewLocal(basePath, urlPrefix string) *Local {
	return &Local{
		basePath:  basePath,
		urlPrefix: urlPrefix,
	}
}

func (l *Local) Put(_ context.Context, key string, body io.Reader, _ string) (string, error) {
	dest := filepath.Join(l.basePath, key)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("creating directory for %s: %w", key, err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("creating file %s: %w", key, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, body); err != nil {
		os.Remove(dest)
		return "", fmt.Errorf("writing file %s: %w", key, err)
	}

	return l.urlPrefix + "/" + key, nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	path := filepath.Join(l.basePath, key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing file %s: %w", key, err)
	}
	return nil
}

func (l *Local) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	// Local storage has no access control — just return the URL.
	return l.urlPrefix + "/" + key, nil
}
