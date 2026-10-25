package domain

import (
	"errors"
	"strings"
)

// ProfileVersion identifies an immutable server-defined encoding profile revision.
type ProfileVersion string

// EncodeProfile contains the complete, server-controlled output media contract.
type EncodeProfile struct {
	ID              string         `json:"id"`
	Version         ProfileVersion `json:"version"`
	Container       string         `json:"container"`
	VideoCodec      string         `json:"video_codec"`
	Width           int            `json:"width"`
	Height          int            `json:"height"`
	PixelFormat     string         `json:"pixel_format"`
	FrameRate       string         `json:"frame_rate"`
	AllowVariableFR bool           `json:"allow_variable_frame_rate"`
	H264Profile     string         `json:"h264_profile"`
	H264Level       string         `json:"h264_level"`
	VideoBitrate    int64          `json:"video_bitrate"`
	AudioCodec      string         `json:"audio_codec"`
	AudioSampleRate int            `json:"audio_sample_rate"`
	AudioChannels   int            `json:"audio_channels"`
	AudioBitrate    int64          `json:"audio_bitrate"`
}

// ValidateEncodeProfile validates the complete structural profile contract without selecting policy values.
func ValidateEncodeProfile(profile EncodeProfile) error {
	if strings.TrimSpace(profile.ID) == "" || strings.TrimSpace(string(profile.Version)) == "" {
		return errors.New("encoding profile requires an ID and version")
	}
	if strings.TrimSpace(profile.Container) == "" || strings.TrimSpace(profile.VideoCodec) == "" || strings.TrimSpace(profile.AudioCodec) == "" {
		return errors.New("encoding profile requires container and audio/video codecs")
	}
	if profile.Width <= 0 || profile.Height <= 0 || strings.TrimSpace(profile.PixelFormat) == "" || strings.TrimSpace(profile.FrameRate) == "" {
		return errors.New("encoding profile requires dimensions, pixel format, and frame rate")
	}
	if strings.TrimSpace(profile.H264Profile) == "" || strings.TrimSpace(profile.H264Level) == "" || profile.VideoBitrate <= 0 {
		return errors.New("encoding profile requires H.264 profile, level, and video bitrate")
	}
	if profile.AudioSampleRate <= 0 || profile.AudioChannels <= 0 || profile.AudioBitrate <= 0 {
		return errors.New("encoding profile requires audio sample rate, channels, and bitrate")
	}
	return nil
}
