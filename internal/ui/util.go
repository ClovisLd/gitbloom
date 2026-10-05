package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// sanitize neutralises control characters and ANSI escape sequences in text
// that originates from the repository (diffs, file contents, paths, commit
// subjects, author names). Without this a crafted repository could smuggle
// escape sequences through git output and into the terminal — moving the
// cursor, changing the window title, spoofing the UI or writing to the
// clipboard via OSC 52. Newlines are preserved so multi-line diffs still work;
// every other control character becomes '?'.
func sanitize(s string) string {
	if s == "" {
		return s
	}
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r < 0x20 || r == 0x7f:
			return '?'
		default:
			return r
		}
	}, s)
}

// truncate cuts s to at most w display columns, ANSI-unaware but rune-aware.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	cur := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if cur+rw > w {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	return b.String()
}

// padRight pads s with spaces to exactly w display columns.
func padRight(s string, w int) string {
	s = truncate(s, w)
	gap := w - lipgloss.Width(s)
	if gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

func expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.ReplaceAll(l, "\t", "    ")
	}
	return strings.Join(lines, "\n")
}

// padRightStyled pads s to exactly w columns, filling with a styled pad so the
// padding inherits a background (inner ANSI resets would otherwise leave gaps).
func padRightStyled(s string, w int, pad lipgloss.Style) string {
	s = truncate(s, w)
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + pad.Render(strings.Repeat(" ", gap))
}

// lineOf places left and right text with padding between to fill w columns.
func lineOf(left, right string, w int) string {
	rw := lipgloss.Width(right)
	if w-rw-1 < 1 {
		return truncate(left+" "+right, w)
	}
	left = truncate(left, w-rw-1)
	gap := w - lipgloss.Width(left) - rw
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
