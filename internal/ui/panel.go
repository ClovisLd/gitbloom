package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// fitBody pads or truncates a block to exactly w columns and h rows, filling
// padding with the theme background so the panel is fully opaque.
func fitBody(s string, w, h int, pad lipgloss.Style) string {
	if h <= 0 {
		return ""
	}
	s = expandTabs(s)
	lines := strings.Split(s, "\n")
	out := make([]string, h)
	for i := 0; i < h; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out[i] = padRightStyled(l, w, pad)
	}
	return strings.Join(out, "\n")
}

// box draws a bordered panel with a plain title row.
func box(t Theme, title, body string, w, h int, focused bool) string {
	titleLine := t.accent().Render(padRight(" "+title, w-2))
	return boxRaw(t, titleLine, body, w, h, focused)
}

// boxRaw is box with a pre-rendered title line (used for tab bars).
func boxRaw(t Theme, titleLine, body string, w, h int, focused bool) string {
	if w < 2 {
		w = 2
	}
	if h < 2 {
		h = 2
	}
	cw, ch := w-2, h-2

	var content string
	if ch <= 0 {
		content = titleLine
	} else {
		content = titleLine + "\n" + fitBody(body, cw, ch-1, t.base())
	}

	borderColor := t.Border
	if focused {
		borderColor = t.Accent
	}
	border := lipgloss.RoundedBorder()
	if focused {
		border = lipgloss.DoubleBorder()
	}
	bs := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Foreground(t.Fg).
		Background(t.Bg)
	return bs.Render(content)
}
