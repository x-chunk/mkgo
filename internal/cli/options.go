// Package cli parses the command line and drives a project creation run.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// ErrHelp signals that usage was requested; the caller exits successfully.
var ErrHelp = errors.New("help requested")

// ErrVersion signals that the version was printed.
var ErrVersion = errors.New("version requested")

// Options is the fully parsed command line.
type Options struct {
	Name string // positional project name

	Dir         string
	Module      string
	Description string
	Author      string
	License     string
	Layout      string
	GoVersion   string
	Branch      string
	Org         string
	Topics      string
	Token       string
	Host        string
	ConfigPath  string
	Remote      string

	Private bool
	SSH     bool

	NoGit    bool
	NoGitHub bool
	NoMod    bool
	NoColor  bool
	NoEmoji  bool

	NoCommit       bool
	NoPush         bool
	NoCI           bool
	NoReadme       bool
	NoMakefile     bool
	NoEditorConfig bool
	NoGitignore    bool
	NoTidy         bool
	NoConfig       bool
	NoSpinner      bool

	Force   bool
	DryRun  bool
	Yes     bool
	Quiet   bool
	Verbose bool

	// set records which flags were provided so config defaults only apply to
	// the ones the user left alone.
	set map[string]bool
}

// Changed reports whether the named flag appeared on the command line.
func (o *Options) Changed(name string) bool { return o.set[name] }

// Parse reads argv into Options. Flags may appear before or after the
// positional project name.
func Parse(args []string, out io.Writer, version string) (*Options, error) {
	opts := &Options{set: map[string]bool{}}

	fs := flag.NewFlagSet("mkgo", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // usage is rendered by help.go instead
	var showHelp, showVersion bool

	fs.StringVar(&opts.Dir, "dir", "", "target directory (default ./<name>)")
	fs.StringVar(&opts.Dir, "d", "", "shorthand for --dir")
	fs.StringVar(&opts.Module, "module", "", "Go module path")
	fs.StringVar(&opts.Module, "m", "", "shorthand for --module")
	fs.StringVar(&opts.Description, "desc", "", "project description")
	fs.StringVar(&opts.Description, "description", "", "alias for --desc")
	fs.StringVar(&opts.Author, "author", "", "copyright holder for the license")
	fs.StringVar(&opts.License, "license", "", "license id: mit, apache-2.0, bsd-3-clause, isc, unlicense, none")
	fs.StringVar(&opts.Layout, "layout", "", "project layout: cli, lib, api, minimal")
	fs.StringVar(&opts.Layout, "t", "", "shorthand for --layout")
	fs.StringVar(&opts.Layout, "template", "", "alias for --layout")
	fs.StringVar(&opts.GoVersion, "go", "", "go directive for go.mod (default: this toolchain)")
	fs.StringVar(&opts.Branch, "branch", "", "initial branch name")
	fs.StringVar(&opts.Branch, "b", "", "shorthand for --branch")
	fs.StringVar(&opts.Org, "org", "", "create the GitHub repository in this organization")
	fs.StringVar(&opts.Topics, "topics", "", "comma separated GitHub topics")
	fs.StringVar(&opts.Token, "token", "", "GitHub token (default: $GITHUB_TOKEN)")
	fs.StringVar(&opts.Host, "host", "", "GitHub host for enterprise servers")
	fs.StringVar(&opts.ConfigPath, "config", "", "path to the mkgo config file")
	fs.StringVar(&opts.Remote, "remote", "", "git remote name (default origin)")

	fs.BoolVar(&opts.Private, "private", false, "create a private GitHub repository")
	fs.BoolVar(&opts.SSH, "ssh", false, "use the SSH remote URL instead of HTTPS")

	fs.BoolVar(&opts.NoGit, "no-git", false, "skip git initialization")
	fs.BoolVar(&opts.NoGitHub, "no-gh", false, "skip creating the GitHub repository")
	fs.BoolVar(&opts.NoMod, "no-mod", false, "skip creating go.mod")
	fs.BoolVar(&opts.NoColor, "no-color", false, "disable colored output")
	fs.BoolVar(&opts.NoEmoji, "no-emoji", false, "use ASCII markers instead of emoji")

	fs.BoolVar(&opts.NoCommit, "no-commit", false, "do not create the initial commit")
	fs.BoolVar(&opts.NoPush, "no-push", false, "do not push to the new remote")
	fs.BoolVar(&opts.NoCI, "no-ci", false, "do not add the GitHub Actions workflow")
	fs.BoolVar(&opts.NoReadme, "no-readme", false, "do not add README.md")
	fs.BoolVar(&opts.NoMakefile, "no-makefile", false, "do not add a Makefile")
	fs.BoolVar(&opts.NoEditorConfig, "no-editorconfig", false, "do not add .editorconfig")
	fs.BoolVar(&opts.NoGitignore, "no-gitignore", false, "do not add .gitignore")
	fs.BoolVar(&opts.NoTidy, "no-tidy", false, "skip go mod tidy and gofmt")
	fs.BoolVar(&opts.NoConfig, "no-config", false, "ignore the mkgo config file")
	fs.BoolVar(&opts.NoSpinner, "no-spinner", false, "disable spinner animation")

	fs.BoolVar(&opts.Force, "force", false, "write into a non-empty directory and overwrite files")
	fs.BoolVar(&opts.Force, "f", false, "shorthand for --force")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "show what would happen without touching anything")
	fs.BoolVar(&opts.DryRun, "n", false, "shorthand for --dry-run")
	fs.BoolVar(&opts.Yes, "yes", false, "answer every prompt with yes")
	fs.BoolVar(&opts.Yes, "y", false, "shorthand for --yes")
	fs.BoolVar(&opts.Quiet, "quiet", false, "only print errors")
	fs.BoolVar(&opts.Quiet, "q", false, "shorthand for --quiet")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print the commands mkgo runs")
	fs.BoolVar(&opts.Verbose, "v", false, "shorthand for --verbose")

	fs.BoolVar(&showHelp, "help", false, "show this help")
	fs.BoolVar(&showHelp, "h", false, "shorthand for --help")
	fs.BoolVar(&showVersion, "version", false, "print the mkgo version")
	fs.BoolVar(&showVersion, "V", false, "shorthand for --version")

	ordered, positional, err := permute(fs, args)
	if err != nil {
		return nil, err
	}
	if err := fs.Parse(ordered); err != nil {
		return nil, fmt.Errorf("%w\n\nRun 'mkgo --help' for usage", err)
	}
	fs.Visit(func(f *flag.Flag) { opts.set[f.Name] = true })

	if showHelp {
		color, emoji := helpRenderOptions(opts.NoColor, opts.NoEmoji)
		PrintUsage(out, version, color, emoji)
		return nil, ErrHelp
	}
	if showVersion {
		fmt.Fprintf(out, "mkgo %s\n", version)
		return nil, ErrVersion
	}

	positional = append(positional, fs.Args()...)
	switch len(positional) {
	case 0:
		// Left empty on purpose: Run reports a friendly error with usage.
	case 1:
		opts.Name = positional[0]
	default:
		return nil, fmt.Errorf("expected a single project name, got %d: %s",
			len(positional), strings.Join(positional, " "))
	}

	opts.normalizeAliases()
	return opts, nil
}

