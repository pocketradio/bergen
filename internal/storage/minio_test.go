package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/pocketradio/bergen/internal/ports"
)

// TestRoundTrip exercises real MinIO when make test supplies integration settings.
func TestRoundTrip(t *testing.T) {
	endpoint := os.Getenv("BERGEN_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set BERGEN_TEST_S3_ENDPOINT for MinIO integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, err := New(endpoint, os.Getenv("BERGEN_TEST_S3_ACCESS_KEY"), os.Getenv("BERGEN_TEST_S3_SECRET_KEY"), "bergen-test", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	id, err := (ports.RandomIDGen{}).NewID()
	if err != nil {
		t.Fatal(err)
	}
	key := "tests/" + id
	t.Cleanup(func() { _ = c.client.RemoveObject(context.Background(), c.bucket, key, minio.RemoveObjectOptions{}) })
	payload := []byte("round trip")
	if err := c.Put(ctx, key, bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatal(err)
	}
	reader, err := c.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close object reader: %v", err)
		}
	}()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}
