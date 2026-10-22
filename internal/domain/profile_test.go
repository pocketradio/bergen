package domain

import "testing"

// TestEncodeProfileValidation verifies every required versioned profile field is structural rather than policy-selected.
func TestEncodeProfileValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*EncodeProfile)
	}{
		{name: "missing ID", mutate: func(profile *EncodeProfile) { profile.ID = "" }},
		{name: "missing version", mutate: func(profile *EncodeProfile) { profile.Version = "" }},
		{name: "missing container", mutate: func(profile *EncodeProfile) { profile.Container = "" }},
		{name: "zero width", mutate: func(profile *EncodeProfile) { profile.Width = 0 }},
		{name: "missing pixel format", mutate: func(profile *EncodeProfile) { profile.PixelFormat = "" }},
		{name: "missing frame rate", mutate: func(profile *EncodeProfile) { profile.FrameRate = "" }},
		{name: "missing H264 profile", mutate: func(profile *EncodeProfile) { profile.H264Profile = "" }},
		{name: "zero video bitrate", mutate: func(profile *EncodeProfile) { profile.VideoBitrate = 0 }},
		{name: "zero audio sample rate", mutate: func(profile *EncodeProfile) { profile.AudioSampleRate = 0 }},
		{name: "zero audio channels", mutate: func(profile *EncodeProfile) { profile.AudioChannels = 0 }},
		{name: "zero audio bitrate", mutate: func(profile *EncodeProfile) { profile.AudioBitrate = 0 }},
	}

	// iterating over our dummy encode profiles with different invalid properties
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := testProfile()
			test.mutate(&profile)
			if err := ValidateEncodeProfile(profile); err == nil {
				t.Fatal("expected incomplete profile to be rejected")
			}
		})
	}
}

// testProfile returns a complete profile for structural contract tests.
func testProfile() EncodeProfile {
	return EncodeProfile{
		ID: "h264-aac", Version: "1", Container: "mp4", VideoCodec: "h264",
		Width: 1920, Height: 1080, PixelFormat: "yuv420p", FrameRate: "30/1",
		H264Profile: "high", H264Level: "4.1", VideoBitrate: 4_000_000,
		AudioCodec: "aac", AudioSampleRate: 48_000, AudioChannels: 2, AudioBitrate: 128_000,
	}
}
