package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/x-chunk/mkgo/internal/config"
	"github.com/x-chunk/mkgo/internal/github"
	"github.com/x-chunk/mkgo/internal/gitutil"
	"github.com/x-chunk/mkgo/internal/scaffold"
	"github.com/x-chunk/mkgo/internal/ui"
)

// Environment carries the streams a run reads and writes.
type Environment struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	Version string
}

// Exit codes returned by Run.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// errAborted is returned when the user declines the confirmation prompt.
var errAborted = errors.New("aborted")

// Run executes mkgo end to end and returns the process exit code.
func Run(ctx context.Context, args []string, env Environment) int {
	opts, err := Parse(args, env.Stdout, env.Version)
	switch {
	case errors.Is(err, ErrHelp), errors.Is(err, ErrVersion):
		return exitOK
	case err != nil:
		fmt.Fprintln(env.Stderr, "mkgo:", err)
		return exitUsage
	}

	p := newPrinter(opts, env)
	defer p.StopSpinner()

	if err := run(ctx, opts, p, env); err != nil {
		if errors.Is(err, errAborted) {
			p.StopSpinner()
			p.Info("nothing was created")
			return exitOK
		}
		p.StopSpinner()
		p.Error("%v", err)
		return exitFailure
	}
	return exitOK
}

// newPrinter builds the output printer from the flags and the terminal state.
func newPrinter(opts *Options, env Environment) *ui.Printer {
	stdout, _ := env.Stdout.(*os.File)
	color := !opts.NoColor && ui.ColorSupported(stdout)
	animate := !opts.NoSpinner && !opts.Verbose && ui.IsTerminal(stdout)
	return ui.New(ui.Options{
		Out:     env.Stdout,
		Err:     env.Stderr,
		In:      env.Stdin,
		Color:   color,
		Emoji:   !opts.NoEmoji,
		Animate: animate,
		Quiet:   opts.Quiet,
		Verbose: opts.Verbose,
	})
}

// run performs the whole workflow: resolve, confirm, scaffold, git, GitHub.
func run(ctx context.Context, opts *Options, p *ui.Printer, env Environment) error {
	started := time.Now()

	cfg := &config.Config{}
	if !opts.NoConfig {
		loaded, err := config.Load(opts.ConfigPath)
		if err != nil {
			p.Warn("%v", err)
		} else {
			cfg = loaded
		}
	}

	plan, err := resolve(ctx, opts, cfg)
	if err != nil {
		return err
	}

	p.Banner(env.Version)
	if cfg.Path != "" {
		p.Debug("config loaded from %s", cfg.Path)
	}

	// Preflight checks decide what is actually possible before anything is
	// written to disk.
	var client *github.Client
	if plan.GitHub {
		client, err = preflightGitHub(ctx, opts, plan, p)
		if err != nil {
			return err
		}
		if client == nil {
			plan.GitHub = false
			plan.Push = false
		}
	}
	plan.resolveModule(opts, cfg)

	if plan.Git && !gitutil.Available() {
		p.Warn("git was not found in PATH; skipping git initialization")
		plan.Git, plan.Commit, plan.Push = false, false, false
	}
	if plan.Tidy && !goAvailable() {
		p.Debug("go was not found in PATH; skipping gofmt and go mod tidy")
		plan.Tidy = false
	}

	files, err := scaffold.Build(scaffold.Spec{
		Name:          plan.Name,
		Module:        plan.Module,
		Description:   plan.Description,
		Author:        firstNonEmpty(plan.Author, plan.Owner, "the authors"),
		LicenseID:     plan.LicenseID,
		GoVersion:     plan.GoVersion,
		Branch:        plan.Branch,
		Layout:        plan.Layout,
		WithMod:       plan.Mod,
		WithReadme:    plan.WithReadme,
		WithMakefile:  plan.WithMakefile,
		WithCI:        plan.WithCI,
		WithGitignore: plan.WithGitignore,
		WithEditorCfg: plan.WithEditorCfg,
	})
	if err != nil {
		return err
	}

	printPlan(p, plan, files)

	if opts.DryRun {
		p.Blank()
		p.Info("dry run: nothing was created")
		return nil
	}

	if err := confirmPlan(p, opts, plan); err != nil {
		return err
	}
	p.Blank()

	if err := createDirectory(p, plan, opts); err != nil {
		return err
	}
	if err := writeFiles(p, plan, files, opts.Force); err != nil {
		return err
	}
	if plan.Tidy {
		tidyProject(ctx, p, plan)
	}

	repo := &gitutil.Repo{Dir: plan.Dir}
	if err := initGit(ctx, p, plan, repo); err != nil {
		return err
	}

	var created *github.Repository
	if plan.GitHub {
		created, err = createRemoteRepo(ctx, p, plan, client)
		if err != nil {
			return err
		}
	}
	if created != nil && plan.Git {
		if err := wireRemote(ctx, p, plan, repo, created); err != nil {
			return err
		}
	}
	if plan.Push && created != nil {
		pushBranch(ctx, p, plan, repo, opts)
	}

	printSummary(p, plan, files, created, time.Since(started))
	return nil
}

