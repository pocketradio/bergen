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
	// get MinIO configuration
	endpoint := os.Getenv("BERGEN_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set BERGEN_TEST_S3_ENDPOINT for MinIO integration")
	}

	// set a timeout and create the storage client
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, err := New(endpoint, os.Getenv("BERGEN_TEST_S3_ACCESS_KEY"), os.Getenv("BERGEN_TEST_S3_SECRET_KEY"), "bergen-test", false)
	if err != nil {
		t.Fatal(err)
	}

	// ensure the bucket exists
	if err := c.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}

	// generate a unique object key
	id, err := (ports.RandomIDGen{}).NewID()
	if err != nil {
		t.Fatal(err)
	}
	key := "tests/" + id

	// delete the object after the test
	t.Cleanup(func() { _ = c.client.RemoveObject(context.Background(), c.bucket, key, minio.RemoveObjectOptions{}) })

	// upload a fixed payload
	payload := []byte("round trip")
	if err := c.Put(ctx, key, bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatal(err)
	}

	// download the same object
	reader, err := c.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	// close the reader and report any errors
	defer func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close object reader: %v", err)
		}
	}()

	// read the downloaded bytes
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	// verify the uploaded and downloaded bytes match
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}
