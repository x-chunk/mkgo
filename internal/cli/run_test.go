package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI executes a full mkgo run with isolated streams.
func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(context.Background(), args, Environment{
		Stdout:  &out,
		Stderr:  &errOut,
		Stdin:   strings.NewReader(""),
		Version: "test",
	})
	return code, out.String(), errOut.String()
}

func TestRunCreatesProjectWithoutGitHub(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	code, stdout, stderr := runCLI(t, "demo", "--dir", dir,
		"--no-gh", "--no-git", "--no-tidy", "--no-color", "--author", "Jane Doe")
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	for _, name := range []string{"main.go", "go.mod", "README.md", "LICENSE", "Makefile", ".editorconfig", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Error("--no-git must not initialize a repository")
	}
	if !strings.Contains(stdout, "is ready") {
		t.Errorf("summary missing from output:\n%s", stdout)
	}
}

func TestRunDryRunTouchesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	code, stdout, _ := runCLI(t, "demo", "--dir", dir, "--no-gh", "--dry-run", "--no-color")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dry run created %s", dir)
	}
	if !strings.Contains(stdout, "dry run") {
		t.Errorf("output does not mention the dry run:\n%s", stdout)
	}
}

func TestRunNoModSkipsGoMod(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	code, _, stderr := runCLI(t, "demo", "--dir", dir, "--no-gh", "--no-git", "--no-mod", "--no-color")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); !os.IsNotExist(err) {
		t.Error("go.mod was created despite --no-mod")
	}
}

func TestRunNoEmojiUsesASCII(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	_, stdout, _ := runCLI(t, "demo", "--dir", dir, "--no-gh", "--no-git",
		"--no-tidy", "--no-color", "--no-emoji")
	if strings.ContainsRune(stdout, '🚀') || strings.ContainsRune(stdout, '✅') {
		t.Errorf("emoji leaked into --no-emoji output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "OK ") {
		t.Errorf("expected ASCII status markers:\n%s", stdout)
	}
}

func TestRunNoColorHasNoEscapes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	_, stdout, _ := runCLI(t, "demo", "--dir", dir, "--no-gh", "--no-git", "--no-tidy", "--no-color")
	if strings.ContainsRune(stdout, '\033') {
		t.Errorf("ANSI escape found in --no-color output:\n%q", stdout)
	}
}

func TestRunQuietPrintsNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	code, stdout, _ := runCLI(t, "demo", "--dir", dir, "--no-gh", "--no-git", "--no-tidy", "--quiet")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("quiet run wrote to stdout:\n%s", stdout)
	}
}

func TestRunRefusesNonEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	code, _, stderr := runCLI(t, "demo", "--dir", dir, "--no-gh", "--no-git", "--no-color")
	if code == 0 {
		t.Fatal("expected a non-zero exit code for a non-empty directory")
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("error should suggest --force:\n%s", stderr)
	}
}

func TestRunForceWritesIntoNonEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())

	code, _, stderr := runCLI(t, "demo", "--dir", dir, "--no-gh", "--no-git",
		"--no-tidy", "--no-color", "--force")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Error("--force must not remove existing files")
	}
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Errorf("main.go was not created: %v", err)
	}
}

func TestRunUsageErrorExitCode(t *testing.T) {
	code, _, stderr := runCLI(t, "--nope")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "unknown flag") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunWithoutTokenSkipsGitHub(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("MKGO_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	code, _, stderr := runCLI(t, "demo", "--dir", dir, "--no-git", "--no-tidy", "--no-color")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "no GitHub token") {
		t.Errorf("expected a warning about the missing token:\n%s", stderr)
	}
}

func TestRunRequiresTokenWhenGitHubFlagsAreUsed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("MKGO_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	code, _, stderr := runCLI(t, "demo", "--dir", dir, "--private", "--no-git", "--no-color")
	if code == 0 {
		t.Fatal("expected a failure when --private is used without a token")
	}
	if !strings.Contains(stderr, "token") {
		t.Errorf("stderr = %q", stderr)
	}
}
