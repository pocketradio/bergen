// Package ports defines small media, storage and time boundaries for future consumers.
package ports

import (
	"context"
	"io"
	"time"
)

// MediaInfo contains the minimum inspected fields needed for initial profile checks.
type MediaInfo struct {
	Duration     time.Duration
	Container    string
	VideoCodec   string
	AudioCodec   string
	VideoStreams int
	AudioStreams int
}

// EncodeSpec contains server-defined output settings; it never carries raw FFmpeg arguments.
type EncodeSpec struct {
	VideoCodec string
	AudioCodec string
	Container  string
}

// Storage transfers objects using server credentials, not worker credentials.
// Scoped presigned URLs can be added at the API consumer boundary in Phase 1.
type Storage interface {
	Put(ctx context.Context, key string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}

// Prober inspects media with a cancellable operation.
type Prober interface {
	Probe(ctx context.Context, path string) (MediaInfo, error)
}

// Encoder encodes a local file using typed server settings.
type Encoder interface {
	Encode(ctx context.Context, input, output string, spec EncodeSpec) error
}

// Clock provides deterministic current time for lease and retry tests.
type Clock interface{ Now() time.Time }

// IDGen produces unique identifiers for durable attempts and other entities.
type IDGen interface{ NewID() (string, error) }
