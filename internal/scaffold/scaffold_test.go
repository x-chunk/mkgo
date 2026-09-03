package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testSpec(layout Layout) Spec {
	return Spec{
		Name:          "demo-tool",
		Module:        "github.com/example/demo-tool",
		Description:   "A demo.",
		Author:        "Jane Doe",
		Year:          2024,
		LicenseID:     "mit",
		GoVersion:     "1.22",
		Branch:        "main",
		Layout:        layout,
		WithMod:       true,
		WithReadme:    true,
		WithMakefile:  true,
		WithCI:        true,
		WithGitignore: true,
		WithEditorCfg: true,
	}
}

func pathsOf(files []File) map[string]string {
	out := make(map[string]string, len(files))
	for _, f := range files {
		out[filepath.ToSlash(f.Path)] = f.Content
	}
	return out
}

func TestBuildLayouts(t *testing.T) {
	want := map[Layout][]string{
		LayoutCLI:     {"main.go", "internal/app/app.go", "internal/app/app_test.go"},
		LayoutAPI:     {"main.go", "internal/server/server.go", "internal/server/server_test.go"},
		LayoutLib:     {"demotool.go", "demotool_test.go"},
		LayoutMinimal: {"main.go"},
	}

	for layout, sources := range want {
		t.Run(string(layout), func(t *testing.T) {
			files, err := Build(testSpec(layout))
			if err != nil {
				t.Fatalf("Build returned an error: %v", err)
			}
			got := pathsOf(files)
			for _, p := range append(sources, "go.mod", "README.md", "Makefile", "LICENSE", ".gitignore", ".editorconfig", ".github/workflows/ci.yml") {
				if _, ok := got[p]; !ok {
					t.Errorf("missing generated file %s", p)
				}
			}
			if layout == LayoutLib {
				if _, ok := got["main.go"]; ok {
					t.Error("a library layout must not contain main.go")
				}
			}
		})
	}
}

func TestBuildRespectsDisabledFiles(t *testing.T) {
	spec := testSpec(LayoutMinimal)
	spec.WithMod = false
	spec.WithReadme = false
	spec.WithMakefile = false
	spec.WithCI = false
	spec.WithGitignore = false
	spec.WithEditorCfg = false
	spec.LicenseID = ""

	files, err := Build(spec)
	if err != nil {
		t.Fatalf("Build returned an error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected only main.go, got %d files", len(files))
	}
	if files[0].Path != "main.go" {
		t.Errorf("unexpected file %s", files[0].Path)
	}
}

func TestGoModContainsModuleAndVersion(t *testing.T) {
	files, err := Build(testSpec(LayoutCLI))
	if err != nil {
		t.Fatalf("Build returned an error: %v", err)
	}
	content := pathsOf(files)["go.mod"]
	if !strings.Contains(content, "module github.com/example/demo-tool") {
		t.Errorf("go.mod is missing the module path:\n%s", content)
	}
	if !strings.Contains(content, "go 1.22") {
		t.Errorf("go.mod is missing the go directive:\n%s", content)
	}
}

func TestLicenseCarriesAuthorAndYear(t *testing.T) {
	files, err := Build(testSpec(LayoutCLI))
	if err != nil {
		t.Fatalf("Build returned an error: %v", err)
	}
	license := pathsOf(files)["LICENSE"]
	if !strings.Contains(license, "2024 Jane Doe") {
		t.Errorf("LICENSE is missing the copyright line:\n%s", license)
	}
}

func TestTemplatesRenderWithoutLeftovers(t *testing.T) {
	for _, layout := range Layouts() {
		l, err := ParseLayout(layout)
		if err != nil {
			t.Fatalf("ParseLayout(%q): %v", layout, err)
		}
		files, err := Build(testSpec(l))
		if err != nil {
			t.Fatalf("Build(%s): %v", layout, err)
		}
		for _, f := range files {
			if strings.Contains(f.Content, "{{") || strings.Contains(f.Content, "<no value>") {
				t.Errorf("%s/%s still contains template markers", layout, f.Path)
			}
		}
	}
}

func TestWriteRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	files := []File{{Path: "main.go", Content: "package main\n"}}

	if err := Write(dir, files, false); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if err := Write(dir, files, false); err == nil {
		t.Fatal("expected the second write to fail without overwrite")
	}
	if err := Write(dir, files, true); err != nil {
		t.Fatalf("overwrite failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "package main\n" {
		t.Errorf("unexpected content %q", string(data))
	}
}

func TestWriteCreatesNestedDirectories(t *testing.T) {
	dir := t.TempDir()
	files := []File{{Path: filepath.Join(".github", "workflows", "ci.yml"), Content: "name: CI\n"}}
	if err := Write(dir, files, false); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "ci.yml")); err != nil {
		t.Fatalf("nested file was not created: %v", err)
	}
}

func TestPackageName(t *testing.T) {
	cases := map[string]string{
		"demo":          "demo",
		"my-cool.tool":  "mycooltool",
		"Some_Project":  "someproject",
		"9lives":        "p9lives",
		"path/to/thing": "thing",
		"---":           "project",
	}
	for in, want := range cases {
		if got := PackageName(in); got != want {
			t.Errorf("PackageName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBinaryName(t *testing.T) {
	cases := map[string]string{
		"demo":         "demo",
		"path/to/tool": "tool",
		"tool.git":     "tool",
	}
	for in, want := range cases {
		if got := BinaryName(in); got != want {
			t.Errorf("BinaryName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeLicense(t *testing.T) {
	cases := map[string]string{
		"MIT":        "mit",
		"apache":     "apache-2.0",
		"apache-2.0": "apache-2.0",
		"bsd3":       "bsd-3-clause",
		"none":       "",
		"":           "",
	}
	for in, want := range cases {
		got, err := NormalizeLicense(in)
		if err != nil {
			t.Fatalf("NormalizeLicense(%q) returned an error: %v", in, err)
		}
		if got != want {
			t.Errorf("NormalizeLicense(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := NormalizeLicense("wtfpl"); err == nil {
		t.Error("expected an error for an unknown license")
	}
}

func TestEveryLicenseRenders(t *testing.T) {
	for _, id := range AvailableLicenses() {
		text, err := renderLicense(id, view{Year: 2024, Author: "Jane Doe"})
		if err != nil {
			t.Fatalf("renderLicense(%q): %v", id, err)
		}
		if len(text) < 100 {
			t.Errorf("license %q looks truncated (%d bytes)", id, len(text))
		}
	}
}

func TestParseLayoutAliases(t *testing.T) {
	cases := map[string]Layout{
		"cli":     LayoutCLI,
		"library": LayoutLib,
		"http":    LayoutAPI,
		"bare":    LayoutMinimal,
		"API":     LayoutAPI,
	}
	for in, want := range cases {
		got, err := ParseLayout(in)
		if err != nil {
			t.Fatalf("ParseLayout(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseLayout(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ParseLayout("rust"); err == nil {
		t.Error("expected an error for an unknown layout")
	}
}

func TestRenderTree(t *testing.T) {
	got := RenderTree("demo", []string{"main.go", "internal/app/app.go", "internal/app/app_test.go", "README.md"})
	want := strings.Join([]string{
		"demo/",
		"├── internal/",
		"│   └── app/",
		"│       ├── app.go",
		"│       └── app_test.go",
		"├── README.md",
		"└── main.go",
	}, "\n")
	if got != want {
		t.Errorf("unexpected tree:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDefaultGoVersion(t *testing.T) {
	original := runtimeVersion
	t.Cleanup(func() { runtimeVersion = original })

	runtimeVersion = func() string { return "go1.26.0" }
	if got := DefaultGoVersion(); got != "1.26" {
		t.Errorf("DefaultGoVersion() = %q, want 1.26", got)
	}
	runtimeVersion = func() string { return "go1.22rc1" }
	if got := DefaultGoVersion(); got != "1.22rc1" {
		t.Errorf("DefaultGoVersion() = %q, want 1.22rc1", got)
	}
}
