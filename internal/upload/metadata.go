package upload

import (
	"os"
	"strings"

	"github.com/dhowden/tag"
)

// MediaMetadata contains best-effort embedded audio metadata. Missing or
// malformed tags never prevent a file from being uploaded.
type MediaMetadata struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Year        int    `json:"year,omitempty"`
	Track       int    `json:"track,omitempty"`
	TrackTotal  int    `json:"track_total,omitempty"`
	TagFormat   string `json:"tag_format,omitempty"`
	AudioFormat string `json:"audio_format,omitempty"`
}

func (m MediaMetadata) HasValues() bool {
	return m.Title != "" || m.Artist != "" || m.Album != "" ||
		m.AlbumArtist != "" || m.Genre != "" || m.Year != 0 ||
		m.Track != 0 || m.TrackTotal != 0
}

func readMediaMetadata(path string) MediaMetadata {
	file, err := os.Open(path)
	if err != nil {
		return MediaMetadata{}
	}
	defer file.Close()

	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return MediaMetadata{}
	}
	track, trackTotal := metadata.Track()

	return MediaMetadata{
		Title:       strings.TrimSpace(metadata.Title()),
		Artist:      strings.TrimSpace(metadata.Artist()),
		Album:       strings.TrimSpace(metadata.Album()),
		AlbumArtist: strings.TrimSpace(metadata.AlbumArtist()),
		Genre:       strings.TrimSpace(metadata.Genre()),
		Year:        metadata.Year(),
		Track:       track,
		TrackTotal:  trackTotal,
		TagFormat:   string(metadata.Format()),
		AudioFormat: string(metadata.FileType()),
	}
}
