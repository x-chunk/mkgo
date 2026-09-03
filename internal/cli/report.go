package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/x-chunk/mkgo/internal/github"
	"github.com/x-chunk/mkgo/internal/scaffold"
	"github.com/x-chunk/mkgo/internal/ui"
)

// printPlan shows what the run is about to do, including the file tree.
func printPlan(p *ui.Printer, plan *plan, files []scaffold.File) {
	c := p.C

	visibility := "public"
	if plan.Private {
		visibility = "private"
	}

	rows := []string{
		field(p, "project", c.Bold.Apply(plan.Name)),
		field(p, "directory", c.Path.Apply(shortPath(plan.Dir))),
		field(p, "module", plan.Module),
		field(p, "layout", string(plan.Layout)),
		field(p, "license", licenseLabel(plan.LicenseID)),
		field(p, "branch", plan.Branch),
	}
	if plan.GitHub {
		rows = append(rows, field(p, "repository", fmt.Sprintf("%s %s", plan.Slug(), c.Muted.Apply("("+visibility+")"))))
	}
	if len(plan.Topics) > 0 {
		rows = append(rows, field(p, "topics", strings.Join(plan.Topics, ", ")))
	}
	rows = append(rows, "", field(p, "steps", stepSummary(p, plan)))

	p.Box(c.Title.Apply("plan"), rows)
	p.Blank()
	p.Section(ui.IconFolder, "files")
	for _, line := range strings.Split(fileTree(p, plan.Name, files), "\n") {
		p.Line("  %s", line)
	}
}

// field formats an aligned label/value pair for the plan box.
func field(p *ui.Printer, key, value string) string {
	return p.C.Muted.Apply(ui.PadRight(key, 11)) + value
}

// stepSummary renders the enabled and disabled steps on one line.
func stepSummary(p *ui.Printer, plan *plan) string {
	type step struct {
		label string
		on    bool
	}
	steps := []step{
		{"files", true},
		{"go.mod", plan.Mod},
		{"git", plan.Git},
		{"commit", plan.Commit},
		{"github", plan.GitHub},
		{"push", plan.Push},
	}
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		if s.on {
			parts = append(parts, p.C.Success.Apply(s.label))
			continue
		}
		parts = append(parts, p.C.Muted.Apply(strikeLabel(s.label)))
	}
	return strings.Join(parts, p.C.Muted.Apply(" · "))
}

// strikeLabel marks a disabled step without relying on terminal strike-through.
func strikeLabel(label string) string { return "no " + label }

// licenseLabel names the license, or says none was requested.
func licenseLabel(id string) string {
	if id == "" {
		return "none"
	}
	return scaffold.LicenseName(id)
}

// fileTree renders the generated files as a colored directory tree.
func fileTree(p *ui.Printer, root string, files []scaffold.File) string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, filepath.ToSlash(f.Path))
	}

	var b strings.Builder
	b.WriteString(p.C.Path.Apply(root+"/") + "\n")
	for _, line := range scaffold.Tree(paths) {
		name := line.Name
		if line.IsDir {
			name = p.C.Path.Apply(name)
		}
		b.WriteString(p.C.Muted.Apply(line.Prefix) + name + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// printSummary closes the run with the result and the obvious next steps.
func printSummary(p *ui.Printer, plan *plan, files []scaffold.File, repo *github.Repository, elapsed time.Duration) {
	c := p.C
	p.Blank()

	lines := []string{
		field(p, "path", c.Path.Apply(shortPath(plan.Dir))),
		field(p, "module", plan.Module),
		field(p, "files", fmt.Sprintf("%d", len(files))),
	}
	if repo != nil {
		lines = append(lines, field(p, "repository", c.Underline.Apply(repo.HTMLURL)))
	}
	lines = append(lines, field(p, "elapsed", c.Muted.Apply(elapsed.Round(10*time.Millisecond).String())))

	title := fmt.Sprintf("%s %s", p.Icon(ui.IconSparkles), c.Success.Apply(plan.Name+" is ready"))
	p.Box(title, lines)

	p.Blank()
	p.Section(ui.IconArrow, "next steps")
	p.Item("%s", c.Command.Apply("cd "+shortPath(plan.Dir)))
	if plan.Layout == scaffold.LayoutLib {
		p.Item("%s", c.Command.Apply("go test ./..."))
	} else {
		p.Item("%s", c.Command.Apply("go run ."))
	}
	if plan.WithMakefile {
		p.Item("%s %s", c.Command.Apply("make check"), c.Muted.Apply("# format, vet and test"))
	}
	if repo == nil && plan.Git {
		p.Item("%s", c.Command.Apply("git push -u origin "+plan.Branch))
	}
}
