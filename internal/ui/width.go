package ui

import (
	"strings"
)

// variationSelector16 forces the emoji presentation of the preceding rune,
// which also makes it occupy two terminal cells.
const variationSelector16 = 0xFE0F

// wideRanges lists the code point ranges that terminals render two cells wide.
var wideRanges = [][2]rune{
	{0x1100, 0x115F},   // Hangul Jamo
	{0x2705, 0x2705},   // white heavy check mark
	{0x270A, 0x270B},   // raised fist/hand
	{0x2728, 0x2728},   // sparkles
	{0x274C, 0x274C},   // cross mark
	{0x274E, 0x274E},   // negative squared cross mark
	{0x2753, 0x2755},   // question/exclamation ornaments
	{0x2757, 0x2757},   // heavy exclamation mark
	{0x2795, 0x2797},   // heavy math symbols
	{0x2B1B, 0x2B1C},   // large squares
	{0x2B50, 0x2B50},   // white medium star
	{0x2E80, 0x303E},   // CJK radicals, Kangxi
	{0x3041, 0x33FF},   // kana, CJK compatibility
	{0x3400, 0x4DBF},   // CJK extension A
	{0x4E00, 0x9FFF},   // CJK unified ideographs
	{0xA000, 0xA4CF},   // Yi syllables
	{0xAC00, 0xD7A3},   // Hangul syllables
	{0xF900, 0xFAFF},   // CJK compatibility ideographs
	{0xFE30, 0xFE6F},   // CJK compatibility forms
	{0xFF00, 0xFF60},   // fullwidth forms
	{0xFFE0, 0xFFE6},   // fullwidth signs
	{0x1F300, 0x1F64F}, // misc symbols and pictographs, emoticons
	{0x1F680, 0x1F6FF}, // transport and map symbols
	{0x1F900, 0x1F9FF}, // supplemental symbols and pictographs
	{0x1FA70, 0x1FAFF}, // symbols and pictographs extended-A
}

// zeroWidthRanges lists code points that do not advance the cursor.
var zeroWidthRanges = [][2]rune{
	{0x0300, 0x036F},   // combining diacritical marks
	{0x200B, 0x200F},   // zero width space through RTL mark
	{0xFE00, 0xFE0F},   // variation selectors
	{0xE0100, 0xE01EF}, // variation selectors supplement
}

func inRanges(r rune, ranges [][2]rune) bool {
	for _, rg := range ranges {
		if r >= rg[0] && r <= rg[1] {
			return true
		}
	}
	return false
}

// StripANSI removes escape sequences so a styled string can be measured.
func StripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == 0x1B {
			// Skip until the final byte of the CSI sequence.
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7E) {
					j++
				}
				if j < len(s) {
					j++
				}
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// TextWidth estimates how many terminal cells a string occupies. It is an
// approximation good enough to align boxes that mix ASCII and emoji.
func TextWidth(s string) int {
	plain := StripANSI(s)
	width := 0
	runes := []rune(plain)
	for i, r := range runes {
		switch {
		case r == '\t':
			width += 4
		case inRanges(r, zeroWidthRanges):
			// Nothing: the rune modifies its neighbor instead of advancing.
		case inRanges(r, wideRanges):
			width += 2
		case i+1 < len(runes) && runes[i+1] == variationSelector16:
			width += 2
		case r < 0x20:
			// Control characters are not printable.
		default:
			width++
		}
	}
	return width
}

// PadRight extends s with spaces until it reaches the requested display width.
func PadRight(s string, width int) string {
	pad := width - TextWidth(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}
