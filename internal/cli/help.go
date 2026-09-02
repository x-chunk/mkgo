package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/x-chunk/mkgo/internal/ui"
)

// usageSection is a titled group of flag descriptions.
type usageSection struct {
	title string
	rows  [][2]string
}

// usageSections describes every flag in the order it should be documented.
var usageSections = []usageSection{
	{"Project", [][2]string{
		{"-d, --dir <path>", "target directory (default ./<name>)"},
		{"-m, --module <path>", "Go module path (default github.com/<owner>/<name>)"},
		{"-t, --layout <kind>", "cli, lib, api or minimal (default cli)"},
		{"    --desc <text>", "short project description"},
		{"    --author <name>", "copyright holder written into the license"},
		{"    --license <id>", "mit, apache-2.0, bsd-3-clause, isc, unlicense, none"},
		{"    --go <version>", "go directive for go.mod (default: this toolchain)"},
		{"-b, --branch <name>", "initial branch name (default main)"},
	}},
	{"GitHub", [][2]string{
		{"    --private", "create the repository as private"},
		{"    --org <name>", "create it inside an organization"},
		{"    --topics <list>", "comma separated repository topics"},
		{"    --token <token>", "GitHub token (default $GITHUB_TOKEN)"},
		{"    --host <host>", "GitHub Enterprise host"},
		{"    --ssh", "use the SSH remote URL instead of HTTPS"},
		{"    --remote <name>", "name of the git remote (default origin)"},
	}},
	{"Skip steps", [][2]string{
		{"    --no-git", "do not run git init"},
		{"    --no-gh", "do not create the GitHub repository"},
		{"    --no-mod", "do not create go.mod"},
		{"    --no-commit", "do not create the initial commit"},
		{"    --no-push", "do not push to the remote"},
		{"    --no-ci", "do not add the GitHub Actions workflow"},
		{"    --no-readme", "do not add README.md"},
		{"    --no-makefile", "do not add a Makefile"},
		{"    --no-gitignore", "do not add .gitignore"},
		{"    --no-editorconfig", "do not add .editorconfig"},
		{"    --no-tidy", "do not run gofmt and go mod tidy"},
		{"    --no-config", "ignore the mkgo config file"},
	}},
	{"Output", [][2]string{
		{"    --no-color", "disable ANSI colors (also honors $NO_COLOR)"},
		{"    --no-emoji", "use ASCII markers instead of emoji"},
		{"    --no-spinner", "disable the spinner animation"},
		{"-q, --quiet", "print errors only"},
		{"-v, --verbose", "print every command mkgo runs"},
	}},
	{"Behavior", [][2]string{
		{"-n, --dry-run", "show the plan without creating anything"},
		{"-y, --yes", "do not ask for confirmation"},
		{"-f, --force", "write into a non-empty directory"},
		{"    --config <path>", "path to the config file"},
		{"-h, --help", "show this help"},
		{"-V, --version", "print the mkgo version"},
	}},
}

// examples shown at the bottom of the help screen.
var examples = [][2]string{
	{"mkgo hello-world", "scaffold, git init, create the repo and push"},
	{"mkgo api-gateway -t api --private", "a private HTTP service"},
	{"mkgo toolkit -t lib --no-gh", "a library, local only"},
	{"mkgo demo --no-git --no-gh --no-mod", "just the files"},
	{"mkgo demo --dry-run", "print the plan and exit"},
}

// PrintUsage renders the help screen.
func PrintUsage(w io.Writer, version string, color, emoji bool) {
	p := ui.New(ui.Options{Out: w, Err: w, Color: color, Emoji: emoji})
	c := p.C

	fmt.Fprintf(w, "%s %s %s\n", p.Icon(ui.IconRocket), c.Title.Apply("mkgo"), c.Muted.Apply(version))
	fmt.Fprintf(w, "%s\n\n", c.Muted.Apply("Create a Go project, a git repository and a GitHub repository in one command."))

	fmt.Fprintf(w, "%s\n", c.Bold.Apply("USAGE"))
	fmt.Fprintf(w, "  %s %s\n\n", c.Command.Apply("mkgo"), "[flags] <project-name>")

	width := 0
	for _, section := range usageSections {
		for _, row := range section.rows {
			if len(row[0]) > width {
				width = len(row[0])
			}
		}
	}

	for _, section := range usageSections {
		fmt.Fprintf(w, "%s\n", c.Bold.Apply(strings.ToUpper(section.title)))
		for _, row := range section.rows {
			fmt.Fprintf(w, "  %s  %s\n", c.Accent.Apply(padRight(row[0], width)), c.Muted.Apply(row[1]))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n", c.Bold.Apply("EXAMPLES"))
	exWidth := 0
	for _, ex := range examples {
		if len(ex[0]) > exWidth {
			exWidth = len(ex[0])
		}
	}
	for _, ex := range examples {
		fmt.Fprintf(w, "  %s  %s\n", c.Command.Apply(padRight(ex[0], exWidth)), c.Muted.Apply("# "+ex[1]))
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "%s\n", c.Bold.Apply("ENVIRONMENT"))
	env := [][2]string{
		{"GITHUB_TOKEN, GH_TOKEN", "token used for the GitHub API"},
		{"NO_COLOR", "disable colors without passing --no-color"},
		{"MKGO_CONFIG_DIR", "directory holding config.json"},
	}
	envWidth := 0
	for _, e := range env {
		if len(e[0]) > envWidth {
			envWidth = len(e[0])
		}
	}
	for _, e := range env {
		fmt.Fprintf(w, "  %s  %s\n", c.Info.Apply(padRight(e[0], envWidth)), c.Muted.Apply(e[1]))
	}
}

// padRight pads plain ASCII help text; help rows never contain wide runes.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// helpRenderOptions decides how the help screen should look before the full
// printer exists.
func helpRenderOptions(noColor, noEmoji bool) (color, emoji bool) {
	color = !noColor && ui.ColorSupported(os.Stdout)
	emoji = !noEmoji
	return color, emoji
}
