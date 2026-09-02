package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}
	if cfg.Author != "" || cfg.Private != nil {
		t.Errorf("expected an empty config, got %+v", cfg)
	}
}

func TestLoadReadsValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"author":"Jane Doe","license":"isc","layout":"lib","private":true,"topics":["go","cli"]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}
	if cfg.Author != "Jane Doe" || cfg.License != "isc" || cfg.Layout != "lib" {
		t.Errorf("unexpected config %+v", cfg)
	}
	if !Bool(cfg.Private, false) {
		t.Error("private should be true")
	}
	if len(cfg.Topics) != 2 {
		t.Errorf("topics = %v", cfg.Topics)
	}
	if cfg.Path != path {
		t.Errorf("path = %q, want %q", cfg.Path, path)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestDefaultPathHonorsOverrides(t *testing.T) {
	t.Setenv("MKGO_CONFIG_DIR", "/custom")
	if got := DefaultPath(); got != filepath.Join("/custom", "config.json") {
		t.Errorf("DefaultPath = %q", got)
	}

	t.Setenv("MKGO_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := DefaultPath(); got != filepath.Join("/xdg", "mkgo", "config.json") {
		t.Errorf("DefaultPath = %q", got)
	}
}

func TestBool(t *testing.T) {
	yes := true
	if !Bool(&yes, false) {
		t.Error("Bool should return the pointed value")
	}
	if Bool(nil, true) != true {
		t.Error("Bool should fall back to the default")
	}
}
