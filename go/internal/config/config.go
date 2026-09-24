// Package config loads secret.json, the same config file used by the Python version.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type ParentPath struct {
	Path   string   `json:"path"`
	Ignore []string `json:"ignore"`
}

type MongoDB struct {
	Enabled           bool   `json:"enabled"`
	BackupURI         string `json:"backup-uri"`
	NoteHowToRestore  string `json:"NOTE-how_to_restore"`
}

type Database struct {
	MongoDB MongoDB `json:"mongodb"`
}

type HetznerSFTP struct {
	Enabled   bool   `json:"enabled"`
	ServerURL string `json:"server-url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	RemoteDir string `json:"remote-dir"`
}

type Backups struct {
	SaveLocation    string                `json:"save-location"`
	MaxLocalBackups int                   `json:"max-local-backups"`
	SaveRelative    bool                  `json:"save-relative"`
	ParentPaths     map[string]ParentPath `json:"parent-paths"`
	HetznerSFTP     HetznerSFTP           `json:"hetzner-sftp"`
	Database        Database              `json:"database"`
}

type Config struct {
	DiscordWebhookEnable bool     `json:"discord-webhook-enable"`
	DiscordWebhook       string   `json:"discord-webhook"`
	Backups              *Backups `json:"backups"`

	// path secret.json was loaded from, kept for Save
	path string
}

// Load reads secret.json from path. Same file the Python version expects
// (repo root, next to secret.json.example).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("\nError loading %s: %w\nMAKE SURE YOU 'cp secret.json.example secret.json'\n", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("\nError loading %s: %w\nMAKE SURE YOU 'cp secret.json.example secret.json'\n", path, err)
	}
	cfg.path = path
	return &cfg, nil
}

// Save writes the config back to the file it was loaded from.
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o644)
}