// preflightGitHub verifies the token and the repository name. A nil client
// with a nil error means "continue without GitHub".
func preflightGitHub(ctx context.Context, opts *Options, plan *plan, p *ui.Printer) (*github.Client, error) {
	source := github.ResolveToken(opts.Token, plan.Host)
	if source.Token == "" {
		if opts.Changed("private") || opts.Changed("org") || opts.Changed("topics") || opts.Changed("token") {
			return nil, fmt.Errorf("no GitHub token found; set GITHUB_TOKEN or pass --token (or use --no-gh)")
		}
		p.Warn("no GitHub token found; skipping repository creation (pass --no-gh to silence this)")
		return nil, nil
	}
	plan.TokenOrigin = source.Origin

	step := p.StartStep("checking GitHub credentials")
	client := github.New(source.Token, plan.APIBase, "mkgo")
	viewer, err := client.Viewer(ctx)
	if err != nil {
		step.Fail("GitHub credentials rejected")
		return nil, errors.New(github.HumanizeError(err))
	}
	plan.Owner = viewer.Login
	step.Done("authenticated as %s %s", p.C.Bold.Apply(viewer.Login), p.C.Muted.Apply("("+source.Origin+")"))

	step = p.StartStep("checking whether %s is available", plan.Slug())
	exists, err := client.RepoExists(ctx, ownerOf(plan), plan.Name)
	if err != nil {
		step.Warn("could not check whether %s exists: %s", plan.Slug(), github.HumanizeError(err))
		return client, nil
	}
	if exists {
		step.Fail("%s already exists on %s", plan.Slug(), plan.Host)
		return nil, fmt.Errorf("repository %s already exists; pick another name or pass --no-gh", plan.Slug())
	}
	step.Done("%s is available", plan.Slug())
	return client, nil
}

// ownerOf returns the account the repository will belong to.
func ownerOf(p *plan) string {
	if p.Org != "" {
		return p.Org
	}
	return p.Owner
}

// createDirectory makes the project directory, asking before reusing a
// non-empty one.
func createDirectory(p *ui.Printer, plan *plan, opts *Options) error {
	step := p.StartStep("creating %s", shortPath(plan.Dir))
	if plan.DirExists {
		empty, err := isDirEmpty(plan.Dir)
		if err != nil {
			step.Fail("cannot read %s", shortPath(plan.Dir))
			return err
		}
		if !empty && !opts.Force {
			step.Fail("%s is not empty", shortPath(plan.Dir))
			return fmt.Errorf("pass --force to scaffold into a directory that already has files")
		}
		step.Skip("using existing directory %s", shortPath(plan.Dir))
		return nil
	}
	if err := os.MkdirAll(plan.Dir, 0o755); err != nil {
		step.Fail("could not create %s", shortPath(plan.Dir))
		return err
	}
	step.Done("created %s", p.C.Path.Apply(shortPath(plan.Dir)))
	return nil
}

// writeFiles materializes the scaffold.
func writeFiles(p *ui.Printer, plan *plan, files []scaffold.File, force bool) error {
	step := p.StartStep("writing project files")
	if err := scaffold.Write(plan.Dir, files, force); err != nil {
		step.Fail("could not write the project files")
		return err
	}
	for _, f := range files {
		p.Debug("wrote %s", f.Path)
	}
	step.Done("wrote %d files", len(files))
	return nil
}

// tidyProject runs gofmt and go mod tidy so the generated code is canonical.
func tidyProject(ctx context.Context, p *ui.Printer, plan *plan) {
	step := p.StartStep("tidying the module")
	if out, err := runCommand(ctx, plan.Dir, "gofmt", "-w", "."); err != nil {
		step.Warn("gofmt failed: %s", firstLine(out, err))
		return
	}
	step.Update("running go mod tidy")
	if out, err := runCommand(ctx, plan.Dir, "go", "mod", "tidy"); err != nil {
		step.Warn("go mod tidy failed: %s", firstLine(out, err))
		return
	}
	step.Done("formatted the code and tidied go.mod")
}

