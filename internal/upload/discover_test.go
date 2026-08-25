package upload

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupTestFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}
	return dir
}

func TestDiscoverSingleFile(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{"song.mp3": "audio"})
	filePath := filepath.Join(dir, "song.mp3")

	files, err := DiscoverFiles([]string{filePath})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Size != 5 {
		t.Errorf("expected size 5, got %d", files[0].Size)
	}
}

func TestDiscoverReadsID3v1Metadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tagged.mp3")

	id3 := make([]byte, 128)
	copy(id3[0:3], "TAG")
	copy(id3[3:33], "Night Drive")
	copy(id3[33:63], "Ama Radio")
	copy(id3[63:93], "Accra After Dark")
	copy(id3[93:97], "2026")
	id3[125] = 0
	id3[126] = 4
	id3[127] = 17
	payload := append([]byte("fake-mp3-audio-data"), id3...)
	if err := os.WriteFile(path, payload, 0644); err != nil {
		t.Fatal(err)
	}

	files, err := DiscoverFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	metadata := files[0].Metadata
	if metadata.Title != "Night Drive" {
		t.Fatalf("expected title Night Drive, got %q", metadata.Title)
	}
	if metadata.Artist != "Ama Radio" {
		t.Fatalf("expected artist Ama Radio, got %q", metadata.Artist)
	}
	if metadata.Album != "Accra After Dark" {
		t.Fatalf("expected album, got %q", metadata.Album)
	}
	if metadata.Year != 2026 || metadata.Track != 4 {
		t.Fatalf("unexpected year/track: %d/%d", metadata.Year, metadata.Track)
	}
	if metadata.TagFormat != "ID3v1" {
		t.Fatalf("expected ID3v1 format, got %q", metadata.TagFormat)
	}
}

func TestDiscoverIgnoresMissingMetadata(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{"untagged.mp3": "plain audio"})
	files, err := DiscoverFiles([]string{filepath.Join(dir, "untagged.mp3")})
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Metadata.HasValues() {
		t.Fatalf("unexpected metadata: %+v", files[0].Metadata)
	}
}

func TestDiscoverMultipleFiles(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"song1.mp3": "audio1",
		"song2.mp3": "audio2",
	})

	paths := []string{
		filepath.Join(dir, "song1.mp3"),
		filepath.Join(dir, "song2.mp3"),
	}

	files, err := DiscoverFiles(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestDiscoverDirectory(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"track1.mp3":     "audio1",
		"track2.mp3":     "audio2",
		"sub/track3.mp3": "audio3",
	})

	files, err := DiscoverFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	// Check deterministic ordering
	for i := 1; i < len(files); i++ {
		if files[i].AbsolutePath < files[i-1].AbsolutePath {
			t.Error("files should be sorted alphabetically")
		}
	}
}

func TestDiscoverSkipsUnsupportedFiles(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"01 STAY.mp3": "audio",
		"01 STAY.lrc": "lyrics",
		"cover.jpg":   "artwork",
		"notes.txt":   "notes",
		"sub/02.flac": "audio",
	})

	files, err := DiscoverFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 supported audio files, got %d", len(files))
	}
	for _, file := range files {
		if !isSupportedAudioFile(file.AbsolutePath) {
			t.Errorf("discovered unsupported file %q", file.AbsolutePath)
		}
	}

	files, err = DiscoverFiles([]string{
		filepath.Join(dir, "01 STAY.lrc"),
		filepath.Join(dir, "01 STAY.mp3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0].AbsolutePath) != "01 STAY.mp3" {
		t.Fatalf("explicit unsupported file was not skipped: %+v", files)
	}
}

func TestDiscoverDirectoryTrailingSlash(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"track1.mp3": "audio1",
		"track2.mp3": "audio2",
	})

	files1, _ := DiscoverFiles([]string{dir})
	files2, _ := DiscoverFiles([]string{dir + "/"})

	if len(files1) != len(files2) {
		t.Errorf("mismatched lengths: %d vs %d", len(files1), len(files2))
	}
}

func TestDiscoverGlob(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"track1.mp3": "audio1",
		"track2.mp3": "audio2",
		"track1.txt": "text1",
	})

	pattern := filepath.Join(dir, "*.mp3")
	files, err := DiscoverFiles([]string{pattern})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 mp3 files, got %d", len(files))
	}
}

func TestDiscoverRecursiveGlob(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"track1.mp3":           "audio1",
		"sub/track2.mp3":       "audio2",
		"sub/deep/track3.flac": "audio3",
	})

	pattern := filepath.Join(dir, "**/*.mp3")
	files, err := DiscoverFiles([]string{pattern})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 mp3 files from recursive glob, got %d", len(files))
	}
}

func TestDiscoverMixedInputs(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"dir1/song1.mp3": "audio1",
		"dir1/song2.mp3": "audio2",
		"dir2/other.mp3": "audio3",
		"root.mp3":       "audio4",
	})

	dir1 := filepath.Join(dir, "dir1")
	rootFile := filepath.Join(dir, "root.mp3")
	globPattern := filepath.Join(dir, "dir2", "*.mp3")

	files, err := DiscoverFiles([]string{dir1, rootFile, globPattern})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d", len(files))
	}
}

func TestDiscoverDeduplication(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"file.mp3": "audio",
	})

	filePath := filepath.Join(dir, "file.mp3")

	files, err := DiscoverFiles([]string{filePath, filePath})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file after dedup, got %d", len(files))
	}
}

func TestDiscoverDedupDirectoryAndGlob(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"Music/track1.mp3": "audio1",
		"Music/track2.mp3": "audio2",
	})

	musicDir := filepath.Join(dir, "Music")
	globPattern := filepath.Join(dir, "Music", "**/*.mp3")

	files, err := DiscoverFiles([]string{musicDir, globPattern})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files after dedup, got %d", len(files))
	}
}

func TestDiscoverMissingPath(t *testing.T) {
	_, err := DiscoverFiles([]string{"/nonexistent/path/file.mp3"})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestDiscoverEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := DiscoverFiles([]string{dir})
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestDiscoverIgnoresSymlinks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "real.mp3"), []byte("content"), 0644)
	os.Symlink(filepath.Join(dir, "real.mp3"), filepath.Join(dir, "link.mp3"))

	files, err := DiscoverFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (symlink ignored), got %d", len(files))
	}
}

func TestDeterministicOrdering(t *testing.T) {
	dir := setupTestFiles(t, map[string]string{
		"z.mp3": "z",
		"a.mp3": "a",
		"m.mp3": "m",
	})

	pattern := filepath.Join(dir, "*.mp3")
	files1, _ := DiscoverFiles([]string{pattern})
	files2, _ := DiscoverFiles([]string{pattern})

	for i := 0; i < len(files1); i++ {
		if files1[i].AbsolutePath != files2[i].AbsolutePath {
			t.Errorf("order mismatch at index %d", i)
		}
	}

	// Verify they're sorted
	if !sort.SliceIsSorted(files1, func(i, j int) bool {
		return files1[i].AbsolutePath < files1[j].AbsolutePath
	}) {
		t.Error("files should be sorted")
	}
}
