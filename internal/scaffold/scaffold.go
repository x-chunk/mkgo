// Package scaffold renders the files that make up a new project.
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Layout names the set of files a project starts with.
type Layout string

// The available project layouts.
const (
	LayoutCLI     Layout = "cli"
	LayoutLib     Layout = "lib"
	LayoutAPI     Layout = "api"
	LayoutMinimal Layout = "minimal"
)

// Layouts lists every supported layout in a stable order.
func Layouts() []string {
	return []string{string(LayoutCLI), string(LayoutLib), string(LayoutAPI), string(LayoutMinimal)}
}

// ParseLayout validates a layout name coming from the command line.
func ParseLayout(s string) (Layout, error) {
	switch Layout(strings.ToLower(strings.TrimSpace(s))) {
	case LayoutCLI:
		return LayoutCLI, nil
	case LayoutLib, "library":
		return LayoutLib, nil
	case LayoutAPI, "server", "http":
		return LayoutAPI, nil
	case LayoutMinimal, "min", "bare":
		return LayoutMinimal, nil
	default:
		return "", fmt.Errorf("unknown layout %q (available: %s)", s, strings.Join(Layouts(), ", "))
	}
}

// Spec describes the project to generate.
type Spec struct {
	Name        string
	Module      string
	Description string
	Author      string
	Year        int
	LicenseID   string // canonical SPDX id; empty means no LICENSE file
	GoVersion   string
	Branch      string
	Layout      Layout

	WithMod       bool
	WithReadme    bool
	WithMakefile  bool
	WithCI        bool
	WithGitignore bool
	WithEditorCfg bool
}

// File is a single generated file.
type File struct {
	Path    string
	Content string
	Mode    os.FileMode
}

// view is the data handed to the templates.
type view struct {
	Name        string
	Module      string
	Package     string
	Binary      string
	Description string
	Author      string
	Year        int
	License     string
	LicenseID   string
	GoVersion   string
	Branch      string
	Template    string
	Layout      string
	IsCommand   bool
	HasLicense  bool
	HasMod      bool
}

// Build renders every file of the project without touching the disk.
func Build(spec Spec) ([]File, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	if spec.Year == 0 {
		spec.Year = time.Now().Year()
	}
	if spec.Layout == "" {
		spec.Layout = LayoutCLI
	}
	if spec.GoVersion == "" {
		spec.GoVersion = DefaultGoVersion()
	}
	if spec.Branch == "" {
		spec.Branch = "main"
	}
	if spec.Description == "" {
		spec.Description = "A Go project."
	}

	v := view{
		Name:        spec.Name,
		Module:      spec.Module,
		Package:     PackageName(spec.Name),
		Binary:      BinaryName(spec.Name),
		Description: spec.Description,
		Author:      spec.Author,
		Year:        spec.Year,
		License:     LicenseName(spec.LicenseID),
		LicenseID:   spec.LicenseID,
		GoVersion:   spec.GoVersion,
		Branch:      spec.Branch,
		Template:    string(spec.Layout),
		IsCommand:   spec.Layout != LayoutLib,
		HasLicense:  spec.LicenseID != "",
		HasMod:      spec.WithMod,
	}
	if v.Module == "" {
		v.Module = v.Package
	}

	var files []File

	// Source files for the selected layout.
	switch spec.Layout {
	case LayoutMinimal:
		files = append(files, tmplFile("main.go", "main_minimal.go.tmpl"))
	case LayoutCLI:
		files = append(files,
			tmplFile("main.go", "main_cli.go.tmpl"),
			tmplFile(filepath.Join("internal", "app", "app.go"), "app_run.go.tmpl"),
			tmplFile(filepath.Join("internal", "app", "app_test.go"), "app_run_test.go.tmpl"),
		)
	case LayoutAPI:
		files = append(files,
			tmplFile("main.go", "main_api.go.tmpl"),
			tmplFile(filepath.Join("internal", "server", "server.go"), "server.go.tmpl"),
			tmplFile(filepath.Join("internal", "server", "server_test.go"), "server_test.go.tmpl"),
		)
	case LayoutLib:
		files = append(files,
			tmplFile(v.Package+".go", "lib.go.tmpl"),
			tmplFile(v.Package+"_test.go", "lib_test.go.tmpl"),
		)
	}

	if spec.WithMod {
		files = append(files, File{Path: "go.mod", Content: goModContent(v.Module, spec.GoVersion)})
	}
	if spec.WithGitignore {
		files = append(files, tmplFile(".gitignore", "gitignore.tmpl"))
	}
	if spec.WithEditorCfg {
		files = append(files, tmplFile(".editorconfig", "editorconfig.tmpl"))
	}
	if spec.WithMakefile {
		files = append(files, tmplFile("Makefile", "Makefile.tmpl"))
	}
	if spec.WithCI {
		files = append(files, tmplFile(filepath.Join(".github", "workflows", "ci.yml"), "ci.yml.tmpl"))
	}

	// The README lists the tree, so it is rendered once the rest is known.
	if spec.WithReadme {
		files = append(files, tmplFile("README.md", "README.md.tmpl"))
	}

	rendered := make([]File, 0, len(files)+1)
	v.Layout = treeView(files, v)
	for _, f := range files {
		content, err := render(f.Content, v)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", f.Path, err)
		}
		rendered = append(rendered, File{Path: f.Path, Content: content, Mode: fileMode(f.Path)})
	}

	if spec.LicenseID != "" {
		text, err := renderLicense(spec.LicenseID, v)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, File{Path: "LICENSE", Content: text, Mode: 0o644})
	}

	sort.Slice(rendered, func(i, j int) bool { return rendered[i].Path < rendered[j].Path })
	return rendered, nil
}

