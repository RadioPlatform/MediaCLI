package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repository       = "RadioPlatform/MediaCLI"
	latestReleaseURL = "https://api.github.com/repos/" + repository + "/releases/latest"
	binaryName       = "media-cli"
	checksumFileName = "checksums.txt"
)

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName    string  `json:"tag_name"`
	HTMLURL    string  `json:"html_url"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

type Client struct {
	HTTPClient *http.Client
	LatestURL  string
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		LatestURL:  latestReleaseURL,
	}
}

func (c *Client) Latest(ctx context.Context) (*Release, error) {
	if c == nil || c.HTTPClient == nil {
		return nil, errors.New("updater HTTP client is not configured")
	}

	url := c.LatestURL
	if url == "" {
		url = latestReleaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create latest-release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "media-cli-updater")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check for updates: GitHub returned %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	if release.TagName == "" || release.Draft || release.Prerelease {
		return nil, errors.New("GitHub did not return a stable release")
	}
	return &release, nil
}

func (r Release) AssetForPlatform(goos, goarch string) (Asset, error) {
	platformName := ""
	switch goos {
	case "darwin":
		platformName = "Darwin"
	case "linux":
		platformName = "Linux"
	default:
		return Asset{}, fmt.Errorf("automatic updates are not supported on %s", goos)
	}

	name := fmt.Sprintf("%s_%s_%s.tar.gz", binaryName, platformName, goarch)
	for _, asset := range r.Assets {
		if asset.Name == name && asset.BrowserDownloadURL != "" {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s does not include %s", r.TagName, name)
}

func (r Release) ChecksumAsset() (Asset, error) {
	for _, asset := range r.Assets {
		if asset.Name == checksumFileName && asset.BrowserDownloadURL != "" {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s does not include %s", r.TagName, checksumFileName)
}

func (c *Client) Install(ctx context.Context, release Release, executable string) error {
	asset, err := release.AssetForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	checksums, err := release.ChecksumAsset()
	if err != nil {
		return err
	}

	expectedChecksum, err := c.downloadChecksum(ctx, checksums.BrowserDownloadURL, asset.Name)
	if err != nil {
		return err
	}

	archive, err := c.download(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer os.Remove(archive.Name())
	defer archive.Close()

	checksum := sha256.New()
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind downloaded archive: %w", err)
	}
	if _, err := io.Copy(checksum, archive); err != nil {
		return fmt.Errorf("verify downloaded archive: %w", err)
	}
	if actual := hex.EncodeToString(checksum.Sum(nil)); !strings.EqualFold(actual, expectedChecksum) {
		return errors.New("downloaded archive does not match the release checksum")
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind verified archive: %w", err)
	}

	return replaceExecutable(archive, executable)
}

func (c *Client) downloadChecksum(ctx context.Context, url, assetName string) (string, error) {
	file, err := c.download(ctx, url)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind checksums: %w", err)
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.TrimPrefix(fields[1], "*") != assetName {
			continue
		}
		if len(fields[0]) != sha256.Size*2 {
			return "", fmt.Errorf("invalid checksum for %s", assetName)
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return "", fmt.Errorf("invalid checksum for %s", assetName)
		}
		return fields[0], nil
	}
	return "", fmt.Errorf("checksums do not contain %s", assetName)
}

func (c *Client) download(ctx context.Context, url string) (*os.File, error) {
	if c == nil || c.HTTPClient == nil {
		return nil, errors.New("updater HTTP client is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "media-cli-updater")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned %s", resp.Status)
	}

	file, err := os.CreateTemp("", "media-cli-update-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("create temporary download: %w", err)
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, fmt.Errorf("save download: %w", err)
	}
	return file, nil
}

func replaceExecutable(archive io.Reader, executable string) error {
	if executable == "" {
		return errors.New("current executable path is empty")
	}
	resolvedPath, err := filepath.EvalSymlinks(executable)
	if err == nil {
		executable = resolvedPath
	}

	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect current executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("current executable is not a regular file: %s", executable)
	}

	replacement, err := os.CreateTemp(filepath.Dir(executable), ".media-cli-update-*")
	if err != nil {
		return fmt.Errorf("create replacement beside current executable: %w", err)
	}
	replacementName := replacement.Name()
	defer os.Remove(replacementName)

	if err := extractBinary(archive, replacement); err != nil {
		replacement.Close()
		return err
	}
	mode := info.Mode().Perm()
	if mode&0111 == 0 {
		mode = 0755
	}
	if err := replacement.Chmod(mode); err != nil {
		replacement.Close()
		return fmt.Errorf("set replacement permissions: %w", err)
	}
	if err := replacement.Sync(); err != nil {
		replacement.Close()
		return fmt.Errorf("sync replacement: %w", err)
	}
	if err := replacement.Close(); err != nil {
		return fmt.Errorf("close replacement: %w", err)
	}
	if err := os.Rename(replacementName, executable); err != nil {
		return fmt.Errorf("replace current executable: %w", err)
	}
	return nil
}

func extractBinary(archive io.Reader, destination io.Writer) error {
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open release archive: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read release archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		if _, err := io.Copy(destination, tarReader); err != nil {
			return fmt.Errorf("extract %s: %w", binaryName, err)
		}
		return nil
	}
	return fmt.Errorf("release archive does not contain %s", binaryName)
}

func CompareVersions(current, latest string) int {
	currentParts, currentOK := parseVersion(current)
	latestParts, latestOK := parseVersion(latest)
	switch {
	case !currentOK && !latestOK:
		return 0
	case !currentOK:
		return -1
	case !latestOK:
		return 1
	}
	for i := range currentParts {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(version string) ([3]int, bool) {
	var parts [3]int
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	version = strings.SplitN(version, "-", 2)[0]
	segments := strings.Split(version, ".")
	if len(segments) != len(parts) {
		return parts, false
	}
	for i, segment := range segments {
		if segment == "" {
			return parts, false
		}
		value, err := strconv.Atoi(segment)
		if err != nil || value < 0 {
			return parts, false
		}
		parts[i] = value
	}
	return parts, true
}
