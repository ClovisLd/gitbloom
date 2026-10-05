package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme is the terminal palette. The default mirrors OpenCode's dark theme:
// full black, near-white text, and a purple accent.
type Theme struct {
	Name      string
	Bg        lipgloss.Color
	Panel     lipgloss.Color
	Fg        lipgloss.Color
	Dim       lipgloss.Color
	Accent    lipgloss.Color
	Good      lipgloss.Color
	Bad       lipgloss.Color
	Warn      lipgloss.Color
	Cyan      lipgloss.Color
	Border    lipgloss.Color
	SelBg     lipgloss.Color
	SelFg     lipgloss.Color
	DiffAddBg lipgloss.Color
	DiffDelBg lipgloss.Color
}

func lipglossFg(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

// OpenCodeTheme is the default: full black, white text, purple accent.
func OpenCodeTheme() Theme {
	return Theme{
		Name:      "opencode",
		Bg:        "#000000",
		Panel:     "#000000",
		Fg:        "#EEEEEE",
		Dim:       "#808080",
		Accent:    "#9D7CD8", // OpenCode accent (purple)
		Good:      "#7FD88F",
		Bad:       "#E06C75",
		Warn:      "#E5C07B",
		Cyan:      "#56B6C2",
		Border:    "#3C3C3C",
		SelBg:     "#EEEEEE",
		SelFg:     "#000000",
		DiffAddBg: "#0E1F13",
		DiffDelBg: "#2A1215",
	}
}

func (t Theme) base() lipgloss.Style {
	return lipgloss.NewStyle().Background(t.Bg)
}

func (t Theme) fg() lipgloss.Style {
	return t.base().Foreground(t.Fg)
}
func (t Theme) dim() lipgloss.Style {
	return t.base().Foreground(t.Dim)
}

// head marks the HEAD (last) commit row in the history: bold so it stays
// visible when it is not the selected row.
func (t Theme) head() lipgloss.Style {
	return t.fg().Bold(true)
}
func (t Theme) accent() lipgloss.Style {
	return t.base().Foreground(t.Accent).Bold(true)
}
func (t Theme) good() lipgloss.Style {
	return t.base().Foreground(t.Good)
}
func (t Theme) bad() lipgloss.Style {
	return t.base().Foreground(t.Bad)
}
func (t Theme) warn() lipgloss.Style {
	return t.base().Foreground(t.Warn)
}
func (t Theme) cyan() lipgloss.Style {
	return t.base().Foreground(t.Cyan)
}

// sel is reverse video so the selection is visible on any terminal: with
// white-on-black it renders as a white bar with black text.
func (t Theme) sel() lipgloss.Style {
	return lipgloss.NewStyle().Reverse(true).Bold(true)
}

// brand is the header wordmark: accent text on black.
func (t Theme) brand() lipgloss.Style {
	return t.base().Foreground(t.Accent).Bold(true)
}

// footer is the function-key strip: muted text on black.
func (t Theme) footer() lipgloss.Style {
	return t.base().Foreground(t.Dim)
}

// addStyle: green text on a dark green wash.
func (t Theme) addStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Good).Background(t.DiffAddBg)
}

// delStyle: red text on a dark red wash.
func (t Theme) delStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Bad).Background(t.DiffDelBg)
}

// statusStyle colours a commit-file status letter.
func (t Theme) statusStyle(st byte) lipgloss.Style {
	switch st {
	case 'A':
		return t.good()
	case 'D':
		return t.bad()
	case 'R', 'C':
		return t.cyan()
	case 'M', 'T':
		return t.warn()
	default:
		return t.fg()
	}
}
