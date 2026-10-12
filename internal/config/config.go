// Package config holds the per-user settings file (%LOCALAPPDATA%\neoHub\config.json).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config is everything the app can be told. Fields match the Node version's config.json.
type Config struct {
	HubHost            string `json:"hubHost"`
	HubPort            int    `json:"hubPort"`
	ListenHost         string `json:"listenHost"`
	ListenPort         int    `json:"listenPort"`
	StatusCacheMs      int    `json:"statusCacheMs"`
	HubTimeoutMs       int    `json:"hubTimeoutMs"`
	HistoryPollMs      int    `json:"historyPollMs"`
	HistoryHeartbeatMs int    `json:"historyHeartbeatMs"`

	mu   sync.Mutex
	path string
}

// Dir is the per-user data directory, created on first use.
func Dir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	d := filepath.Join(base, "neoHub")
	_ = os.MkdirAll(d, 0o755)
	return d
}

// Load reads the settings file, filling defaults for anything missing.
func Load() *Config {
	c := &Config{HubPort: 4242, ListenHost: "0.0.0.0", ListenPort: 8484, StatusCacheMs: 4000, HubTimeoutMs: 15000, HistoryPollMs: 60000, HistoryHeartbeatMs: 600000}
	c.path = filepath.Join(Dir(), "config.json")
	if b, err := os.ReadFile(c.path); err == nil {
		_ = json.Unmarshal(b, c)
	}
	if c.HubPort == 0 {
		c.HubPort = 4242
	}
	if c.ListenPort == 0 {
		c.ListenPort = 8484
	}
	if c.ListenHost == "" {
		c.ListenHost = "0.0.0.0"
	}
	return c
}

// Save writes the settings file.
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, append(b, '\n'), 0o644)
}

// Path returns where the settings live.
func (c *Config) Path() string { return c.path }
