package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3 stores files in an S3-compatible bucket (AWS, CEPH, MinIO).
type S3 struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	publicURL string // base URL for public access, empty for private-only buckets
}

// S3Config holds the settings for an S3-compatible connection.
type S3Config struct {
	Endpoint       string // e.g. "https://s3.ceph-provider.com"
	Region         string // e.g. "us-east-1" (dummy for CEPH)
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool   // true for CEPH/MinIO
	Bucket         string // bucket name
	PublicURL      string // public base URL for this bucket (empty = private)
}

// NewS3 creates an S3-compatible storage client.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.Bucket,
		publicURL: strings.TrimRight(cfg.PublicURL, "/"),
	}, nil
}

func (s *S3) Put(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}

	if _, err := s.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("putting object %s: %w", key, err)
	}

	if s.publicURL != "" {
		return s.publicURL + "/" + key, nil
	}
	return fmt.Sprintf("s3://%s/%s", s.bucket, key), nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if _, err := s.client.DeleteObject(ctx, input); err != nil {
		return fmt.Errorf("deleting object %s: %w", key, err)
	}
	return nil
}

func (s *S3) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	resp, err := s.presigner.PresignGetObject(ctx, input, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presigning GET for %s: %w", key, err)
	}
	return resp.URL, nil
}

// KeyFromURL extracts the S3 object key from a full public URL.
// Returns the key unchanged if it does not start with the public URL prefix.
func (s *S3) KeyFromURL(url string) string {
	if s.publicURL != "" && strings.HasPrefix(url, s.publicURL+"/") {
		return strings.TrimPrefix(url, s.publicURL+"/")
	}
	return url
}