// initGit initializes the repository and records the first commit.
func initGit(ctx context.Context, p *ui.Printer, plan *plan, repo *gitutil.Repo) error {
	if !plan.Git {
		p.StartStep("git").Skip("git initialization skipped (--no-git)")
		return nil
	}
	step := p.StartStep("initializing the git repository")
	if repo.IsRepo(ctx) {
		step.Skip("%s is already a git repository", shortPath(plan.Dir))
	} else {
		if err := repo.Init(ctx, plan.Branch); err != nil {
			step.Fail("git init failed")
			return err
		}
		step.Done("initialized a repository on branch %s", p.C.Bold.Apply(plan.Branch))
	}

	if !plan.Commit {
		p.StartStep("commit").Skip("initial commit skipped (--no-commit)")
		return nil
	}

	step = p.StartStep("creating the initial commit")
	if !repo.HasIdentity(ctx) {
		step.Warn("skipped the commit: git user.name and user.email are not configured")
		plan.Push = false
		return nil
	}
	if err := repo.AddAll(ctx); err != nil {
		step.Fail("git add failed")
		return err
	}
	if err := repo.Commit(ctx, "Initial commit"); err != nil {
		step.Fail("git commit failed")
		return err
	}
	step.Done("committed the initial tree")
	return nil
}

// createRemoteRepo creates the repository through the GitHub REST API.
func createRemoteRepo(ctx context.Context, p *ui.Printer, plan *plan, client *github.Client) (*github.Repository, error) {
	visibility := "public"
	if plan.Private {
		visibility = "private"
	}
	step := p.StartStep("creating the %s repository %s", visibility, plan.Slug())
	repo, err := client.CreateRepository(ctx, github.CreateRepoOptions{
		Name:        plan.Name,
		Description: plan.Description,
		Private:     plan.Private,
		Org:         plan.Org,
		HasIssues:   true,
		Topics:      plan.Topics,
	})
	if err != nil {
		if repo != nil {
			// The repository exists; only the topics call failed.
			step.Warn("created %s but could not set topics: %v", repo.FullName, err)
			return repo, nil
		}
		step.Fail("could not create the repository")
		return nil, errors.New(github.HumanizeError(err))
	}
	icon := ui.IconGlobe
	if repo.Private {
		icon = ui.IconLock
	}
	step.Done("created %s %s", p.C.Bold.Apply(repo.FullName), p.Icon(icon))
	return repo, nil
}

// wireRemote points the local repository at the freshly created remote.
func wireRemote(ctx context.Context, p *ui.Printer, plan *plan, repo *gitutil.Repo, created *github.Repository) error {
	url := plan.RemoteURL()
	if created != nil {
		if plan.SSH && created.SSHURL != "" {
			url = created.SSHURL
		} else if !plan.SSH && created.CloneURL != "" {
			url = created.CloneURL
		}
	}
	step := p.StartStep("configuring the %s remote", plan.Remote)
	var err error
	if repo.HasRemote(ctx, plan.Remote) {
		err = repo.SetRemote(ctx, plan.Remote, url)
	} else {
		err = repo.AddRemote(ctx, plan.Remote, url)
	}
	if err != nil {
		step.Fail("could not configure the remote")
		return err
	}
	step.Done("%s %s %s", plan.Remote, p.Icon(ui.IconArrow), p.C.Path.Apply(url))
	return nil
}

// pushBranch publishes the initial commit. A failure here is reported but
// does not undo the work that already succeeded.
func pushBranch(ctx context.Context, p *ui.Printer, plan *plan, repo *gitutil.Repo, opts *Options) {
	step := p.StartStep("pushing %s to %s", plan.Branch, plan.Remote)
	token := ""
	if !plan.SSH {
		token = github.ResolveToken(opts.Token, plan.Host).Token
	}
	branch, err := repo.CurrentBranch(ctx)
	if err != nil || branch == "" {
		branch = plan.Branch
	}
	if err := repo.Push(ctx, gitutil.PushOptions{Remote: plan.Remote, Branch: branch, Token: token}); err != nil {
		step.Warn("push failed: %v", err)
		p.Item("push it yourself with %s", p.C.Command.Apply("git push -u "+plan.Remote+" "+branch))
		return
	}
	step.Done("pushed %s to %s", p.C.Bold.Apply(branch), plan.Remote)
}

// confirmPlan asks for confirmation before anything outward facing happens.
func confirmPlan(p *ui.Printer, opts *Options, plan *plan) error {
	if opts.Yes || opts.Quiet || !plan.GitHub {
		return nil
	}
	p.Blank()
	question := fmt.Sprintf("Create %s on %s?", p.C.Bold.Apply(plan.Slug()), plan.Host)
	ok, err := p.Confirm(question, true)
	if err != nil {
		return err
	}
	if !ok {
		return errAborted
	}
	return nil
}

// isDirEmpty reports whether a directory has no entries.
func isDirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// goAvailable reports whether the go toolchain is on PATH.
func goAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

// runCommand executes a helper command inside the project directory.
func runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// firstLine picks the most useful line out of a failed command.
func firstLine(out string, err error) string {
	for _, line := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return err.Error()
}

// shortPath renders a path relative to the working directory when that is
// shorter, and abbreviates the home directory as "~".
func shortPath(path string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return "."
			}
			return "./" + rel
		}
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
