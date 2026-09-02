package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Options configures a Printer. The zero value is not useful; build one with
// New instead.
type Options struct {
	Out     io.Writer
	Err     io.Writer
	In      io.Reader
	Color   bool
	Emoji   bool
	Animate bool // run spinners; disabled for pipes, --quiet and --no-color output
	Quiet   bool
	Verbose bool
}

// Printer is the single writer used by the CLI. Every method is safe to call
// while a spinner is running: the spinner line is cleared first so messages
// never interleave with the animation.
type Printer struct {
	C Palette

	out     io.Writer
	err     io.Writer
	in      *bufio.Reader
	inFile  *os.File
	emoji   bool
	animate bool
	quiet   bool
	verbose bool

	mu   sync.Mutex
	spin *spinner
}

// New builds a Printer from the given options, filling in sensible defaults.
func New(opts Options) *Printer {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Err == nil {
		opts.Err = os.Stderr
	}
	if opts.In == nil {
		opts.In = os.Stdin
	}
	p := &Printer{
		C:       newPalette(opts.Color),
		out:     opts.Out,
		err:     opts.Err,
		in:      bufio.NewReader(opts.In),
		emoji:   opts.Emoji,
		animate: opts.Animate && !opts.Quiet,
		quiet:   opts.Quiet,
		verbose: opts.Verbose,
	}
	if f, ok := opts.In.(*os.File); ok {
		p.inFile = f
	}
	frames := spinnerFrames
	if !opts.Emoji {
		frames = asciiSpinnerFrames
	}
	p.spin = newSpinner(opts.Out, frames, p.C.Info)
	return p
}

// Icon renders a named glyph honoring the --no-emoji setting.
func (p *Printer) Icon(i Icon) string { return glyph(i, p.emoji) }

// Emoji reports whether emoji rendering is on.
func (p *Printer) Emoji() bool { return p.emoji }

// Verbose reports whether verbose output was requested.
func (p *Printer) Verbose() bool { return p.verbose }

// Quiet reports whether the printer suppresses progress output.
func (p *Printer) Quiet() bool { return p.quiet }

// write emits a line to stdout, pausing any spinner so the two never mix.
func (p *Printer) write(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.spin.active() {
		p.spin.clear()
	}
	fmt.Fprintln(p.out, line)
}

// Line prints a raw line unless the printer is quiet.
func (p *Printer) Line(format string, args ...any) {
	if p.quiet {
		return
	}
	p.write(fmt.Sprintf(format, args...))
}

// Blank prints an empty separating line.
func (p *Printer) Blank() {
	if p.quiet {
		return
	}
	p.write("")
}

// Banner prints the mkgo header shown at the start of a run.
func (p *Printer) Banner(version string) {
	if p.quiet {
		return
	}
	name := p.C.Title.Apply("mkgo")
	ver := p.C.Muted.Apply(version)
	p.write(fmt.Sprintf("%s %s %s", p.Icon(IconRocket), name, ver))
	p.write(p.C.Muted.Apply("create a ready-to-push Go project in one command"))
	p.Blank()
}

// Section prints a bold heading with a leading icon.
func (p *Printer) Section(i Icon, format string, args ...any) {
	if p.quiet {
		return
	}
	p.write(fmt.Sprintf("%s %s", p.Icon(i), p.C.Bold.Sprintf(format, args...)))
}

// Item prints an indented bullet line.
func (p *Printer) Item(format string, args ...any) {
	if p.quiet {
		return
	}
	p.write(fmt.Sprintf("  %s %s", p.C.Muted.Apply(p.Icon(IconBullet)), fmt.Sprintf(format, args...)))
}

// Success prints a positive result line.
func (p *Printer) Success(format string, args ...any) {
	if p.quiet {
		return
	}
	p.write(fmt.Sprintf("%s %s", p.Icon(IconSuccess), fmt.Sprintf(format, args...)))
}

// Info prints a neutral note.
func (p *Printer) Info(format string, args ...any) {
	if p.quiet {
		return
	}
	p.write(fmt.Sprintf("%s %s", p.C.Info.Apply(p.Icon(IconInfo)), fmt.Sprintf(format, args...)))
}

// Warn prints a warning. Warnings survive --quiet because they report
// something the user did not ask for.
func (p *Printer) Warn(format string, args ...any) {
	p.mu.Lock()
	if p.spin.active() {
		p.spin.clear()
	}
	fmt.Fprintf(p.err, "%s %s\n", p.Icon(IconWarning), p.C.Warn.Sprintf(format, args...))
	p.mu.Unlock()
}

// Error prints an error to stderr.
func (p *Printer) Error(format string, args ...any) {
	p.mu.Lock()
	if p.spin.active() {
		p.spin.clear()
	}
	fmt.Fprintf(p.err, "%s %s\n", p.Icon(IconFailure), p.C.Error.Sprintf(format, args...))
	p.mu.Unlock()
}

