package cli

import (
	"testing"

	"radioplatform-media-ci/internal/upload"
)

func TestFormatTrack(t *testing.T) {
	tests := []struct {
		metadata upload.MediaMetadata
		want     string
	}{
		{metadata: upload.MediaMetadata{}, want: "-"},
		{metadata: upload.MediaMetadata{Track: 4}, want: "4"},
		{metadata: upload.MediaMetadata{Track: 4, TrackTotal: 12}, want: "4/12"},
	}
	for _, test := range tests {
		if got := formatTrack(test.metadata); got != test.want {
			t.Fatalf("formatTrack(%+v) = %q, want %q", test.metadata, got, test.want)
		}
	}
}

func TestMetadataValueFallback(t *testing.T) {
	if got := metadataValue(""); got != "-" {
		t.Fatalf("expected fallback dash, got %q", got)
	}
	if got := metadataValue("Night Drive"); got != "Night Drive" {
		t.Fatalf("unexpected metadata value %q", got)
	}
}
