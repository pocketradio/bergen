// Package storage implements object transfer against S3-compatible storage.
package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client holds a server-side MinIO client scoped to one bucket.
type Client struct {
	client *minio.Client
	bucket string
}

// New creates a storage adapter; credentials are never returned or logged.
func New(endpoint, accessKey, secretKey, bucket string, secure bool) (*Client, error) {
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure})
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}
	return &Client{client: client, bucket: bucket}, nil
}

// EnsureBucket creates the configured bucket when absent; use only in local setup and tests.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := c.client.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("make bucket: %w", err)
	}
	return nil
}

// Put uploads exactly size bytes at a server-chosen key.
func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	if key == "" || size < 0 {
		return fmt.Errorf("invalid object key or size")
	}
	_, err := c.client.PutObject(ctx, c.bucket, key, body, size, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

// Get returns an object reader; StatObject makes missing objects fail before returning.
func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, fmt.Errorf("empty object key")
	}
	obj, err := c.client.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return obj, nil
}