// Debug prints a dimmed line that only shows up with --verbose.
func (p *Printer) Debug(format string, args ...any) {
	if !p.verbose || p.quiet {
		return
	}
	p.write("    " + p.C.Muted.Sprintf(format, args...))
}

// Step is a unit of work rendered as a spinner while it runs and as a status
// line once it finishes.
type Step struct {
	p       *Printer
	label   string
	started time.Time
	closed  bool
}

// StartStep begins a step and starts the spinner when animation is enabled.
func (p *Printer) StartStep(format string, args ...any) *Step {
	s := &Step{p: p, label: fmt.Sprintf(format, args...), started: time.Now()}
	if p.quiet {
		return s
	}
	if p.animate {
		p.mu.Lock()
		p.spin.start(s.label)
		p.mu.Unlock()
	}
	return s
}

// Update changes the text shown while the step is running.
func (s *Step) Update(format string, args ...any) {
	s.label = fmt.Sprintf(format, args...)
	if s.p.quiet || !s.p.animate {
		return
	}
	s.p.mu.Lock()
	s.p.spin.update(s.label)
	s.p.mu.Unlock()
}

// finish stops the spinner and prints the terminal status of the step.
func (s *Step) finish(icon Icon, style Style, text string, showElapsed bool) {
	if s.closed {
		return
	}
	s.closed = true
	if s.p.quiet {
		return
	}
	s.p.mu.Lock()
	if s.p.spin.active() {
		s.p.spin.stop()
	}
	s.p.mu.Unlock()

	elapsed := ""
	if showElapsed {
		if d := time.Since(s.started); d >= 150*time.Millisecond {
			elapsed = " " + s.p.C.Muted.Apply(formatDuration(d))
		}
	}
	s.p.write(fmt.Sprintf("%s %s%s", s.p.Icon(icon), style.Apply(text), elapsed))
}

// Done marks the step as successful.
func (s *Step) Done(format string, args ...any) {
	s.finish(IconSuccess, Style{}, fmt.Sprintf(format, args...), true)
}

// Skip marks the step as intentionally skipped.
func (s *Step) Skip(format string, args ...any) {
	s.finish(IconSkip, s.p.C.Muted, fmt.Sprintf(format, args...), false)
}

// Fail marks the step as failed.
func (s *Step) Fail(format string, args ...any) {
	s.finish(IconFailure, s.p.C.Error, fmt.Sprintf(format, args...), false)
}

// Warn marks the step as finished with a caveat.
func (s *Step) Warn(format string, args ...any) {
	s.finish(IconWarning, s.p.C.Warn, fmt.Sprintf(format, args...), false)
}

// StopSpinner clears any running animation; used before the process exits.
func (p *Printer) StopSpinner() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.spin.active() {
		p.spin.stop()
	}
}

// Box prints a rounded frame around the given lines with an optional title.
func (p *Printer) Box(title string, lines []string) {
	if p.quiet {
		return
	}
	width := 0
	for _, l := range lines {
		if w := TextWidth(l); w > width {
			width = w
		}
	}
	width += 2 // one space of padding on each side
	// The title sits inside the top border as "╭─ title ─...─╮".
	if titleWidth := TextWidth(title) + 4; title != "" && titleWidth > width {
		width = titleWidth
	}

	horizontal := strings.Repeat("─", width)
	top := "╭" + horizontal + "╮"
	if title != "" {
		pad := width - TextWidth(title) - 3
		if pad < 0 {
			pad = 0
		}
		top = "╭─ " + title + " " + strings.Repeat("─", pad) + "╮"
	}
	p.write(p.C.Muted.Apply(top))
	for _, l := range lines {
		body := " " + PadRight(l, width-1)
		p.write(p.C.Muted.Apply("│") + body + p.C.Muted.Apply("│"))
	}
	p.write(p.C.Muted.Apply("╰" + horizontal + "╯"))
}

// Confirm asks a yes/no question. It returns def when there is no interactive
// terminal to ask on.
func (p *Printer) Confirm(question string, def bool) (bool, error) {
	if p.inFile == nil || !IsTerminal(p.inFile) {
		return def, nil
	}
	choices := "[y/N]"
	if def {
		choices = "[Y/n]"
	}
	p.mu.Lock()
	if p.spin.active() {
		p.spin.stop()
	}
	fmt.Fprintf(p.out, "%s %s %s ", p.Icon(IconQuestion), question, p.C.Muted.Apply(choices))
	p.mu.Unlock()

	line, err := p.in.ReadString('\n')
	if err != nil && line == "" {
		return def, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return def, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, nil
	}
}

// formatDuration renders a compact elapsed time such as "1.2s" or "340ms".
func formatDuration(d time.Duration) string {
	switch {
	case d >= time.Minute:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	case d >= time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
}
