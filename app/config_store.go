package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// defaultMemoryLimitGB is the Go runtime soft memory limit (in GB) used when
// the user has not set an explicit override.
const defaultMemoryLimitGB = 8

// appConfig holds all application settings persisted to disk.
// Frontend-owned sections use json.RawMessage so Go round-trips them without
// needing to duplicate the full type definitions — the frontend owns the schema.
type appConfig struct {
	MemoryLimitGB   int64             `json:"memoryLimitGB"`             // 0 = default (8 GB)
	InstalledLibs   map[string]string `json:"installedLibs,omitempty"`   // libID → local dir overrides
	RecentFiles     []string          `json:"recentFiles,omitempty"`     // most-recently-opened file paths
	SavedTabs       json.RawMessage   `json:"savedTabs,omitempty"`       // frontend-owned tab state
	ActiveTab       string            `json:"activeTab,omitempty"`       // frontend-owned active tab path
	Appearance      json.RawMessage   `json:"appearance,omitempty"`      // frontend-owned
	Editor          json.RawMessage   `json:"editor,omitempty"`          // frontend-owned
	Assistant       json.RawMessage   `json:"assistant,omitempty"`      // frontend-owned
	Camera          json.RawMessage   `json:"camera,omitempty"`          // frontend-owned
	Slicer          json.RawMessage   `json:"slicer,omitempty"`          // frontend-owned
	LibrarySettings json.RawMessage   `json:"librarySettings,omitempty"` // frontend-owned (autoPull, etc.)
}

// configDir returns the OS-specific Facet config directory.
func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "Facet")
}

// configPath returns the path to the backend settings file.
func configPath() string {
	return filepath.Join(configDir(), "settings.json")
}

// loadConfig reads the backend settings file. A missing file is not an error —
// it just returns an empty config (first launch). Any other read failure, or a
// JSON unmarshal failure, is propagated so callers do not silently overwrite
// the user's real settings with defaults on the next save.
//
// Stateless: safe for concurrent read-only use. Callers performing a
// load-mutate-save sequence must serialize through ConfigStore.Mutate.
func loadConfig() (appConfig, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return appConfig{}, nil
		}
		return appConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return appConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// saveConfig writes cfg to disk. Not mutex-serialized; callers performing a
// load-mutate-save sequence must hold ConfigStore.mu (use ConfigStore.Mutate).
func saveConfig(cfg appConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("saveConfig: mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("saveConfig: marshal: %w", err)
	}
	path := configPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("saveConfig: write %s: %w", path, err)
	}
	return nil
}

// ConfigStore owns the write mutex for the on-disk application settings file.
// Mutate and Patch serialize writes so concurrent load-modify-save sequences
// cannot lose updates. Read-only callers can use the package-level loadConfig
// helper directly — reads do not participate in the mutex.
type ConfigStore struct {
	mu sync.Mutex
}

// NewConfigStore creates a new config store. The config file is not created
// eagerly; a missing file is treated as an empty config and is only written
// on the first mutation.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{}
}

// Mutate serializes access to the config: it acquires the mutex, loads the
// current config, passes it (by pointer) to fn, then saves the result. If
// Load fails the load error is returned and fn is not called, so a corrupt
// settings file is never silently overwritten. If fn returns an error, the
// config is not saved.
func (c *ConfigStore) Mutate(fn func(*appConfig) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := fn(&cfg); err != nil {
		return err
	}
	return saveConfig(cfg)
}

// GetJSON returns the current config marshaled as a JSON string for the
// frontend. Propagates errors from loadConfig so the frontend can warn the
// user rather than silently overwriting the file with defaults on the next
// save.
func (c *ConfigStore) GetJSON() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal settings: %w", err)
	}
	return string(data), nil
}

// Patch merges the provided partial JSON into the existing config.
// Only keys present in the patch are updated; missing keys are preserved.
// This is the primary way both frontend and Go code should update settings.
//
// If the existing settings file exists but cannot be parsed, Patch refuses to
// save — overwriting it would silently wipe the user's real settings. A
// missing file, by contrast, is treated as an empty base.
func (c *ConfigStore) Patch(jsonStr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := configPath()
	base := make(map[string]json.RawMessage)
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(existing, &base); err != nil {
			return fmt.Errorf("parse %s: %w (refusing to overwrite)", path, err)
		}
	case os.IsNotExist(err):
		// No existing file — first write.
	default:
		return fmt.Errorf("read %s: %w", path, err)
	}

	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &patch); err != nil {
		return fmt.Errorf("PatchSettings: bad patch JSON: %w", err)
	}

	// Merge: patch keys override base keys.
	for k, v := range patch {
		base[k] = v
	}

	// Round-trip through appConfig to drop unknown keys and validate types.
	merged, err := json.Marshal(base)
	if err != nil {
		return fmt.Errorf("PatchSettings: marshal merged: %w", err)
	}
	var cfg appConfig
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return fmt.Errorf("PatchSettings: validate merged: %w", err)
	}
	return saveConfig(cfg)
}
