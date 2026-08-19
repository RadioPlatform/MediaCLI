package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDirDefault(t *testing.T) {
	os.Unsetenv("XDG_CONFIG_HOME")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, ".config/rpmedia-cli") {
		t.Errorf("unexpected config dir: %s", dir)
	}
}

func TestConfigDirXDG(t *testing.T) {
	os.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	defer os.Unsetenv("XDG_CONFIG_HOME")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/xdg-test/rpmedia-cli" {
		t.Errorf("expected /tmp/xdg-test/rpmedia-cli, got %s", dir)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	cfg := &Config{
		APIKey:             "sk_test_key",
		DefaultStationUUID: "uuid-123",
		DefaultStationName: "Test Station",
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "sk_test_key" {
		t.Errorf("expected sk_test_key, got %s", loaded.APIKey)
	}
	if loaded.DefaultStationUUID != "uuid-123" {
		t.Errorf("expected uuid-123, got %s", loaded.DefaultStationUUID)
	}
}

func TestEnvKeyOverride(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	os.Setenv("RADIO_PLATFORM_CLI_KEY", "env-key")
	defer os.Unsetenv("RADIO_PLATFORM_CLI_KEY")

	cfg := &Config{
		DefaultStationUUID: "uuid-123",
		DefaultStationName: "Test",
	}
	Save(cfg)

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EffectiveAPIKey() != "env-key" {
		t.Errorf("expected effective env-key, got %s", loaded.EffectiveAPIKey())
	}
	if loaded.APIKey != "" {
		t.Errorf("environment key must not be copied into persisted APIKey, got %s", loaded.APIKey)
	}
}

func TestServerURLConfigurationAndEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := Save(&Config{ServerURL: "https://configured.example.com/"}); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.EffectiveServerURL(); got != "https://configured.example.com" {
		t.Fatalf("expected configured URL, got %q", got)
	}
	if got := loaded.ServerSource(); got != "config file" {
		t.Fatalf("expected config file source, got %q", got)
	}

	t.Setenv("RADIO_PLATFORM_CLI_URL", "https://environment.example.com/")
	loaded, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.EffectiveServerURL(); got != "https://environment.example.com" {
		t.Fatalf("expected environment URL, got %q", got)
	}
	if got := loaded.ServerSource(); got != "environment" {
		t.Fatalf("expected environment source, got %q", got)
	}
}