// tmplFile loads an embedded template and pairs it with its output path. The
// template body is stored in Content until Build renders it.
func tmplFile(path, name string) File {
	raw, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		// Templates are embedded at build time, so a miss is a programming
		// error rather than something a user can trigger.
		panic("scaffold: missing embedded template " + name)
	}
	return File{Path: path, Content: string(raw)}
}

// render expands a template body with the project view.
func render(body string, v view) (string, error) {
	tmpl, err := template.New("file").Parse(body)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// fileMode picks the permissions for a generated file.
func fileMode(path string) os.FileMode {
	if strings.HasSuffix(path, ".sh") {
		return 0o755
	}
	return 0o644
}

// goModContent builds a go.mod without invoking the toolchain, so --dry-run
// can show the exact result.
func goModContent(module, goVersion string) string {
	return fmt.Sprintf("module %s\n\ngo %s\n", module, goVersion)
}

// treeView renders the file list as a tree for the README.
func treeView(files []File, v view) string {
	paths := make([]string, 0, len(files)+1)
	for _, f := range files {
		paths = append(paths, filepath.ToSlash(f.Path))
	}
	if v.HasLicense {
		paths = append(paths, "LICENSE")
	}
	return RenderTree(v.Name, paths)
}

// Write materializes the files under dir. Existing files are only replaced
// when overwrite is set.
func Write(dir string, files []File, overwrite bool) error {
	for _, f := range files {
		target := filepath.Join(dir, f.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
		}
		if !overwrite {
			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("%s already exists (use --force to overwrite)", f.Path)
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("stat %s: %w", target, err)
			}
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(target, []byte(f.Content), mode); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return nil
}

// PackageName turns a project name into a valid Go package identifier.
func PackageName(name string) string {
	name = strings.ToLower(filepath.Base(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			// Separators are dropped: "my-cool.tool" becomes "mycooltool".
		}
	}
	out := b.String()
	if out == "" {
		return "project"
	}
	if unicode.IsDigit(rune(out[0])) {
		out = "p" + out
	}
	return out
}

// BinaryName derives the command name from the project name.
func BinaryName(name string) string {
	base := filepath.Base(strings.TrimSuffix(name, "/"))
	base = strings.TrimSuffix(base, ".git")
	if base == "" || base == "." {
		return "app"
	}
	return base
}

// DefaultGoVersion reports the language version to write into go.mod, taken
// from the toolchain that built mkgo.
func DefaultGoVersion() string {
	v := strings.TrimPrefix(runtimeVersion(), "go")
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	if v == "" {
		return "1.22"
	}
	return v
}
