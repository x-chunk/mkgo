package ui

// Icon is a symbolic name for a glyph. Every icon has both an emoji form and a
// plain ASCII fallback used when --no-emoji is passed or the output is piped.
type Icon string

// The icon set used across the CLI.
const (
	IconRocket   Icon = "rocket"
	IconFolder   Icon = "folder"
	IconSuccess  Icon = "success"
	IconFailure  Icon = "failure"
	IconSkip     Icon = "skip"
	IconWarning  Icon = "warning"
	IconInfo     Icon = "info"
	IconSparkles Icon = "sparkles"
	IconLock     Icon = "lock"
	IconGlobe    Icon = "globe"
	IconQuestion Icon = "question"
	IconBullet   Icon = "bullet"
	IconArrow    Icon = "arrow"
)

// emojiGlyphs maps an icon to its emoji rendering. Pictographs only, no
// smileys, so the output stays businesslike.
var emojiGlyphs = map[Icon]string{
	IconRocket:   "🚀",
	IconFolder:   "📁",
	IconSuccess:  "✅",
	IconFailure:  "❌",
	IconSkip:     "⏭️",
	IconWarning:  "⚠️",
	IconInfo:     "ℹ️",
	IconSparkles: "✨",
	IconLock:     "🔒",
	IconGlobe:    "🌍",
	IconQuestion: "❓",
	IconBullet:   "•",
	IconArrow:    "➜",
}

// asciiGlyphs holds the fallback rendering for every icon above.
var asciiGlyphs = map[Icon]string{
	IconRocket:   "*",
	IconFolder:   "[d]",
	IconSuccess:  "OK",
	IconFailure:  "FAIL",
	IconSkip:     "SKIP",
	IconWarning:  "!",
	IconInfo:     "i",
	IconSparkles: "*",
	IconLock:     "[private]",
	IconGlobe:    "[public]",
	IconQuestion: "?",
	IconBullet:   "-",
	IconArrow:    ">",
}

// glyph returns the rendering of an icon for the current emoji setting.
func glyph(i Icon, emoji bool) string {
	if emoji {
		if g, ok := emojiGlyphs[i]; ok {
			return g
		}
	}
	if g, ok := asciiGlyphs[i]; ok {
		return g
	}
	return string(i)
}
