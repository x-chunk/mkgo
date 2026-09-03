package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestRepo returns a Repo in a temp directory with an isolated identity.
func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	if !Available() {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	gitConfig := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(gitConfig, []byte("[user]\n\tname = Test\n\temail = test@example.com\n"), 0o644); err != nil {
		t.Fatalf("write gitconfig: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	return &Repo{Dir: dir}
}

func TestInitCommitAndRemote(t *testing.T) {
	repo := newTestRepo(t)
	ctx := t.Context()

	if repo.IsRepo(ctx) {
		t.Fatal("a fresh temp directory should not be a repository")
	}
	if err := repo.Init(ctx, "trunk"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !repo.IsRepo(ctx) {
		t.Fatal("Init did not create a repository")
	}
	if branch, err := repo.CurrentBranch(ctx); err != nil || branch != "trunk" {
		t.Errorf("branch = %q (err=%v), want trunk", branch, err)
	}
	if !repo.HasIdentity(ctx) {
		t.Fatal("the test identity should be visible to git")
	}

	if err := os.WriteFile(filepath.Join(repo.Dir, "file.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := repo.AddAll(ctx); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if err := repo.Commit(ctx, "Initial commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	out, err := repo.Exec(ctx, "log", "--oneline")
	if err != nil || !strings.Contains(out, "Initial commit") {
		t.Errorf("log = %q (err=%v)", out, err)
	}

	if repo.HasRemote(ctx, "origin") {
		t.Error("there should be no origin yet")
	}
	if err := repo.AddRemote(ctx, "origin", "https://example.com/x.git"); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	if !repo.HasRemote(ctx, "origin") {
		t.Error("origin was not added")
	}
	if err := repo.SetRemote(ctx, "origin", "https://example.com/y.git"); err != nil {
		t.Fatalf("SetRemote: %v", err)
	}
	url, err := repo.Exec(ctx, "remote", "get-url", "origin")
	if err != nil || strings.TrimSpace(url) != "https://example.com/y.git" {
		t.Errorf("remote url = %q (err=%v)", url, err)
	}
}

func TestExecReportsGitErrors(t *testing.T) {
	repo := newTestRepo(t)
	_, err := repo.Exec(t.Context(), "log")
	if err == nil {
		t.Fatal("expected an error outside a repository")
	}
	if !strings.Contains(err.Error(), "git log") {
		t.Errorf("error should name the command: %v", err)
	}
}

func TestPushFailureIsRedacted(t *testing.T) {
	repo := newTestRepo(t)
	ctx := t.Context()
	if err := repo.Init(ctx, "main"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := repo.Commit(ctx, "Initial commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := repo.AddRemote(ctx, "origin", "https://127.0.0.1:1/nope.git"); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}

	err := repo.Push(ctx, PushOptions{Remote: "origin", Branch: "main", Token: "super-secret"})
	if err == nil {
		t.Fatal("expected the push to fail")
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Errorf("the token leaked into the error: %v", err)
	}
}
