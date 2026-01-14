package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds application configuration
type Config struct {
	// Data directory
	DataDir string `json:"data_dir"`

	// Sync settings
	SyncIntervalMinutes int  `json:"sync_interval_minutes"`
	SyncOnStartup       bool `json:"sync_on_startup"`

	// UI settings
	Theme           string `json:"theme"` // "light", "dark", "system"
	DefaultView     string `json:"default_view"` // "month", "week", "day"
	WeekStartsOn    int    `json:"week_starts_on"` // 0=Sunday, 1=Monday
	ShowWeekNumbers bool   `json:"show_week_numbers"`

	// Notification settings
	NotificationsEnabled bool `json:"notifications_enabled"`
	DefaultReminderMins  int  `json:"default_reminder_mins"`

	// Window state
	WindowWidth  int `json:"window_width"`
	WindowHeight int `json:"window_height"`
	WindowX      int `json:"window_x"`
	WindowY      int `json:"window_y"`
	Maximized    bool `json:"maximized"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		DataDir:              getDefaultDataDir(),
		SyncIntervalMinutes:  15,
		SyncOnStartup:        true,
		Theme:                "system",
		DefaultView:          "month",
		WeekStartsOn:         1, // Monday
		ShowWeekNumbers:      false,
		NotificationsEnabled: true,
		DefaultReminderMins:  30,
		WindowWidth:          900,
		WindowHeight:         600,
	}
}

// Load loads config from disk
func Load() (*Config, error) {
	configPath := getConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default config
		cfg := DefaultConfig()
		// Save default config
		cfg.Save()
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save saves config to disk
func (c *Config) Save() error {
	configPath := getConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// DatabasePath returns the path to the SQLite database
func (c *Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "switchcal.db")
}

// getConfigPath returns the path to the config file
func getConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, "switchcal", "config.json")
}

// getDefaultDataDir returns the default data directory
func getDefaultDataDir() string {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(dataDir, "switchcal")
}
