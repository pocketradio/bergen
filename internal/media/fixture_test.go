package media

import (
	"context"

	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// tinyMP4 generates a short H.264/AAC clip in the test temp directory.
func tinyMP4(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	path := filepath.Join(t.TempDir(), "tiny.mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=64x64:rate=10", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "1", "-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p", "-y", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate tiny MP4: %v: %s", err, output)
	}
	return path
}

// TestTinyMP4 verifies the fixture can be generated and probed as H.264/AAC.
func TestTinyMP4(t *testing.T) {
	path := tinyMP4(t)
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "h264\naac\n" {
		t.Fatalf("unexpected codecs: %q", got)
	}
}
