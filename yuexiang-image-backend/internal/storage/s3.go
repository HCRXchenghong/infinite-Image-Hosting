package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3CompatibleObjectStore struct {
	client *minio.Client
	bucket string
}

func NewS3CompatibleObjectStore(cfg S3CompatibleConfig) (*S3CompatibleObjectStore, error) {
	endpoint, secure, err := normalizeEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	return &S3CompatibleObjectStore{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3CompatibleObjectStore) Ping(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("s3 bucket %q does not exist", s.bucket)
	}
	return nil
}

func (s *S3CompatibleObjectStore) PutObject(ctx context.Context, input PutObjectInput) error {
	if input.Key == "" {
		return fmt.Errorf("object key is required")
	}
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, s.bucket, input.Key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: input.ContentType,
	})
	return err
}

func (s *S3CompatibleObjectStore) GetObject(ctx context.Context, key string) ([]byte, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}
	defer obj.Close()
	stat, err := obj.Stat()
	if err != nil {
		return nil, "", err
	}
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", err
	}
	return data, stat.ContentType, nil
}

func (s *S3CompatibleObjectStore) DeleteObject(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func normalizeEndpoint(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, fmt.Errorf("s3 endpoint is required")
	}
	if !strings.Contains(raw, "://") {
		return raw, false, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false, err
	}
	return parsed.Host, parsed.Scheme == "https", nil
}
