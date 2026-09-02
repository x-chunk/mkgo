// Package ui renders the terminal output of mkgo: colors, icons, spinners and
// the small layout helpers used to keep every message aligned.
package ui

import (
	"fmt"
	"os"
	"strings"
)

// ANSI escape sequences. They are only ever written when a Printer has colors
// enabled, so the raw constants stay unexported.
const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiDim       = "\033[2m"
	ansiItalic    = "\033[3m"
	ansiUnderline = "\033[4m"

	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiGray    = "\033[90m"
)

// Style paints a string with an ANSI sequence. A zero Style is a no-op, which
// is what --no-color and non-interactive output fall back to.
type Style struct {
	codes   string
	enabled bool
}

func newStyle(enabled bool, codes ...string) Style {
	return Style{codes: strings.Join(codes, ""), enabled: enabled}
}

// Apply wraps s in the style's escape sequences when colors are enabled.
func (s Style) Apply(text string) string {
	if !s.enabled || s.codes == "" || text == "" {
		return text
	}
	return s.codes + text + ansiReset
}

// Sprintf formats and then styles the result.
func (s Style) Sprintf(format string, args ...any) string {
	return s.Apply(fmt.Sprintf(format, args...))
}

// Palette groups every style used by the CLI so call sites read as prose.
type Palette struct {
	Bold      Style
	Dim       Style
	Italic    Style
	Underline Style

	Success Style
	Warn    Style
	Error   Style
	Info    Style
	Accent  Style
	Muted   Style
	Title   Style
	Path    Style
	Command Style
}

func newPalette(enabled bool) Palette {
	return Palette{
		Bold:      newStyle(enabled, ansiBold),
		Dim:       newStyle(enabled, ansiDim),
		Italic:    newStyle(enabled, ansiItalic),
		Underline: newStyle(enabled, ansiUnderline),

		Success: newStyle(enabled, ansiGreen),
		Warn:    newStyle(enabled, ansiYellow),
		Error:   newStyle(enabled, ansiRed),
		Info:    newStyle(enabled, ansiCyan),
		Accent:  newStyle(enabled, ansiMagenta),
		Muted:   newStyle(enabled, ansiGray),
		Title:   newStyle(enabled, ansiBold, ansiCyan),
		Path:    newStyle(enabled, ansiBlue),
		Command: newStyle(enabled, ansiBold, ansiMagenta),
	}
}

// ColorSupported reports whether the given stream can reasonably display ANSI
// colors. It honors the NO_COLOR and FORCE_COLOR conventions from
// https://no-color.org and https://force-color.org before probing the terminal.
func ColorSupported(f *os.File) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if v, ok := os.LookupEnv("FORCE_COLOR"); ok && v != "0" && v != "false" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return IsTerminal(f)
}

// IsTerminal reports whether f is attached to a character device, which is the
// portable way to tell an interactive terminal from a pipe or a file.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
