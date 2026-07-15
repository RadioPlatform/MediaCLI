package upload

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanDirectFileMapsToMediaRoot(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "song.mp3")
	os.WriteFile(filePath, []byte("content"), 0644)

	files, _ := DiscoverFiles([]string{filePath})
	plan, err := BuildPlan(files, []string{filePath}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(plan.Items))
	}
	if plan.Items[0].DestinationFolder != "" {
		t.Errorf("expected media root for direct file, got %q", plan.Items[0].DestinationFolder)
	}
	if plan.Items[0].DestinationName != "song.mp3" {
		t.Errorf("expected song.mp3, got %s", plan.Items[0].DestinationName)
	}
}

func TestPlanDirectoryBasenameMapsToFolder(t *testing.T) {
	dir := t.TempDir()
	musicDir := filepath.Join(dir, "NewReleases")
	os.MkdirAll(musicDir, 0755)
	os.WriteFile(filepath.Join(musicDir, "track1.mp3"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(musicDir, "track2.mp3"), []byte("b"), 0644)

	files, _ := DiscoverFiles([]string{musicDir})
	plan, err := BuildPlan(files, []string{musicDir}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(plan.Items))
	}
	if plan.Items[0].DestinationFolder != "NewReleases" {
		t.Errorf("expected NewReleases folder, got %q", plan.Items[0].DestinationFolder)
	}
}

func TestPlanHomeDirectoryArgumentMapsToFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	musicDir := filepath.Join(home, "Music")
	if err := os.MkdirAll(musicDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(musicDir, "track.mp3"), []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := DiscoverFiles([]string{"~/Music"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(files, []string{"~/Music"}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Items[0].DestinationFolder; got != "Music" {
		t.Fatalf("expected Music destination, got %q", got)
	}
}

func TestPlanExplicitFolderOverridesAll(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "song.mp3")
	os.WriteFile(file1, []byte("content"), 0644)

	files, _ := DiscoverFiles([]string{file1})
	plan, err := BuildPlan(files, []string{file1}, PlanOptions{
		StationUUID:    "station-uuid",
		StationName:    "Test Station",
		ExplicitFolder: "High Rotation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Items[0].DestinationFolder != "High Rotation" {
		t.Errorf("expected High Rotation, got %q", plan.Items[0].DestinationFolder)
	}
}

func TestPlanNestedDirectoryFlattening(t *testing.T) {
	dir := t.TempDir()
	musicDir := filepath.Join(dir, "Music")
	os.MkdirAll(filepath.Join(musicDir, "album-one", "sub"), 0755)
	os.WriteFile(filepath.Join(musicDir, "album-one", "track1.mp3"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(musicDir, "album-one", "sub", "track2.mp3"), []byte("b"), 0644)

	files, _ := DiscoverFiles([]string{musicDir})
	plan, err := BuildPlan(files, []string{musicDir}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(plan.Items))
	}
	for _, item := range plan.Items {
		if item.DestinationFolder != "Music" {
			t.Errorf("expected folder Music, got %q", item.DestinationFolder)
		}
	}
}

func TestPlanMultipleDirectoriesToSeparateFolders(t *testing.T) {
	dir := t.TempDir()
	musicDir := filepath.Join(dir, "Music")
	jinglesDir := filepath.Join(dir, "Jingles")
	os.MkdirAll(musicDir, 0755)
	os.MkdirAll(jinglesDir, 0755)
	os.WriteFile(filepath.Join(musicDir, "song.mp3"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(jinglesDir, "jingle.mp3"), []byte("b"), 0644)

	files, _ := DiscoverFiles([]string{musicDir, jinglesDir})
	plan, err := BuildPlan(files, []string{musicDir, jinglesDir}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(plan.Items))
	}

	// One should be in Music, one in Jingles
	hasMusic := false
	hasJingles := false
	for _, item := range plan.Items {
		if item.DestinationFolder == "Music" {
			hasMusic = true
		}
		if item.DestinationFolder == "Jingles" {
			hasJingles = true
		}
	}
	if !hasMusic || !hasJingles {
		t.Errorf("expected both Music and Jingles folders: Music=%v, Jingles=%v", hasMusic, hasJingles)
	}
}

func TestPlanRelativePaths(t *testing.T) {
	dir := t.TempDir()
	musicDir := filepath.Join(dir, "Music")
	os.MkdirAll(filepath.Join(musicDir, "album"), 0755)
	os.WriteFile(filepath.Join(musicDir, "album", "track.mp3"), []byte("a"), 0644)

	files, _ := DiscoverFiles([]string{musicDir})
	plan, _ := BuildPlan(files, []string{musicDir}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	if len(plan.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(plan.Items))
	}
	expected := filepath.Join("album", "track.mp3")
	if plan.Items[0].RelativePath != expected {
		t.Errorf("expected relative path %q, got %q", expected, plan.Items[0].RelativePath)
	}
}

func TestPlanJingleFlag(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "jingle.mp3")
	os.WriteFile(filePath, []byte("content"), 0644)

	files, _ := DiscoverFiles([]string{filePath})
	plan, err := BuildPlan(files, []string{filePath}, PlanOptions{
		StationUUID:  "station-uuid",
		StationName:  "Test Station",
		GlobalJingle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Items[0].IsJingle {
		t.Error("expected IsJingle=true")
	}
}

func TestPlanTotalBytes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.mp3"), []byte("AAAAA"), 0644)
	os.WriteFile(filepath.Join(dir, "b.mp3"), []byte("BBBBB"), 0644)

	files, _ := DiscoverFiles([]string{dir})
	plan, _ := BuildPlan(files, []string{dir}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	if plan.TotalBytes != 10 {
		t.Errorf("expected 10 bytes, got %d", plan.TotalBytes)
	}
}

func TestPlanGlobbedFilesMapToMediaRoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.mp3"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.mp3"), []byte("b"), 0644)

	pattern := filepath.Join(dir, "*.mp3")
	files, _ := DiscoverFiles([]string{pattern})
	plan, _ := BuildPlan(files, []string{pattern}, PlanOptions{
		StationUUID: "station-uuid",
		StationName: "Test Station",
	})
	for _, item := range plan.Items {
		if item.DestinationFolder != "" {
			t.Errorf("expected media root for globbed files, got %q", item.DestinationFolder)
		}
	}
}

func TestPlanStationUUID(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "song.mp3")
	os.WriteFile(filePath, []byte("a"), 0644)

	files, _ := DiscoverFiles([]string{filePath})
	plan, _ := BuildPlan(files, []string{filePath}, PlanOptions{
		StationUUID: "specific-station-uuid",
		StationName: "Specific Station",
	})
	if plan.StationUUID != "specific-station-uuid" {
		t.Errorf("expected specific-station-uuid, got %s", plan.StationUUID)
	}
}
