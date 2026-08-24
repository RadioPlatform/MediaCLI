package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Fatalf("Accept = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "media-cli-updater" {
			t.Fatalf("User-Agent = %q", got)
		}
		_, _ = io.WriteString(w, `{"tag_name":"v1.2.3","html_url":"https://example.test/release","assets":[]}`)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), LatestURL: server.URL}
	release, err := client.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if release.TagName != "v1.2.3" || release.HTMLURL != "https://example.test/release" {
		t.Fatalf("unexpected release: %+v", release)
	}
}

func TestAssetForPlatform(t *testing.T) {
	release := Release{TagName: "v1.2.3", Assets: []Asset{
		{Name: "media-cli_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.test/darwin"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.test/checksums"},
	}}

	asset, err := release.AssetForPlatform("darwin", "arm64")
	if err != nil {
		t.Fatalf("AssetForPlatform() error = %v", err)
	}
	if asset.Name != "media-cli_Darwin_arm64.tar.gz" {
		t.Fatalf("unexpected asset: %+v", asset)
	}
	if _, err := release.ChecksumAsset(); err != nil {
		t.Fatalf("ChecksumAsset() error = %v", err)
	}
	if _, err := release.AssetForPlatform("windows", "amd64"); err == nil {
		t.Fatal("AssetForPlatform() succeeded for unsupported platform")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.3", "v1.2.4", -1},
		{"1.10.0", "v1.2.9", 1},
		{"dev", "v1.0.0", -1},
		{"v1.0.0", "invalid", 1},
	}
	for _, test := range tests {
		if got := CompareVersions(test.current, test.latest); got != test.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.current, test.latest, got, test.want)
		}
	}
}

func TestExtractBinary(t *testing.T) {
	archive := testArchive(t, map[string]string{
		"README.md": "notes",
		"media-cli": "new binary",
	})
	var binary bytes.Buffer
	if err := extractBinary(bytes.NewReader(archive), &binary); err != nil {
		t.Fatalf("extractBinary() error = %v", err)
	}
	if got := binary.String(); got != "new binary" {
		t.Fatalf("extracted binary = %q", got)
	}
}

func TestReplaceExecutable(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, binaryName)
	if err := os.WriteFile(executable, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	archive := testArchive(t, map[string]string{binaryName: "new binary"})
	if err := replaceExecutable(bytes.NewReader(archive), executable); err != nil {
		t.Fatalf("replaceExecutable() error = %v", err)
	}
	contents, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "new binary" {
		t.Fatalf("updated executable = %q", got)
	}
	info, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("updated executable permissions = %#o", info.Mode().Perm())
	}
}

func TestInstall(t *testing.T) {
	archive := testArchive(t, map[string]string{binaryName: "updated binary"})
	checksum := fmt.Sprintf("%x", sha256.Sum256(archive))
	assetName := platformArchiveName(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/archive":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = fmt.Fprintf(w, "%s  %s\n", checksum, assetName)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	directory := t.TempDir()
	executable := filepath.Join(directory, binaryName)
	if err := os.WriteFile(executable, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	client := &Client{HTTPClient: server.Client()}
	release := Release{TagName: "v1.2.3", Assets: []Asset{
		{Name: assetName, BrowserDownloadURL: server.URL + "/archive"},
		{Name: checksumFileName, BrowserDownloadURL: server.URL + "/checksums"},
	}}
	if err := client.Install(context.Background(), release, executable); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	contents, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "updated binary" {
		t.Fatalf("updated executable = %q", got)
	}
}

func platformArchiveName(t *testing.T) string {
	t.Helper()
	platformName := map[string]string{"darwin": "Darwin", "linux": "Linux"}[runtime.GOOS]
	if platformName == "" {
		t.Skipf("unsupported platform for updater test: %s", runtime.GOOS)
	}
	return fmt.Sprintf("%s_%s_%s.tar.gz", binaryName, platformName, runtime.GOARCH)
}

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, contents := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(contents))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tarWriter, contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
