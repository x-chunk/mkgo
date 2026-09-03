package ui

import "testing"

func TestTextWidth(t *testing.T) {
	cases := map[string]int{
		"":                   0,
		"abc":                3,
		"✅":                  2,
		"🚀":                  2,
		"⚠️":                 2,
		"a✅b":                4,
		"\033[31mred\033[0m": 3,
	}
	for in, want := range cases {
		if got := TextWidth(in); got != want {
			t.Errorf("TextWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestStripANSI(t *testing.T) {
	if got := StripANSI("\033[1;36mmkgo\033[0m"); got != "mkgo" {
		t.Errorf("StripANSI = %q, want mkgo", got)
	}
}

func TestPadRightUsesDisplayWidth(t *testing.T) {
	if got := TextWidth(PadRight("✅", 5)); got != 5 {
		t.Errorf("padded width = %d, want 5", got)
	}
	if got := PadRight("toolong", 3); got != "toolong" {
		t.Errorf("PadRight must not truncate: %q", got)
	}
}

func TestStyleDisabledIsIdentity(t *testing.T) {
	s := newStyle(false, ansiRed)
	if got := s.Apply("text"); got != "text" {
		t.Errorf("disabled style changed the text: %q", got)
	}
	enabled := newStyle(true, ansiRed)
	if got := enabled.Apply("text"); got != "\033[31mtext\033[0m" {
		t.Errorf("enabled style = %q", got)
	}
}

func TestColorSupportedHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if ColorSupported(nil) {
		t.Error("NO_COLOR must disable colors")
	}
}

func TestIconFallback(t *testing.T) {
	if got := glyph(IconSuccess, true); got != "✅" {
		t.Errorf("emoji glyph = %q", got)
	}
	if got := glyph(IconSuccess, false); got != "OK" {
		t.Errorf("ascii glyph = %q", got)
	}
}
