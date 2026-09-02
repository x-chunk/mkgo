package cli

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func mustParse(t *testing.T, args ...string) *Options {
	t.Helper()
	opts, err := Parse(args, io.Discard, "test")
	if err != nil {
		t.Fatalf("Parse(%v) returned an error: %v", args, err)
	}
	return opts
}

func TestParsePositionalBeforeFlags(t *testing.T) {
	opts := mustParse(t, "demo", "--private", "--no-gh")
	if opts.Name != "demo" {
		t.Errorf("name = %q", opts.Name)
	}
	if !opts.Private || !opts.NoGitHub {
		t.Errorf("flags after the name were not parsed: %+v", opts)
	}
}

func TestParsePositionalAfterFlags(t *testing.T) {
	opts := mustParse(t, "--private", "--layout", "api", "demo")
	if opts.Name != "demo" || opts.Layout != "api" || !opts.Private {
		t.Errorf("unexpected options: %+v", opts)
	}
}

func TestParsePositionalBetweenFlags(t *testing.T) {
	opts := mustParse(t, "--no-color", "demo", "--license", "isc")
	if opts.Name != "demo" || opts.License != "isc" || !opts.NoColor {
		t.Errorf("unexpected options: %+v", opts)
	}
}

func TestParseAttachedValues(t *testing.T) {
	opts := mustParse(t, "--module=github.com/example/demo", "demo")
	if opts.Module != "github.com/example/demo" {
		t.Errorf("module = %q", opts.Module)
	}
}

func TestParseShorthands(t *testing.T) {
	opts := mustParse(t, "-t", "lib", "-m", "example.com/x", "-y", "-q", "demo")
	if opts.Layout != "lib" || opts.Module != "example.com/x" || !opts.Yes || !opts.Quiet {
		t.Errorf("unexpected options: %+v", opts)
	}
	if !opts.Changed("layout") || !opts.Changed("yes") {
		t.Error("shorthand flags should mark their canonical name as changed")
	}
}

func TestParseDoubleDashStopsFlagParsing(t *testing.T) {
	opts := mustParse(t, "--no-gh", "--", "-weird-name")
	if opts.Name != "-weird-name" {
		t.Errorf("name = %q", opts.Name)
	}
}

func TestParseUnknownFlag(t *testing.T) {
	_, err := Parse([]string{"demo", "--nope"}, io.Discard, "test")
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected an unknown flag error, got %v", err)
	}
}

func TestParseMissingFlagValue(t *testing.T) {
	_, err := Parse([]string{"demo", "--module"}, io.Discard, "test")
	if err == nil || !strings.Contains(err.Error(), "needs a value") {
		t.Fatalf("expected a missing value error, got %v", err)
	}
}

func TestParseTooManyPositionalArguments(t *testing.T) {
	_, err := Parse([]string{"one", "two"}, io.Discard, "test")
	if err == nil || !strings.Contains(err.Error(), "single project name") {
		t.Fatalf("expected a single-name error, got %v", err)
	}
}

func TestParseHelpAndVersion(t *testing.T) {
	var out strings.Builder
	if _, err := Parse([]string{"--help"}, &out, "1.0.0"); !errors.Is(err, ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
	if !strings.Contains(out.String(), "USAGE") {
		t.Error("help output is missing the usage section")
	}

	out.Reset()
	if _, err := Parse([]string{"--version"}, &out, "1.0.0"); !errors.Is(err, ErrVersion) {
		t.Fatalf("expected ErrVersion, got %v", err)
	}
	if !strings.Contains(out.String(), "1.0.0") {
		t.Errorf("version output = %q", out.String())
	}
}

func TestParseSkipFlags(t *testing.T) {
	opts := mustParse(t, "demo", "--no-git", "--no-gh", "--no-mod", "--no-color", "--no-emoji")
	if !opts.NoGit || !opts.NoGitHub || !opts.NoMod || !opts.NoColor || !opts.NoEmoji {
		t.Errorf("skip flags were not all set: %+v", opts)
	}
}
