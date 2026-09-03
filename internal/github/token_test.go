package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTokenPrefersFlag(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "from-env")
	got := ResolveToken("from-flag", "github.com")
	if got.Token != "from-flag" {
		t.Errorf("token = %q, want from-flag", got.Token)
	}
}

func TestResolveTokenFromEnvironment(t *testing.T) {
	t.Setenv("MKGO_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("GH_TOKEN", "other")
	got := ResolveToken("", "github.com")
	if got.Token != "env-token" {
		t.Errorf("token = %q, want env-token", got.Token)
	}
	if got.Origin != "$GITHUB_TOKEN" {
		t.Errorf("origin = %q", got.Origin)
	}
}

func TestResolveTokenFromHostsFile(t *testing.T) {
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts.yml")
	content := "github.com:\n    user: octocat\n    oauth_token: gho_fromfile\n    git_protocol: https\nghe.example.com:\n    oauth_token: enterprise\n"
	if err := os.WriteFile(hosts, []byte(content), 0o600); err != nil {
		t.Fatalf("write hosts file: %v", err)
	}

	t.Setenv("GH_CONFIG_DIR", dir)
	for _, name := range tokenEnvVars {
		t.Setenv(name, "")
	}

	if got := ResolveToken("", "github.com"); got.Token != "gho_fromfile" {
		t.Errorf("token = %q, want gho_fromfile", got.Token)
	}
	if got := ResolveToken("", "ghe.example.com"); got.Token != "enterprise" {
		t.Errorf("enterprise token = %q", got.Token)
	}
	if got := ResolveToken("", "missing.example.com"); got.Token != "" {
		t.Errorf("expected no token for an unknown host, got %q", got.Token)
	}
}

func TestResolveTokenMissing(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	for _, name := range tokenEnvVars {
		t.Setenv(name, "")
	}
	if got := ResolveToken("", "github.com"); got.Token != "" {
		t.Errorf("expected no token, got %q", got.Token)
	}
}
