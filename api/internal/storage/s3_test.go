package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// newTestS3 creates an S3 struct with a mock HTTP backend for testing.
// The mock server simulates basic S3 API responses.
func newTestS3(t *testing.T, handler http.Handler) (*S3, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(server.URL),
		Region:       "us-east-1",
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("test-key", "test-secret", ""),
	})

	store := &S3{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    "test-bucket",
		publicURL: "https://cdn.example.com",
	}

	return store, server
}

func TestS3_Put_Success(t *testing.T) {
	var capturedKey string
	var capturedContentType string
	var capturedBody string

	store, server := newTestS3(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// S3 PutObject is a PUT request.
		if r.Method == http.MethodPut {
			capturedKey = r.URL.Path
			capturedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			capturedBody = string(body)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	url, err := store.Put(context.Background(), "products/image.webp", strings.NewReader("image data"), "image/webp")
	if err != nil {
		t.Fatalf("Put: unexpected error: %v", err)
	}

	// With a publicURL set, should return the CDN URL.
	if url != "https://cdn.example.com/products/image.webp" {
		t.Errorf("Put URL: got %q, want %q", url, "https://cdn.example.com/products/image.webp")
	}

	if !strings.HasSuffix(capturedKey, "products/image.webp") {
		t.Errorf("S3 key: got %q, want suffix %q", capturedKey, "products/image.webp")
	}
	if capturedContentType != "image/webp" {
		t.Errorf("Content-Type: got %q, want %q", capturedContentType, "image/webp")
	}
	if capturedBody != "image data" {
		t.Errorf("body: got %q, want %q", capturedBody, "image data")
	}
}

func TestS3_Put_NoPublicURL(t *testing.T) {
	store, server := newTestS3(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store.publicURL = "" // No public URL = private bucket.

	url, err := store.Put(context.Background(), "private/file.pdf", strings.NewReader("data"), "application/pdf")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if url != "s3://test-bucket/private/file.pdf" {
		t.Errorf("Put URL (private): got %q, want %q", url, "s3://test-bucket/private/file.pdf")
	}
}

func TestS3_Put_Error(t *testing.T) {
	store, server := newTestS3(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<?xml version="1.0"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
	}))
	defer server.Close()

	_, err := store.Put(context.Background(), "forbidden.jpg", strings.NewReader("data"), "image/jpeg")
	if err == nil {
		t.Fatal("expected error for S3 403, got nil")
	}
	if !strings.Contains(err.Error(), "putting object") {
		t.Errorf("error should wrap with context, got: %v", err)
	}
}

func TestS3_Delete_Success(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	store, server := newTestS3(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := store.Delete(context.Background(), "products/old-image.jpg")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("method: got %q, want DELETE", capturedMethod)
	}
	if !strings.HasSuffix(capturedPath, "products/old-image.jpg") {
		t.Errorf("path: got %q, want suffix %q", capturedPath, "products/old-image.jpg")
	}
}

func TestS3_Delete_Error(t *testing.T) {
	store, server := newTestS3(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<?xml version="1.0"?><Error><Code>InternalError</Code><Message>Server Error</Message></Error>`))
	}))
	defer server.Close()

	err := store.Delete(context.Background(), "error-key.jpg")
	if err == nil {
		t.Fatal("expected error for S3 500, got nil")
	}
	if !strings.Contains(err.Error(), "deleting object") {
		t.Errorf("error should wrap with context, got: %v", err)
	}
}

func TestS3_PresignGet(t *testing.T) {
	store, server := newTestS3(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PresignGet does not make an actual HTTP call — it generates a URL locally.
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	url, err := store.PresignGet(context.Background(), "products/image.webp", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}

	// The presigned URL should contain the bucket and key.
	if !strings.Contains(url, "test-bucket") {
		t.Errorf("presigned URL should contain bucket name, got %q", url)
	}
	if !strings.Contains(url, "products") {
		t.Errorf("presigned URL should contain key path, got %q", url)
	}
	// Should be a signed URL with query parameters.
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("presigned URL should contain X-Amz-Signature, got %q", url)
	}
}

func TestS3_PresignGet_Expiry(t *testing.T) {
	store, server := newTestS3(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Different expiry durations should produce different URLs (different Expires param).
	url1, err := store.PresignGet(context.Background(), "key.jpg", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet 5min: %v", err)
	}

	url2, err := store.PresignGet(context.Background(), "key.jpg", 2*time.Hour)
	if err != nil {
		t.Fatalf("PresignGet 2hr: %v", err)
	}

	// URLs should differ (at minimum in the Expires or X-Amz-Expires parameter).
	if url1 == url2 {
		t.Error("presigned URLs with different expiry should differ")
	}
}

func TestNewS3_Success(t *testing.T) {
	cfg := S3Config{
		Endpoint:       "https://s3.example.com",
		Region:         "eu-west-1",
		AccessKey:      "AKIA...",
		SecretKey:      "secret",
		ForcePathStyle: true,
		Bucket:         "my-bucket",
		PublicURL:      "https://cdn.example.com/",
	}

	s, err := NewS3(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	if s.bucket != "my-bucket" {
		t.Errorf("bucket: got %q, want %q", s.bucket, "my-bucket")
	}
	// PublicURL should have trailing slash trimmed.
	if s.publicURL != "https://cdn.example.com" {
		t.Errorf("publicURL: got %q, want %q", s.publicURL, "https://cdn.example.com")
	}
	if s.client == nil {
		t.Error("client should not be nil")
	}
	if s.presigner == nil {
		t.Error("presigner should not be nil")
	}
}

func TestNewS3_EmptyPublicURL(t *testing.T) {
	cfg := S3Config{
		Endpoint:  "https://s3.example.com",
		Region:    "us-east-1",
		AccessKey: "key",
		SecretKey: "secret",
		Bucket:    "private-bucket",
		PublicURL: "",
	}

	s, err := NewS3(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	if s.publicURL != "" {
		t.Errorf("publicURL: got %q, want empty", s.publicURL)
	}
}
