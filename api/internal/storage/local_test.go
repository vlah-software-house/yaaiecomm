package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --------------------------------------------------------------------------
// Tests for Local.Put
// --------------------------------------------------------------------------

func TestLocal_Put(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir, "/media")
	ctx := context.Background()

	content := "hello, image data"
	url, err := store.Put(ctx, "test-image.jpg", strings.NewReader(content), "image/jpeg")
	if err != nil {
		t.Fatalf("Put: unexpected error: %v", err)
	}

	// Verify URL format.
	if url != "/media/test-image.jpg" {
		t.Errorf("Put: url = %q, want %q", url, "/media/test-image.jpg")
	}

	// Verify file was written.
	data, err := os.ReadFile(filepath.Join(dir, "test-image.jpg"))
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

func TestLocal_Put_NestedKey(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir, "/assets")
	ctx := context.Background()

	url, err := store.Put(ctx, "products/images/thumb.png", strings.NewReader("png data"), "image/png")
	if err != nil {
		t.Fatalf("Put nested key: %v", err)
	}

	if url != "/assets/products/images/thumb.png" {
		t.Errorf("url = %q, want %q", url, "/assets/products/images/thumb.png")
	}

	// Verify subdirectories were created.
	if _, err := os.Stat(filepath.Join(dir, "products", "images", "thumb.png")); err != nil {
		t.Errorf("nested file not found: %v", err)
	}
}

func TestLocal_Put_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir, "/media")
	ctx := context.Background()

	_, err := store.Put(ctx, "empty.jpg", bytes.NewReader(nil), "image/jpeg")
	if err != nil {
		t.Fatalf("Put empty content: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "empty.jpg"))
	if err != nil {
		t.Fatalf("reading empty file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestLocal_Put_InvalidBasePath(t *testing.T) {
	// Use /dev/null as base path — can't create subdirectories under a device file.
	store := NewLocal("/dev/null", "/media")
	ctx := context.Background()

	_, err := store.Put(ctx, "sub/image.jpg", strings.NewReader("data"), "image/jpeg")
	if err == nil {
		t.Fatal("expected error when base path is invalid, got nil")
	}
	if !strings.Contains(err.Error(), "creating directory") {
		t.Errorf("expected directory creation error, got: %v", err)
	}
}

// errReader is a reader that always returns an error after reading some bytes.
type errReader struct{ err error }

func (r *errReader) Read(p []byte) (int, error) { return 0, r.err }

func TestLocal_Put_ReaderError(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir, "/media")
	ctx := context.Background()

	_, err := store.Put(ctx, "broken.jpg", &errReader{err: errors.New("disk exploded")}, "image/jpeg")
	if err == nil {
		t.Fatal("expected error from broken reader, got nil")
	}
	if !strings.Contains(err.Error(), "writing file") {
		t.Errorf("expected writing file error, got: %v", err)
	}

	// Verify the partial file was cleaned up.
	if _, statErr := os.Stat(filepath.Join(dir, "broken.jpg")); !os.IsNotExist(statErr) {
		t.Error("expected partial file to be cleaned up after write error")
	}
}

func TestLocal_Put_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	os.MkdirAll(roDir, 0o755)
	os.Chmod(roDir, 0o444)
	t.Cleanup(func() { os.Chmod(roDir, 0o755) })

	store := NewLocal(roDir, "/media")
	ctx := context.Background()

	_, err := store.Put(ctx, "image.jpg", strings.NewReader("data"), "image/jpeg")
	if err == nil {
		t.Fatal("expected error writing to read-only directory, got nil")
	}
	if !strings.Contains(err.Error(), "creating file") {
		t.Errorf("expected file creation error, got: %v", err)
	}
}

func TestLocal_Delete_PermissionDenied(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "protected")
	os.MkdirAll(subDir, 0o755)

	// Create a file inside, then remove write permission from the directory.
	filePath := filepath.Join(subDir, "locked.jpg")
	os.WriteFile(filePath, []byte("data"), 0o644)
	os.Chmod(subDir, 0o444)
	t.Cleanup(func() { os.Chmod(subDir, 0o755) })

	store := NewLocal(subDir, "/media")
	ctx := context.Background()

	err := store.Delete(ctx, "locked.jpg")
	if err == nil {
		t.Fatal("expected error deleting from read-only directory, got nil")
	}
	if !strings.Contains(err.Error(), "removing file") {
		t.Errorf("expected removing file error, got: %v", err)
	}
}

