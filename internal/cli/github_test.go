package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/x-chunk/mkgo/internal/gitutil"
)

// stubGitHub serves the handful of endpoints a run touches and records the
// repository payload it received.
func stubGitHub(t *testing.T, exists bool) (*httptest.Server, *map[string]any) {
	t.Helper()
	created := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user":
			json.NewEncoder(w).Encode(map[string]string{"login": "octocat", "name": "The Octocat"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/"):
			if exists {
				w.Write([]byte("{}"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/user/repos":
			json.NewDecoder(r.Body).Decode(&created)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"name":      "demo",
				"full_name": "octocat/demo",
				"private":   created["private"],
				"html_url":  "https://github.com/octocat/demo",
				"clone_url": "https://github.com/octocat/demo.git",
				"ssh_url":   "git@github.com:octocat/demo.git",
				"owner":     map[string]string{"login": "octocat"},
			})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &created
}

// useStubGitHub points the run at the stub server with a fake token.
func useStubGitHub(t *testing.T, srv *httptest.Server) {
	t.Helper()
	t.Setenv("MKGO_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("MKGO_GITHUB_TOKEN", "stub-token")
}

func TestRunCreatesGitHubRepository(t *testing.T) {
	srv, payload := stubGitHub(t, false)
	useStubGitHub(t, srv)

	dir := filepath.Join(t.TempDir(), "demo")
	code, stdout, stderr := runCLI(t, "demo", "--dir", dir, "--no-git", "--no-tidy",
		"--no-color", "--yes", "--private", "--desc", "A demo project.")
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if (*payload)["name"] != "demo" {
		t.Errorf("repository payload = %v", *payload)
	}
	if (*payload)["private"] != true {
		t.Error("--private was not forwarded to the API")
	}
	if (*payload)["description"] != "A demo project." {
		t.Errorf("description = %v", (*payload)["description"])
	}
	if !strings.Contains(stdout, "octocat/demo") {
		t.Errorf("summary is missing the repository:\n%s", stdout)
	}
}

func TestRunUsesOwnerForTheModulePath(t *testing.T) {
	srv, _ := stubGitHub(t, false)
	useStubGitHub(t, srv)

	dir := filepath.Join(t.TempDir(), "demo")
	if code, stdout, stderr := runCLI(t, "demo", "--dir", dir, "--no-git", "--no-tidy",
		"--no-color", "--yes"); code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(data), "module github.com/octocat/demo") {
		t.Errorf("go.mod = %q", string(data))
	}
}

func TestRunStopsWhenRepositoryExists(t *testing.T) {
	srv, _ := stubGitHub(t, true)
	useStubGitHub(t, srv)

	dir := filepath.Join(t.TempDir(), "demo")
	code, _, stderr := runCLI(t, "demo", "--dir", dir, "--no-git", "--no-color", "--yes")
	if code == 0 {
		t.Fatal("expected a failure when the repository already exists")
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr = %q", stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("nothing should be written when the name is taken")
	}
}

func TestRunNoGHSkipsTheAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--no-gh must not call the API, got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	useStubGitHub(t, srv)

	dir := filepath.Join(t.TempDir(), "demo")
	if code, _, stderr := runCLI(t, "demo", "--dir", dir, "--no-gh", "--no-git",
		"--no-tidy", "--no-color"); code != 0 {
		t.Fatalf("exit code = %d, stderr:\n%s", code, stderr)
	}
}

func TestRunWiresTheGitRemote(t *testing.T) {
	if !gitutil.Available() {
		t.Skip("git is not installed")
	}
	srv, _ := stubGitHub(t, false)
	useStubGitHub(t, srv)

	// An isolated git identity keeps the commit step working anywhere.
	gitConfig := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(gitConfig, []byte("[user]\n\tname = Test\n\temail = test@example.com\n"), 0o644); err != nil {
		t.Fatalf("write gitconfig: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	dir := filepath.Join(t.TempDir(), "demo")
	code, stdout, stderr := runCLI(t, "demo", "--dir", dir, "--no-tidy", "--no-push", "--no-color", "--yes")
	if code != 0 {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	repo := &gitutil.Repo{Dir: dir}
	out, err := repo.Exec(t.Context(), "remote", "get-url", "origin")
	if err != nil {
		t.Fatalf("git remote get-url: %v", err)
	}
	if got := strings.TrimSpace(out); got != "https://github.com/octocat/demo.git" {
		t.Errorf("origin = %q", got)
	}
	if out, err := repo.Exec(t.Context(), "log", "--oneline"); err != nil || !strings.Contains(out, "Initial commit") {
		t.Errorf("initial commit missing (out=%q, err=%v)", out, err)
	}
}
