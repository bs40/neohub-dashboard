// Package config holds the per-user settings file (%LOCALAPPDATA%\neoHub\config.json).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

// Dir is the data directory, created on first use. NEOHUB_DATA overrides it (the systemd
// unit sets it to /var/lib/neohub); otherwise it is %LOCALAPPDATA%\neoHub on Windows and
// ~/.config/neohub elsewhere.
func Dir() string {
	if d := os.Getenv("NEOHUB_DATA"); d != "" {
		_ = os.MkdirAll(d, 0o755)
		return d
	}
	var d string
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base, _ = os.UserConfigDir()
		}
		d = filepath.Join(base, "neoHub")
	} else {
		base, err := os.UserConfigDir()
		if err != nil || base == "" {
			base = "/var/lib"
		}
		d = filepath.Join(base, "neohub")
	}
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
