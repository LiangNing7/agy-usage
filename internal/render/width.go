package render

import (
	"regexp"
	"strings"
	"unicode"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes ANSI escape sequences from s.
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// RuneWidth returns the terminal column width of a rune (0 for combining, 2 for East Asian Wide/Fullwidth, 1 otherwise).
func RuneWidth(r rune) int {
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

// isWideRune covers Unicode East Asian Wide (W) and Fullwidth (F) blocks used in terminal emulators.
func isWideRune(r rune) bool {
	return (r >= 0x1100 && r <= 0x115F) || // Hangul Jamo
		r == 0x2329 || r == 0x232A ||
		(r >= 0x2E80 && r <= 0x303E) || // CJK Radicals, Kangxi, Ideographic, CJK Symbols & Punctuation
		(r >= 0x3040 && r <= 0x3247) || // Hiragana, Katakana, Bopomofo, Hangul Compat, Kanbun, Enclosed
		(r >= 0x3250 && r <= 0x4DBF) || // Enclosed CJK, CJK Compatibility, CJK Unified Ext A
		(r >= 0x4E00 && r <= 0xA4C6) || // CJK Unified Ideographs, Yi
		(r >= 0xA960 && r <= 0xA97C) || // Hangul Jamo Extended-A
		(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0xFE10 && r <= 0xFE19) || // Vertical Forms
		(r >= 0xFE30 && r <= 0xFE6B) || // CJK Compatibility Forms, Small Form Variants
		(r >= 0xFF01 && r <= 0xFF60) || // Fullwidth ASCII & Punctuation
		(r >= 0xFFE0 && r <= 0xFFE6) || // Fullwidth Symbol Variants
		(r >= 0x1F300 && r <= 0x1F64F) || // Misc Symbols & Pictographs, Emoticons
		(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols & Pictographs
		(r >= 0x20000 && r <= 0x3FFFD) // CJK Unified Ideographs Extensions B..I
}

// DispWidth returns the visible terminal column width of s (ignoring ANSI codes).
func DispWidth(s string) int {
	clean := StripANSI(s)
	w := 0
	for _, r := range clean {
		w += RuneWidth(r)
	}
	return w
}

// TruncateDisp truncates a plain string so its display width does not exceed maxW.
func TruncateDisp(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if DispWidth(s) <= maxW {
		return s
	}
	const ell = "…"
	ellW := 1
	target := maxW - ellW
	if target < 0 {
		return ""
	}
	var sb strings.Builder
	cur := 0
	for _, r := range s {
		rw := RuneWidth(r)
		if cur+rw > target {
			break
		}
		sb.WriteRune(r)
		cur += rw
	}
	if maxW >= ellW {
		sb.WriteString(ell)
	}
	return sb.String()
}

// PadRight pads s with trailing spaces to reach width visible columns.
func PadRight(s string, width int) string {
	pad := width - DispWidth(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// PadLeft pads s with leading spaces to reach width visible columns.
func PadLeft(s string, width int) string {
	pad := width - DispWidth(s)
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

// FitCell guarantees exact visible width (truncating plain text if needed).
func FitCell(s string, width int, rightAlign bool) string {
	dw := DispWidth(s)
	if dw > width {
		return PadRight(TruncateDisp(StripANSI(s), width), width)
	}
	if rightAlign {
		return PadLeft(s, width)
	}
	return PadRight(s, width)
}
