package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/x-chunk/mkgo/internal/config"
	"github.com/x-chunk/mkgo/internal/scaffold"
)

func resolveArgs(t *testing.T, cfg *config.Config, args ...string) *plan {
	t.Helper()
	opts := mustParse(t, args...)
	if cfg == nil {
		cfg = &config.Config{}
	}
	p, err := resolve(context.Background(), opts, cfg)
	if err != nil {
		t.Fatalf("resolve(%v) returned an error: %v", args, err)
	}
	return p
}

func TestResolveDefaults(t *testing.T) {
	p := resolveArgs(t, nil, "demo")
	if p.Name != "demo" {
		t.Errorf("name = %q", p.Name)
	}
	if p.Layout != scaffold.LayoutCLI {
		t.Errorf("layout = %q, want cli", p.Layout)
	}
	if p.LicenseID != "mit" {
		t.Errorf("license = %q, want mit", p.LicenseID)
	}
	if p.Branch != "main" {
		t.Errorf("branch = %q", p.Branch)
	}
	if !p.Git || !p.GitHub || !p.Mod || !p.Commit {
		t.Errorf("every default step should be enabled: %+v", p)
	}
}

func TestResolveSkipFlags(t *testing.T) {
	p := resolveArgs(t, nil, "demo", "--no-git", "--no-gh", "--no-mod")
	if p.Git || p.GitHub || p.Mod || p.Commit || p.Push || p.Tidy {
		t.Errorf("steps should all be disabled: %+v", p)
	}
}

func TestResolveDirFromName(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	p := resolveArgs(t, nil, "demo")
	if want := filepath.Join(wd, "demo"); p.Dir != want {
		t.Errorf("dir = %q, want %q", p.Dir, want)
	}
}

func TestResolveDirFlagWins(t *testing.T) {
	dir := t.TempDir()
	p := resolveArgs(t, nil, "demo", "--dir", dir)
	if p.Dir != dir {
		t.Errorf("dir = %q, want %q", p.Dir, dir)
	}
	if p.Name != "demo" {
		t.Errorf("name = %q", p.Name)
	}
}

func TestResolvePathAsName(t *testing.T) {
	p := resolveArgs(t, nil, "projects/demo")
	if p.Name != "demo" {
		t.Errorf("name = %q, want demo", p.Name)
	}
	if !strings.HasSuffix(p.Dir, filepath.Join("projects", "demo")) {
		t.Errorf("dir = %q", p.Dir)
	}
}

func TestResolveDotUsesCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	p := resolveArgs(t, nil, ".")
	if p.Name != filepath.Base(dir) {
		t.Errorf("name = %q, want %q", p.Name, filepath.Base(dir))
	}
}

func TestResolveRejectsInvalidName(t *testing.T) {
	opts := mustParse(t, "not a name")
	if _, err := resolve(context.Background(), opts, &config.Config{}); err == nil {
		t.Fatal("expected an error for an invalid project name")
	}
}

func TestResolveConfigDefaults(t *testing.T) {
	private := true
	cfg := &config.Config{
		Author:       "Config Author",
		License:      "isc",
		Layout:       "lib",
		Branch:       "trunk",
		ModulePrefix: "example.com/team",
		Private:      &private,
		Topics:       []string{"go", "tools"},
	}
	p := resolveArgs(t, cfg, "demo")
	if p.Author != "Config Author" {
		t.Errorf("author = %q", p.Author)
	}
	if p.LicenseID != "isc" || p.Layout != scaffold.LayoutLib || p.Branch != "trunk" {
		t.Errorf("config defaults were not applied: %+v", p)
	}
	if !p.Private {
		t.Error("private should come from the config")
	}
	if len(p.Topics) != 2 {
		t.Errorf("topics = %v", p.Topics)
	}

	p.resolveModule(mustParse(t, "demo"), cfg)
	if p.Module != "example.com/team/demo" {
		t.Errorf("module = %q", p.Module)
	}
}

func TestFlagsBeatConfig(t *testing.T) {
	cfg := &config.Config{License: "isc", Layout: "lib", Branch: "trunk"}
	p := resolveArgs(t, cfg, "demo", "--license", "mit", "--layout", "api", "--branch", "main")
	if p.LicenseID != "mit" || p.Layout != scaffold.LayoutAPI || p.Branch != "main" {
		t.Errorf("flags did not win over the config: %+v", p)
	}
}

func TestResolveModuleFromOwner(t *testing.T) {
	p := resolveArgs(t, nil, "demo")
	p.Owner = "octocat"
	p.resolveModule(mustParse(t, "demo"), &config.Config{})
	if p.Module != "github.com/octocat/demo" {
		t.Errorf("module = %q", p.Module)
	}
}

func TestResolveModuleFallsBackToName(t *testing.T) {
	p := resolveArgs(t, nil, "demo")
	p.resolveModule(mustParse(t, "demo"), &config.Config{})
	if p.Module != "demo" {
		t.Errorf("module = %q, want demo", p.Module)
	}
}

func TestRemoteAndWebURLs(t *testing.T) {
	p := resolveArgs(t, nil, "demo")
	p.Owner = "octocat"
	if got := p.RemoteURL(); got != "https://github.com/octocat/demo.git" {
		t.Errorf("https remote = %q", got)
	}
	p.SSH = true
	if got := p.RemoteURL(); got != "git@github.com:octocat/demo.git" {
		t.Errorf("ssh remote = %q", got)
	}
	p.Org = "acme"
	if got := p.Slug(); got != "acme/demo" {
		t.Errorf("slug = %q, want acme/demo", got)
	}
}

func TestEnterpriseHost(t *testing.T) {
	p := resolveArgs(t, nil, "demo", "--host", "https://ghe.example.com/")
	if p.Host != "ghe.example.com" {
		t.Errorf("host = %q", p.Host)
	}
	if p.APIBase != "https://ghe.example.com/api/v3" {
		t.Errorf("api base = %q", p.APIBase)
	}
}

func TestSplitTopics(t *testing.T) {
	got := splitTopics(" Go , cli ,, Tools ")
	want := []string{"go", "cli", "tools"}
	if len(got) != len(want) {
		t.Fatalf("topics = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("topics[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if splitTopics("  ") != nil {
		t.Error("expected nil for an empty topic list")
	}
}
