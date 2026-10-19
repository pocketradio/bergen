package ports

import (
	"crypto/rand"
	"fmt"
	"time"
)

// RealClock reads wall-clock time in UTC.
type RealClock struct{}

// Now returns the current UTC time.
func (RealClock) Now() time.Time { return time.Now().UTC() }

// RandomIDGen uses cryptographic randomness for unpredictable IDs.
type RandomIDGen struct{}

// NewID returns a random 128-bit hexadecimal identifier or a randomness error.
func (RandomIDGen) NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
