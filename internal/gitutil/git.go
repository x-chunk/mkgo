// Package gitutil wraps the local git binary for the handful of operations
// mkgo performs: init, commit and push.
package gitutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrGitNotFound is returned when no git executable is on PATH.
var ErrGitNotFound = errors.New("git executable not found in PATH")

// Repo runs git commands inside a working directory.
type Repo struct {
	Dir string
	Env []string // extra environment entries applied to every command
}

// Available reports whether git can be executed at all.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// run executes git and returns its combined output. Errors carry the trimmed
// stderr so callers can show something meaningful.
func run(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", ErrGitNotFound
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), firstLines(detail, 4))
	}
	return stdout.String(), nil
}

// firstLines trims long git output down to the informative part.
func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(strings.Join(lines[:n], "\n")) + " …"
}

// Exec runs an arbitrary git command in the repository directory.
func (r *Repo) Exec(ctx context.Context, args ...string) (string, error) {
	return run(ctx, r.Dir, r.Env, args...)
}

// Init creates a repository with the requested initial branch name.
func (r *Repo) Init(ctx context.Context, branch string) error {
	args := []string{"init"}
	if branch != "" {
		args = append(args, "--initial-branch="+branch)
	}
	if _, err := r.Exec(ctx, args...); err != nil {
		// Older git versions do not know --initial-branch; fall back to
		// init plus an explicit branch rename.
		if branch == "" || !strings.Contains(err.Error(), "initial-branch") {
			return err
		}
		if _, err := r.Exec(ctx, "init"); err != nil {
			return err
		}
		if _, err := r.Exec(ctx, "checkout", "-b", branch); err != nil {
			return err
		}
	}
	return nil
}

// IsRepo reports whether the directory is already inside a git work tree.
func (r *Repo) IsRepo(ctx context.Context) bool {
	out, err := r.Exec(ctx, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// AddAll stages every file in the work tree.
func (r *Repo) AddAll(ctx context.Context) error {
	_, err := r.Exec(ctx, "add", "--all")
	return err
}

// Commit records a commit, allowing an empty tree so a bare project still gets
// a starting point.
func (r *Repo) Commit(ctx context.Context, message string) error {
	_, err := r.Exec(ctx, "commit", "--allow-empty", "-m", message)
	return err
}

// HasIdentity reports whether user.name and user.email are configured, which
// git requires before it will create a commit.
func (r *Repo) HasIdentity(ctx context.Context) bool {
	name, errName := r.Exec(ctx, "config", "--get", "user.name")
	email, errEmail := r.Exec(ctx, "config", "--get", "user.email")
	return errName == nil && errEmail == nil &&
		strings.TrimSpace(name) != "" && strings.TrimSpace(email) != ""
}

// AddRemote points "origin" (or another name) at a URL.
func (r *Repo) AddRemote(ctx context.Context, name, url string) error {
	_, err := r.Exec(ctx, "remote", "add", name, url)
	return err
}

// SetRemote updates an existing remote URL.
func (r *Repo) SetRemote(ctx context.Context, name, url string) error {
	_, err := r.Exec(ctx, "remote", "set-url", name, url)
	return err
}

// HasRemote reports whether a remote with the given name already exists.
func (r *Repo) HasRemote(ctx context.Context, name string) bool {
	out, err := r.Exec(ctx, "remote")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// CurrentBranch returns the checked out branch name. It works on an unborn
// branch too, which is the state right after git init.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	if out, err := r.Exec(ctx, "branch", "--show-current"); err == nil {
		if branch := strings.TrimSpace(out); branch != "" {
			return branch, nil
		}
	}
	out, err := r.Exec(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out), err
}

// PushOptions configures the initial push.
type PushOptions struct {
	Remote string
	Branch string
	// Token, when set, is supplied through a one-shot credential helper so it
	// never lands in the remote URL, the reflog or the process arguments.
	Token string
}

// credentialHelper builds an inline helper that answers git's credential
// query from an environment variable.
const credentialHelper = `!f() { test "$1" = get && printf 'username=x-access-token\npassword=%s\n' "$MKGO_GIT_TOKEN"; }; f`

// Push publishes the branch and sets it as the upstream.
func (r *Repo) Push(ctx context.Context, opts PushOptions) error {
	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}
	args := []string{}
	env := []string{"GIT_TERMINAL_PROMPT=0"}
	if opts.Token != "" {
		// The empty helper resets any inherited chain so only ours is asked.
		args = append(args, "-c", "credential.helper=", "-c", "credential.helper="+credentialHelper)
		env = append(env, "MKGO_GIT_TOKEN="+opts.Token)
	}
	args = append(args, "push", "--set-upstream", remote, opts.Branch)

	saved := r.Env
	r.Env = append(append([]string{}, saved...), env...)
	defer func() { r.Env = saved }()

	out, err := r.Exec(ctx, args...)
	if err != nil {
		return errors.New(redact(err.Error(), opts.Token))
	}
	_ = out
	return nil
}

// redact removes a secret from text that may be printed.
func redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}
