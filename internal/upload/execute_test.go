package upload

import "testing"

func TestTallyResultsCountsOnlySuccessfulBytes(t *testing.T) {
	summary := &UploadSummary{TotalBytes: 30}
	results := []UploadResult{
		{Item: UploadItem{Size: 10}, Success: true},
		{Item: UploadItem{Size: 20}, Success: false},
	}

	tallyResults(summary, results)

	if summary.SuccessCount != 1 || summary.FailedCount != 1 {
		t.Fatalf("unexpected counts: succeeded=%d failed=%d", summary.SuccessCount, summary.FailedCount)
	}
	if summary.UploadedBytes != 10 {
		t.Fatalf("expected 10 uploaded bytes, got %d", summary.UploadedBytes)
	}
	if summary.TotalBytes != 30 {
		t.Fatalf("planned total changed: got %d", summary.TotalBytes)
	}
}

func TestProgressNameTruncatesLongFilenames(t *testing.T) {
	short := "song.mp3"
	if got := progressName(short); got != short {
		t.Fatalf("short name changed: %q", got)
	}
	long := "very-long-radio-track-filename-for-upload.mp3"
	got := progressName(long)
	if len([]rune(got)) != 28 || got[len(got)-3:] != "…" {
		t.Fatalf("unexpected truncated name: %q", got)
	}
}

func TestAggregateProgressName(t *testing.T) {
	activeUploads := map[int]string{
		2: "third-song.mp3",
		0: "first-song.mp3",
		1: "second-song.mp3",
	}

	if got, want := aggregateProgressName(2, 8, activeUploads), "Uploading 2/8 files: first-song.mp3 +2"; got != want {
		t.Fatalf("aggregateProgressName() = %q, want %q", got, want)
	}
	if got, want := aggregateProgressName(8, 8, nil), "Uploading 8/8 files"; got != want {
		t.Fatalf("aggregateProgressName() = %q, want %q", got, want)
	}
}