func TestEnvironmentServerURLIsNotPersisted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("RADIO_PLATFORM_CLI_URL", "https://temporary.example.com")

	if err := Save(&Config{ServerURL: "https://stored.example.com"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveWithPreservation(cfg); err != nil {
		t.Fatal(err)
	}

	t.Setenv("RADIO_PLATFORM_CLI_URL", "")
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.EffectiveServerURL(); got != "https://stored.example.com" {
		t.Fatalf("temporary environment URL was persisted: got %q", got)
	}
}

func TestEnvironmentKeyIsNotPersisted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("RADIO_PLATFORM_CLI_KEY", "temporary-ci-key")

	if err := Save(&Config{
		APIKey:             "stored-key",
		DefaultStationUUID: "old-station",
		DefaultStationName: "Old Station",
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultStationUUID = "new-station"
	cfg.DefaultStationName = "New Station"
	if err := SaveWithPreservation(cfg); err != nil {
		t.Fatal(err)
	}

	t.Setenv("RADIO_PLATFORM_CLI_KEY", "")
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.APIKey != "stored-key" {
		t.Fatalf("temporary environment key was persisted: got %q", reloaded.APIKey)
	}
	if reloaded.DefaultStationUUID != "new-station" {
		t.Fatalf("station update was not persisted: got %q", reloaded.DefaultStationUUID)
	}
}

func TestClearCredentials(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	cfg := &Config{
		APIKey:             "key-to-clear",
		DefaultStationUUID: "uuid-to-clear",
		DefaultStationName: "name-to-clear",
	}
	Save(cfg)

	if err := ClearCredentials(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "" {
		t.Error("API key should be cleared")
	}
	if loaded.DefaultStationUUID != "" {
		t.Error("station UUID should be cleared")
	}
	if loaded.DefaultStationName != "" {
		t.Error("station name should be cleared")
	}
}

func TestPreserveUnknownFields(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	cfgPath := filepath.Join(dir, "rpmedia-cli", "config.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0700)

	initial := map[string]interface{}{
		"api_key":        "test-key",
		"future_setting": "should-survive",
		"another_future": 42,
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(cfgPath, data, 0600)

	if err := ClearCredentials(); err != nil {
		t.Fatal(err)
	}

	saved, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	json.Unmarshal(saved, &result)

	if result["future_setting"] != "should-survive" {
		t.Error("future_setting should be preserved")
	}
	if result["another_future"] != float64(42) {
		t.Error("another_future should be preserved")
	}
	if _, ok := result["api_key"]; ok {
		t.Error("api_key should be removed")
	}
}

func TestMaskedKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "***"},
		{"12345678", "********"},
		{"sk_live_abcdefgh12345678", "sk_l****************5678"},
	}
	for _, tt := range tests {
		cfg := &Config{APIKey: tt.input}
		masked := cfg.MaskedKey()
		if masked != tt.expected {
			t.Errorf("MaskedKey(%q) = %q, want %q", tt.input, masked, tt.expected)
		}
	}
}

func TestHasAPIKey(t *testing.T) {
	cfg := &Config{APIKey: "key"}
	if !cfg.HasAPIKey() {
		t.Error("should have API key")
	}
	cfg = &Config{}
	if cfg.HasAPIKey() {
		t.Error("should not have API key")
	}
}

func TestHasDefaultStation(t *testing.T) {
	cfg := &Config{DefaultStationUUID: "u", DefaultStationName: "n"}
	if !cfg.HasDefaultStation() {
		t.Error("should have default station")
	}
	cfg = &Config{}
	if cfg.HasDefaultStation() {
		t.Error("should not have default station")
	}
	cfg = &Config{DefaultStationUUID: "u"}
	if cfg.HasDefaultStation() {
		t.Error("should not have default station without name")
	}
}

func TestCredentialSource(t *testing.T) {
	os.Unsetenv("RADIO_PLATFORM_CLI_KEY")

	cfg := &Config{APIKey: "key"}
	source := cfg.CredentialSource()
	if source != "config file" {
		t.Errorf("expected config file, got %s", source)
	}

	cfg = &Config{}
	source = cfg.CredentialSource()
	if source != "not configured" {
		t.Errorf("expected not configured, got %s", source)
	}
}

func TestPermissions(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	cfg := &Config{APIKey: "test"}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	cfgPath, _ := ConfigPath()
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("config file should not be world-accessible: %o", info.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Dir(cfgPath))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm()&0077 != 0 {
		t.Errorf("config dir should not be world-accessible: %o", dirInfo.Mode().Perm())
	}
}

func TestInvalidJSONConfig(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	cfgPath := filepath.Join(dir, "rpmedia-cli", "config.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0700)
	os.WriteFile(cfgPath, []byte("invalid json"), 0600)

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	cfg := &Config{APIKey: "original", DefaultStationName: "Original Station"}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Read back
	loaded, _ := Load()
	if loaded.APIKey != "original" {
		t.Errorf("expected original key, got %s", loaded.APIKey)
	}

	// Overwrite
	cfg.APIKey = "new-key"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, _ = Load()
	if loaded.APIKey != "new-key" {
		t.Errorf("expected new-key, got %s", loaded.APIKey)
	}

	// Should only have one config file
	cfgPath, _ := ConfigPath()
	dirEntries, _ := os.ReadDir(filepath.Dir(cfgPath))
	jsonFiles := 0
	for _, e := range dirEntries {
		if !e.IsDir() {
			jsonFiles++
		}
	}
	if jsonFiles != 1 {
		t.Errorf("expected 1 config file, found %d", jsonFiles)
	}
}

func TestCheckPermissionsRepair(t *testing.T) {
	dir := t.TempDir()
	origEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", origEnv)

	cfg := &Config{APIKey: "test"}
	Save(cfg)

	cfgPath, _ := ConfigPath()
	os.Chmod(cfgPath, 0644)

	if err := CheckPermissions(); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(cfgPath)
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("permissions should have been repaired: %o", info.Mode().Perm())
	}
}