// --------------------------------------------------------------------------
// Tests for Local.Delete
// --------------------------------------------------------------------------

func TestLocal_Delete(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir, "/media")
	ctx := context.Background()

	// Create a file first.
	path := filepath.Join(dir, "to-delete.jpg")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	err := store.Delete(ctx, "to-delete.jpg")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify file is gone.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestLocal_Delete_NonExistent(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir, "/media")
	ctx := context.Background()

	// Deleting a non-existent file should not error.
	err := store.Delete(ctx, "does-not-exist.jpg")
	if err != nil {
		t.Errorf("Delete non-existent: expected no error, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Tests for Local.PresignGet
// --------------------------------------------------------------------------

func TestLocal_PresignGet(t *testing.T) {
	store := NewLocal("/tmp/media", "/media")
	ctx := context.Background()

	url, err := store.PresignGet(ctx, "test.jpg", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}

	if url != "/media/test.jpg" {
		t.Errorf("PresignGet: url = %q, want %q", url, "/media/test.jpg")
	}
}

func TestLocal_PresignGet_NestedKey(t *testing.T) {
	store := NewLocal("/tmp/media", "/assets")
	ctx := context.Background()

	url, err := store.PresignGet(ctx, "2026/02/image.webp", time.Hour)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}

	if url != "/assets/2026/02/image.webp" {
		t.Errorf("PresignGet: url = %q, want %q", url, "/assets/2026/02/image.webp")
	}
}

// --------------------------------------------------------------------------
// Tests for S3.KeyFromURL (no actual S3 connection needed)
// --------------------------------------------------------------------------

func TestS3_KeyFromURL(t *testing.T) {
	s := &S3{publicURL: "https://cdn.example.com"}

	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "matching public URL prefix",
			url:  "https://cdn.example.com/uuid-image.jpg",
			want: "uuid-image.jpg",
		},
		{
			name: "matching with nested path",
			url:  "https://cdn.example.com/products/images/thumb.webp",
			want: "products/images/thumb.webp",
		},
		{
			name: "non-matching URL returned as-is",
			url:  "https://other-cdn.com/image.jpg",
			want: "https://other-cdn.com/image.jpg",
		},
		{
			name: "empty URL",
			url:  "",
			want: "",
		},
		{
			name: "plain key without prefix",
			url:  "image.jpg",
			want: "image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.KeyFromURL(tt.url)
			if got != tt.want {
				t.Errorf("KeyFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestS3_KeyFromURL_EmptyPublicURL(t *testing.T) {
	s := &S3{publicURL: ""} // private bucket, no public URL

	url := "s3://bucket/key.jpg"
	got := s.KeyFromURL(url)
	if got != url {
		t.Errorf("with empty publicURL, expected URL unchanged, got %q", got)
	}
}

// --------------------------------------------------------------------------
// Tests for Put + Delete round-trip
// --------------------------------------------------------------------------

func TestLocal_PutThenDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir, "/media")
	ctx := context.Background()

	_, err := store.Put(ctx, "roundtrip.gif", strings.NewReader("gif data"), "image/gif")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// File should exist.
	path := filepath.Join(dir, "roundtrip.gif")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist after Put: %v", err)
	}

	err = store.Delete(ctx, "roundtrip.gif")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// File should be gone.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after Delete")
	}
}
