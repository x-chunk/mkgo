package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/x-chunk/mkgo/internal/config"
	"github.com/x-chunk/mkgo/internal/gitutil"
	"github.com/x-chunk/mkgo/internal/scaffold"
)

// namePattern matches the characters GitHub accepts in a repository name.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// plan is the resolved description of what a run will do. Everything the
// steps need is decided up front so the plan can be shown before any change.
type plan struct {
	Name        string
	Dir         string
	Module      string
	Description string
	Author      string
	LicenseID   string
	Layout      scaffold.Layout
	GoVersion   string
	Branch      string
	Remote      string

	Git    bool
	Commit bool
	GitHub bool
	Push   bool
	Mod    bool
	Tidy   bool

	Private bool
	Org     string
	SSH     bool
	Topics  []string
	Host    string
	APIBase string

	Owner       string // filled in once the token is verified
	TokenOrigin string

	WithReadme    bool
	WithMakefile  bool
	WithCI        bool
	WithGitignore bool
	WithEditorCfg bool

	DirExists bool
}

// resolve merges flags with the config file and derives everything else.
func resolve(ctx context.Context, opts *Options, cfg *config.Config) (*plan, error) {
	p := &plan{}

	name, dir, err := resolveNameAndDir(opts)
	if err != nil {
		return nil, err
	}
	p.Name, p.Dir = name, dir

	if !namePattern.MatchString(p.Name) {
		return nil, fmt.Errorf("invalid project name %q: use letters, digits, '.', '-' and '_'", p.Name)
	}

	layoutName := firstNonEmpty(opts.Layout, cfg.Layout, string(scaffold.LayoutCLI))
	layout, err := scaffold.ParseLayout(layoutName)
	if err != nil {
		return nil, err
	}
	p.Layout = layout

	licenseID, err := scaffold.NormalizeLicense(firstNonEmpty(opts.License, cfg.License, "mit"))
	if err != nil {
		return nil, err
	}
	p.LicenseID = licenseID

	p.Branch = firstNonEmpty(opts.Branch, cfg.Branch, "main")
	p.GoVersion = firstNonEmpty(opts.GoVersion, cfg.GoVersion, scaffold.DefaultGoVersion())
	p.Remote = firstNonEmpty(opts.Remote, "origin")
	p.Description = firstNonEmpty(opts.Description, defaultDescription(p.Name, layout))
	p.Org = firstNonEmpty(opts.Org, cfg.Org)
	p.Host = normalizeHost(firstNonEmpty(opts.Host, cfg.Host, "github.com"))
	p.APIBase = apiBaseURL(p.Host)
	p.Private = opts.Private || (!opts.Changed("private") && config.Bool(cfg.Private, false))
	p.SSH = opts.SSH || (!opts.Changed("ssh") && config.Bool(cfg.SSH, false))
	p.Topics = splitTopics(firstNonEmpty(opts.Topics, strings.Join(cfg.Topics, ",")))

	p.Git = !opts.NoGit
	p.GitHub = !opts.NoGitHub
	p.Mod = !opts.NoMod
	p.Commit = p.Git && !opts.NoCommit
	p.Push = p.GitHub && p.Commit && !opts.NoPush
	p.Tidy = p.Mod && !opts.NoTidy

	p.WithReadme = !opts.NoReadme
	p.WithMakefile = !opts.NoMakefile
	p.WithCI = !opts.NoCI
	p.WithGitignore = !opts.NoGitignore
	p.WithEditorCfg = !opts.NoEditorConfig

	p.Author = firstNonEmpty(opts.Author, cfg.Author, gitConfigValue(ctx, "user.name"))

	if info, err := os.Stat(p.Dir); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("%s exists and is not a directory", p.Dir)
		}
		p.DirExists = true
	}

	return p, nil
}

// resolveModule decides the module path once the GitHub owner is known.
func (p *plan) resolveModule(opts *Options, cfg *config.Config) {
	switch {
	case opts.Module != "":
		p.Module = opts.Module
	case cfg.ModulePrefix != "":
		p.Module = strings.TrimSuffix(cfg.ModulePrefix, "/") + "/" + p.Name
	case p.Owner != "":
		p.Module = fmt.Sprintf("%s/%s/%s", p.Host, p.Owner, p.Name)
	default:
		// A bare name is a valid module path, and keeps "go build" working
		// offline for projects that are not published anywhere.
		p.Module = p.Name
	}
}

// Slug returns the owner/name pair of the GitHub repository.
func (p *plan) Slug() string {
	owner := p.Org
	if owner == "" {
		owner = p.Owner
	}
	if owner == "" {
		return p.Name
	}
	return owner + "/" + p.Name
}

// RemoteURL builds the git remote for the created repository.
func (p *plan) RemoteURL() string {
	if p.SSH {
		return fmt.Sprintf("git@%s:%s.git", p.Host, p.Slug())
	}
	return fmt.Sprintf("https://%s/%s.git", p.Host, p.Slug())
}

// resolveNameAndDir turns the positional argument into a project name and a
// target directory. A path is accepted, and "." means the current directory.
func resolveNameAndDir(opts *Options) (name, dir string, err error) {
	raw := strings.TrimSpace(opts.Name)
	if raw == "" {
		return "", "", fmt.Errorf("missing project name\n\nUsage: mkgo [flags] <project-name>")
	}

	cleaned := filepath.Clean(raw)
	switch {
	case opts.Dir != "":
		dir = opts.Dir
		name = filepath.Base(cleaned)
	case cleaned == "." || cleaned == string(filepath.Separator):
		wd, wderr := os.Getwd()
		if wderr != nil {
			return "", "", fmt.Errorf("resolve current directory: %w", wderr)
		}
		dir = wd
		name = filepath.Base(wd)
	default:
		dir = cleaned
		name = filepath.Base(cleaned)
	}

	if name == "." || name == string(filepath.Separator) || name == "" {
		return "", "", fmt.Errorf("cannot derive a project name from %q", raw)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("resolve %s: %w", dir, err)
	}
	return name, abs, nil
}

// defaultDescription writes a sensible one-liner when none is given.
func defaultDescription(name string, layout scaffold.Layout) string {
	switch layout {
	case scaffold.LayoutAPI:
		return fmt.Sprintf("%s is an HTTP service written in Go.", name)
	case scaffold.LayoutLib:
		return fmt.Sprintf("%s is a Go library.", name)
	case scaffold.LayoutMinimal:
		return fmt.Sprintf("%s is a Go program.", name)
	default:
		return fmt.Sprintf("%s is a command line tool written in Go.", name)
	}
}

// gitConfigValue reads a git configuration value, ignoring failures.
func gitConfigValue(ctx context.Context, key string) string {
	if !gitutil.Available() {
		return ""
	}
	repo := &gitutil.Repo{}
	out, err := repo.Exec(ctx, "config", "--get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// normalizeHost strips a scheme or trailing slash from a host name.
func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")
	if host == "" {
		return "github.com"
	}
	return host
}

// apiBaseURL maps a host to its REST API root.
func apiBaseURL(host string) string {
	if host == "github.com" || host == "" {
		if custom := os.Getenv("GITHUB_API_URL"); custom != "" {
			return strings.TrimSuffix(custom, "/")
		}
		return "https://api.github.com"
	}
	return "https://" + host + "/api/v3"
}

// splitTopics parses a comma separated topic list.
func splitTopics(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	topics := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.ToLower(strings.TrimSpace(p)); t != "" {
			topics = append(topics, t)
		}
	}
	return topics
}

// firstNonEmpty returns the first value that is not empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