// normalizeAliases folds long and short spellings into one record so later
// code can ask Changed("dir") regardless of which spelling was used.
func (o *Options) normalizeAliases() {
	aliases := map[string][]string{
		"dir":      {"d"},
		"module":   {"m"},
		"desc":     {"description"},
		"layout":   {"t", "template"},
		"branch":   {"b"},
		"force":    {"f"},
		"dry-run":  {"n"},
		"yes":      {"y"},
		"quiet":    {"q"},
		"verbose":  {"v"},
		"no-color": {},
	}
	for canonical, alts := range aliases {
		for _, alt := range alts {
			if o.set[alt] {
				o.set[canonical] = true
			}
		}
	}
}

// permute splits argv into flags and positional arguments so the project name
// can be written before, between or after the flags.
func permute(fs *flag.FlagSet, args []string) (flags, positional []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			return flags, positional, nil
		case len(arg) > 1 && arg[0] == '-':
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			if strings.Contains(name, "=") {
				continue // value is attached, nothing to consume
			}
			f := fs.Lookup(name)
			if f == nil {
				return nil, nil, fmt.Errorf("unknown flag: %s\n\nRun 'mkgo --help' for usage", arg)
			}
			if isBoolFlag(f) {
				continue
			}
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("flag %s needs a value", arg)
			}
			i++
			flags = append(flags, args[i])
		default:
			positional = append(positional, arg)
		}
	}
	return flags, positional, nil
}

// isBoolFlag reports whether a flag can appear without a value.
func isBoolFlag(f *flag.Flag) bool {
	bf, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && bf.IsBoolFlag()
}
