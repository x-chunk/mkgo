// Package config loads the optional mkgo configuration file so users do not
// have to repeat the same flags on every run.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config mirrors the JSON file at $XDG_CONFIG_HOME/mkgo/config.json. Pointer
// fields distinguish "not set" from "set to false".
type Config struct {
	Author       string   `json:"author,omitempty"`
	License      string   `json:"license,omitempty"`
	Layout       string   `json:"layout,omitempty"`
	Branch       string   `json:"branch,omitempty"`
	GoVersion    string   `json:"go_version,omitempty"`
	Org          string   `json:"org,omitempty"`
	Host         string   `json:"github_host,omitempty"`
	ModulePrefix string   `json:"module_prefix,omitempty"`
	Topics       []string `json:"topics,omitempty"`
	Private      *bool    `json:"private,omitempty"`
	SSH          *bool    `json:"ssh,omitempty"`
	Color        *bool    `json:"color,omitempty"`
	Emoji        *bool    `json:"emoji,omitempty"`

	// Path records where the config was read from; it is not serialized.
	Path string `json:"-"`
}

// DefaultPath returns the location mkgo reads its configuration from.
func DefaultPath() string {
	if dir := os.Getenv("MKGO_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "mkgo", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mkgo", "config.json")
}

// Load reads the configuration file. A missing file is not an error: it
// returns an empty config so every default still applies.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	if path == "" {
		return &Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return &Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return &Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.Path = path
	return cfg, nil
}

// Bool resolves an optional boolean against a default value.
func Bool(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}
