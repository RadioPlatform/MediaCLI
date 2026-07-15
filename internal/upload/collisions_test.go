package upload

import (
	"testing"
)

func TestCollisionSameFolderSameName(t *testing.T) {
	items := []UploadItem{
		{LocalPath: "/music/a/intro.mp3", DestinationFolder: "Music", DestinationName: "intro.mp3"},
		{LocalPath: "/music/b/intro.mp3", DestinationFolder: "Music", DestinationName: "intro.mp3"},
	}

	collisions := DetectCollisions(items)
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}
	if collisions[0].DestinationName != "intro.mp3" {
		t.Errorf("expected intro.mp3, got %s", collisions[0].DestinationName)
	}
	if len(collisions[0].ConflictingPaths) != 2 {
		t.Errorf("expected 2 conflicting paths, got %d", len(collisions[0].ConflictingPaths))
	}
}

func TestCollisionDifferentFoldersNoCollision(t *testing.T) {
	items := []UploadItem{
		{LocalPath: "/music/intro.mp3", DestinationFolder: "Music", DestinationName: "intro.mp3"},
		{LocalPath: "/jingles/intro.mp3", DestinationFolder: "Jingles", DestinationName: "intro.mp3"},
	}

	collisions := DetectCollisions(items)
	if len(collisions) != 0 {
		t.Errorf("expected no collisions for different folders, got %d", len(collisions))
	}
}

func TestCollisionCaseInsensitive(t *testing.T) {
	items := []UploadItem{
		{LocalPath: "/a/Intro.mp3", DestinationFolder: "Music", DestinationName: "Intro.mp3"},
		{LocalPath: "/b/intro.MP3", DestinationFolder: "Music", DestinationName: "intro.MP3"},
	}

	collisions := DetectCollisions(items)
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}
}

func TestCollisionNoDuplicateNames(t *testing.T) {
	items := []UploadItem{
		{LocalPath: "/a/track1.mp3", DestinationFolder: "", DestinationName: "track1.mp3"},
		{LocalPath: "/b/track2.mp3", DestinationFolder: "", DestinationName: "track2.mp3"},
	}

	collisions := DetectCollisions(items)
	if len(collisions) != 0 {
		t.Errorf("expected no collisions, got %d", len(collisions))
	}
}

func TestCollisionMediaRoot(t *testing.T) {
	items := []UploadItem{
		{LocalPath: "/a/intro.mp3", DestinationFolder: "", DestinationName: "intro.mp3"},
		{LocalPath: "/b/intro.mp3", DestinationFolder: "", DestinationName: "intro.mp3"},
	}

	collisions := DetectCollisions(items)
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision in media root, got %d", len(collisions))
	}
}

func TestCollisionErrorFormatting(t *testing.T) {
	collisions := []Collision{
		{
			DestinationFolder: "Music",
			DestinationName:   "intro.mp3",
			ConflictingPaths:  []string{"/a/intro.mp3", "/b/intro.mp3"},
		},
	}

	err := CollisionError(collisions)
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
	if !contains(errMsg, "Music/intro.mp3") {
		t.Error("error should mention the colliding folder/name")
	}
	if !contains(errMsg, "--allow-name-collisions") {
		t.Error("error should suggest --allow-name-collisions")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
