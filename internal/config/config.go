package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	APIKey             string `json:"api_key,omitempty"`
	ServerURL          string `json:"server_url,omitempty"`
	DefaultStationUUID string `json:"default_station_uuid,omitempty"`
	DefaultStationName string `json:"default_station_name,omitempty"`
	runtimeAPIKey      string
	runtimeServerURL   string
}

func ConfigDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "rpmedia-cli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "rpmedia-cli"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func apiKeyFromEnv() string {
	return os.Getenv("RADIO_PLATFORM_CLI_KEY")
}

func serverURLFromEnv() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("RADIO_PLATFORM_CLI_URL")), "/")
}

func Load() (*Config, error) {
	cfg := &Config{
		runtimeAPIKey:    apiKeyFromEnv(),
		runtimeServerURL: serverURLFromEnv(),
	}

	path, err := ConfigPath()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed to read config: %w", err)
	}

	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.APIKey = fileCfg.APIKey
	cfg.ServerURL = strings.TrimRight(strings.TrimSpace(fileCfg.ServerURL), "/")
	if cfg.DefaultStationUUID == "" {
		cfg.DefaultStationUUID = fileCfg.DefaultStationUUID
	}
	if cfg.DefaultStationName == "" {
		cfg.DefaultStationName = fileCfg.DefaultStationName
	}

	return cfg, nil
}

func (c *Config) HasAPIKey() bool {
	return c.EffectiveAPIKey() != ""
}

func (c *Config) HasServerURL() bool {
	return c.EffectiveServerURL() != ""
}

// EffectiveServerURL returns the environment override when present without
// copying it into the configuration that is persisted to disk.
func (c *Config) EffectiveServerURL() string {
	if c.runtimeServerURL != "" {
		return c.runtimeServerURL
	}
	return strings.TrimRight(strings.TrimSpace(c.ServerURL), "/")
}

func (c *Config) ServerSource() string {
	if c.runtimeServerURL != "" {
		return "environment"
	}
	if c.ServerURL != "" {
		return "config file"
	}
	return "not configured"
}

// EffectiveAPIKey returns the environment override when present without
// copying it into the configuration that is persisted to disk.
func (c *Config) EffectiveAPIKey() string {
	if c.runtimeAPIKey != "" {
		return c.runtimeAPIKey
	}
	return c.APIKey
}

func (c *Config) HasDefaultStation() bool {
	return c.DefaultStationUUID != "" && c.DefaultStationName != ""
}

func (c *Config) CredentialSource() string {
	if c.runtimeAPIKey != "" {
		return "environment"
	}
	if c.APIKey != "" {
		return "config file"
	}
	return "not configured"
}

func (c *Config) MaskedKey() string {
	key := c.EffectiveAPIKey()
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := filepath.Join(dir, "config.json")
	return atomicWriteFile(path, data, 0600)
}

func SaveWithPreservation(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path := filepath.Join(dir, "config.json")

	existing := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}

	newData, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var newMap map[string]json.RawMessage
	if err := json.Unmarshal(newData, &newMap); err != nil {
		return fmt.Errorf("failed to unmarshal new config: %w", err)
	}

	for k, v := range newMap {
		existing[k] = v
	}

	merged, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged config: %w", err)
	}

	return atomicWriteFile(path, merged, 0600)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "config.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func CheckPermissions() error {
	path, err := ConfigPath()
	if err != nil {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if runtime.GOOS != "windows" {
		mode := info.Mode().Perm()
		if mode&0077 != 0 {
			newMode := mode &^ 0077
			if err := os.Chmod(path, newMode); err != nil {
				return fmt.Errorf("failed to fix config file permissions: %w", err)
			}
		}
	}

	return nil
}

func ClearCredentials() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "config.json")

	existing := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}

	delete(existing, "api_key")
	delete(existing, "default_station_uuid")
	delete(existing, "default_station_name")

	if len(existing) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	merged, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return atomicWriteFile(path, merged, 0600)
}
