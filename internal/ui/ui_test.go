package ui

import (
	"bytes"
	"strings"
	"testing"
)

func newTestPrinter(color, emoji bool) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	p := New(Options{Out: &out, Err: &errOut, In: strings.NewReader(""), Color: color, Emoji: emoji})
	return p, &out, &errOut
}

func TestPrinterWithoutColorEmitsNoEscapes(t *testing.T) {
	p, out, _ := newTestPrinter(false, false)
	p.Banner("1.0.0")
	p.Success("done")
	p.Info("note")
	p.Section(IconFolder, "files")
	if strings.ContainsRune(out.String(), '\033') {
		t.Errorf("escape sequence in plain output: %q", out.String())
	}
}

func TestPrinterWithColorEmitsEscapes(t *testing.T) {
	p, out, _ := newTestPrinter(true, true)
	p.Section(IconFolder, "files")
	if !strings.ContainsRune(out.String(), '\033') {
		t.Errorf("expected ANSI codes, got %q", out.String())
	}
}

func TestPrinterWithoutEmojiUsesASCII(t *testing.T) {
	p, out, _ := newTestPrinter(false, false)
	p.Success("done")
	if strings.ContainsRune(out.String(), '✅') {
		t.Errorf("emoji leaked into ASCII output: %q", out.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("expected the ASCII marker, got %q", out.String())
	}
}

func TestQuietSuppressesProgressButNotErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	p := New(Options{Out: &out, Err: &errOut, In: strings.NewReader(""), Quiet: true})
	p.Success("done")
	p.Info("note")
	p.Line("raw")
	p.StartStep("working").Done("finished")
	p.Error("broken")
	p.Warn("careful")

	if out.Len() != 0 {
		t.Errorf("quiet printer wrote to stdout: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "broken") || !strings.Contains(errOut.String(), "careful") {
		t.Errorf("errors and warnings must survive quiet mode: %q", errOut.String())
	}
}

func TestVerboseGatesDebug(t *testing.T) {
	p, out, _ := newTestPrinter(false, false)
	p.Debug("hidden")
	if out.Len() != 0 {
		t.Errorf("debug output leaked: %q", out.String())
	}

	var buf bytes.Buffer
	verbose := New(Options{Out: &buf, Err: &buf, In: strings.NewReader(""), Verbose: true})
	verbose.Debug("shown")
	if !strings.Contains(buf.String(), "shown") {
		t.Errorf("verbose debug missing: %q", buf.String())
	}
}

func TestStepStates(t *testing.T) {
	p, out, errOut := newTestPrinter(false, false)
	p.StartStep("a").Done("done")
	p.StartStep("b").Skip("skipped")
	p.StartStep("c").Fail("failed")
	p.StartStep("d").Warn("warned")

	got := out.String() + errOut.String()
	for _, want := range []string{"done", "skipped", "failed", "warned"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output: %q", want, got)
		}
	}
}

func TestStepFinishesOnlyOnce(t *testing.T) {
	p, out, _ := newTestPrinter(false, false)
	step := p.StartStep("a")
	step.Done("first")
	step.Done("second")
	if strings.Count(out.String(), "first") != 1 || strings.Contains(out.String(), "second") {
		t.Errorf("a step must print exactly one result: %q", out.String())
	}
}

func TestBoxAlignsRows(t *testing.T) {
	p, out, _ := newTestPrinter(false, false)
	p.Box("plan", []string{"short", "a much longer row"})

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), out.String())
	}
	width := TextWidth(lines[0])
	for i, line := range lines {
		if got := TextWidth(line); got != width {
			t.Errorf("line %d has width %d, want %d: %q", i, got, width, line)
		}
	}
}

func TestBoxFitsALongTitle(t *testing.T) {
	p, out, _ := newTestPrinter(false, true)
	p.Box("✨ a considerably long title", []string{"x"})

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	width := TextWidth(lines[0])
	for i, line := range lines {
		if got := TextWidth(line); got != width {
			t.Errorf("line %d has width %d, want %d: %q", i, got, width, line)
		}
	}
}

func TestConfirmDefaultsWithoutTerminal(t *testing.T) {
	p, _, _ := newTestPrinter(false, false)
	for _, def := range []bool{true, false} {
		got, err := p.Confirm("proceed?", def)
		if err != nil {
			t.Fatalf("Confirm returned an error: %v", err)
		}
		if got != def {
			t.Errorf("Confirm() = %v, want the default %v", got, def)
		}
	}
}
