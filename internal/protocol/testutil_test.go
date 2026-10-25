package protocol

import "github.com/pocketradio/bergen/internal/domain"

// testProfile returns a complete profile for protocol contract tests.
func testProfile() domain.EncodeProfile {
	return domain.EncodeProfile{
		ID: "h264-aac", Version: "1", Container: "mp4", VideoCodec: "h264",
		Width: 1920, Height: 1080, PixelFormat: "yuv420p", FrameRate: "30/1",
		H264Profile: "high", H264Level: "4.1", VideoBitrate: 4_000_000,
		AudioCodec: "aac", AudioSampleRate: 48_000, AudioChannels: 2, AudioBitrate: 128_000,
	}
}
