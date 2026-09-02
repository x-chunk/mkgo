package github

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// TokenSource records where a credential came from so the CLI can tell the
// user which one it picked.
type TokenSource struct {
	Token  string
	Origin string
}

// tokenEnvVars are checked in order; the first non-empty one wins.
var tokenEnvVars = []string{"MKGO_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"}

// ResolveToken finds a GitHub credential without invoking any external tool.
// Precedence: explicit flag, environment variables, then the token stored by
// the GitHub CLI in its hosts file (read as a plain file, never executed).
func ResolveToken(explicit, host string) TokenSource {
	if explicit != "" {
		return TokenSource{Token: explicit, Origin: "--token flag"}
	}
	for _, name := range tokenEnvVars {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return TokenSource{Token: v, Origin: "$" + name}
		}
	}
	if token, path := tokenFromHostsFile(host); token != "" {
		return TokenSource{Token: token, Origin: path}
	}
	return TokenSource{}
}

// hostsFilePath returns the location of the GitHub CLI hosts file, honoring
// GH_CONFIG_DIR and XDG_CONFIG_HOME.
func hostsFilePath() string {
	if dir := os.Getenv("GH_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "hosts.yml")
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "gh", "hosts.yml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gh", "hosts.yml")
}

// tokenFromHostsFile extracts "oauth_token" for the requested host from the
// GitHub CLI hosts file. The format is a shallow YAML map, so a small scanner
// is enough and keeps mkgo dependency free.
func tokenFromHostsFile(host string) (token, path string) {
	path = hostsFilePath()
	if path == "" {
		return "", ""
	}
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	if host == "" {
		host = "github.com"
	}

	var currentHost string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			currentHost = strings.TrimSuffix(trimmed, ":")
			continue
		}
		if currentHost != host {
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(key) != "oauth_token" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if value != "" {
			return value, path
		}
	}
	return "", ""
}
