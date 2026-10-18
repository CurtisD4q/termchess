// Package theme defines the four board palettes.
//
// Colors are chosen with lipgloss.LightDark so a light terminal and a dark
// terminal each get a readable pair. Lip Gloss downsamples them when the
// terminal has no true color. The high-contrast theme avoids backgrounds
// entirely and relies on reverse, bold, and brackets.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// ID identifies a palette.
type ID string

const (
	Wood  ID = "wood"
	Green ID = "green"
	Blue  ID = "blue"
	Mono  ID = "mono"
)

// All returns the palettes in cycle order.
func All() []ID { return []ID{Wood, Green, Blue, Mono} }

// Parse falls back to classic wood.
func Parse(s string) ID {
	switch ID(s) {
	case Wood, Green, Blue, Mono:
		return ID(s)
	default:
		return Wood
	}
}

// Next cycles t.
func Next(id ID) ID {
	all := All()
	for i, c := range all {
		if c == id {
			return all[(i+1)%len(all)]
		}
	}
	return Wood
}

// Styles is every highlight the board and chrome need.
type Styles struct {
	Name  string
	ID    ID
	Mono  bool
	Light lipgloss.Style
	Dark  lipgloss.Style

	Cursor   lipgloss.Style
	Selected lipgloss.Style
	Target   lipgloss.Style
	Capture  lipgloss.Style
	LastFrom lipgloss.Style
	LastTo   lipgloss.Style
	Check    lipgloss.Style
	CheckHot lipgloss.Style
	Premove  lipgloss.Style
	Illegal  lipgloss.Style

	Text     lipgloss.Style
	Muted    lipgloss.Style
	Accent   lipgloss.Style
	Border   lipgloss.Style
	Panel    lipgloss.Style
	Status   lipgloss.Style
	Title    lipgloss.Style
	ClockOn  lipgloss.Style
	ClockOff lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	// Frame is the dark border behind the file letters and rank numbers.
	Frame lipgloss.Style

	// WhitePiece and BlackPiece are the chips behind solid piece glyphs.
	// The glyph itself is a dark ink on the white chip and a light ink on
	// the black chip, so both sides stay readable on light and dark squares.
	WhitePiece color.Color
	BlackPiece color.Color
}

// For builds a palette. dark follows tea.BackgroundColorMsg.
func For(id ID, dark bool) Styles {
	if id == Mono {
		return mono()
	}
	switch id {
	case Green:
		return paint("Green tournament", Green, spec{
			light: "#7F9A62", dark: "#3E5C3A",
			lightFg: "#1C2416", darkFg: "#10200C",
			cursor: "#F5F682", cursorFg: "#1A1A1A",
			selected: "#BBCC44", selectedFg: "#1A1A1A",
			target: "#F6F669", capture: "#E07A5F",
			lastFrom: "#F3F5C0", lastTo: "#C8D68A",
			check: "#E07070", checkHot: "#FFB4B4",
			premove: "#C58BB9", illegal: "#FF5C5C",
			text: "#E7F0D8", muted: "#A8B89A", accent: "#C6E07A",
			border: "#8FBC8F", panel: "#1E2A1C",
		}, dark)
	case Blue:
		return paint("Blue", Blue, spec{
			light: "#7E98A8", dark: "#3E5A68",
			lightFg: "#102028", darkFg: "#0C1820",
			cursor: "#F2E394", cursorFg: "#1A1A1A",
			selected: "#8EC6E8", selectedFg: "#102028",
			target: "#F7E7A1", capture: "#E08B7A",
			lastFrom: "#D5E4EE", lastTo: "#A9C6D6",
			check: "#E07070", checkHot: "#FFC1C1",
			premove: "#C9A6E0", illegal: "#FF5C5C",
			text: "#E5F1F8", muted: "#9BB0BE", accent: "#8ECAE6",
			border: "#5C7C8A", panel: "#142028",
		}, dark)
	default:
		return paint("Classic wood", Wood, spec{
			light: "#B08A64", dark: "#6B4F3A",
			lightFg: "#2B1D12", darkFg: "#2B1D12",
			cursor: "#F2F27A", cursorFg: "#2B1D12",
			selected: "#CDD26A", selectedFg: "#2B1D12",
			target: "#F3E38A", capture: "#E07070",
			lastFrom: "#F6E7B2", lastTo: "#D6C07A",
			check: "#E07070", checkHot: "#FFB0A8",
			premove: "#D7A0C4", illegal: "#FF5A5A",
			text: "#F6EFE4", muted: "#C3B39A", accent: "#E6C27A",
			border: "#8C6239", panel: "#2A2118",
		}, dark)
	}
}

type spec struct {
	light, dark          string
	lightFg, darkFg      string
	cursor, cursorFg     string
	selected, selectedFg string
	target, capture      string
	lastFrom, lastTo     string
	check, checkHot      string
	premove, illegal     string
	text, muted, accent  string
	border, panel        string
}

func paint(name string, id ID, s spec, dark bool) Styles {
	fgOn := func(bg, fg string) lipgloss.Style {
		return lipgloss.NewStyle().Background(c(bg)).Foreground(c(fg)).Bold(true)
	}
	// Light terminal: pull the panel and text toward ink on paper.
	text := s.text
	muted := s.muted
	panel := s.panel
	border := s.border
	accent := s.accent
	if !dark {
		text = "#24180F"
		muted = "#5C5146"
		panel = "#F7F1E6"
		border = "#8C6239"
		accent = "#6B4A1B"
	}
	baseFgLight := s.lightFg
	baseFgDark := s.darkFg
	// LightDark picks the ink that stays readable on a light or dark terminal.
	// Square hues themselves stay put so the board still looks like wood or felt.
	ld := lipgloss.LightDark(dark)
	ink := ld(lipgloss.Color("#24180F"), lipgloss.Color(text))
	soft := ld(lipgloss.Color("#5C5146"), lipgloss.Color(muted))
	// The wood frame stays the dark brown from the reference, on a light
	// terminal as well. Other palettes use their panel.
	frameBG, frameFG := s.panel, muted
	if id == Wood {
		frameBG, frameFG = "#2A2018", "#C3B39A"
	}
	return Styles{
		Name:     name,
		ID:       id,
		Light:    lipgloss.NewStyle().Background(c(s.light)).Foreground(c(baseFgLight)),
		Dark:     lipgloss.NewStyle().Background(c(s.dark)).Foreground(c(baseFgDark)),
		Cursor:   fgOn(s.cursor, s.cursorFg),
		Selected: fgOn(s.selected, s.selectedFg),
		Target:   fgOn(s.target, "#1A1A1A"),
		Capture:  fgOn(s.capture, "#1A1A1A"),
		LastFrom: fgOn(s.lastFrom, "#1A1A1A"),
		LastTo:   fgOn(s.lastTo, "#1A1A1A"),
		Check:    fgOn(s.check, "#1A1A1A"),
		CheckHot: fgOn(s.checkHot, "#1A1A1A"),
		Premove:  fgOn(s.premove, "#1A1A1A"),
		Illegal:  fgOn(s.illegal, "#1A1A1A"),
		Text:     lipgloss.NewStyle().Foreground(ink),
		Muted:    lipgloss.NewStyle().Foreground(soft),
		Accent:   lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#6B4A1B"), c(accent))).Bold(true),
		Border:   lipgloss.NewStyle().Foreground(c(border)),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c(border)).
			Background(c(panel)).
			Padding(0, 1),
		Status:     lipgloss.NewStyle().Foreground(c(text)),
		Title:      lipgloss.NewStyle().Foreground(c(accent)).Bold(true),
		ClockOn:    lipgloss.NewStyle().Foreground(c("#1A1A1A")).Background(c(accent)).Bold(true).Padding(0, 1),
		ClockOff:   lipgloss.NewStyle().Foreground(c(text)).Padding(0, 1),
		HelpKey:    lipgloss.NewStyle().Foreground(c(accent)).Bold(true),
		HelpDesc:   lipgloss.NewStyle().Foreground(c(muted)),
		Frame:      lipgloss.NewStyle().Background(c(frameBG)).Foreground(c(frameFG)),
		WhitePiece: c("#FFF8EE"),
		BlackPiece: c("#1A140F"),
	}
}

func mono() Styles {
	plain := lipgloss.NewStyle()
	return Styles{
		Name:       "High contrast",
		ID:         Mono,
		Mono:       true,
		Light:      plain,
		Dark:       plain,
		Cursor:     plain.Reverse(true).Bold(true),
		Selected:   plain.Underline(true).Bold(true),
		Target:     plain.Bold(true),
		Capture:    plain.Underline(true),
		LastFrom:   plain.Italic(true),
		LastTo:     plain.Italic(true).Bold(true),
		Check:      plain.Bold(true).Reverse(true),
		CheckHot:   plain.Bold(true).Reverse(true),
		Premove:    plain.Underline(true).Italic(true),
		Illegal:    plain.Reverse(true),
		Text:       plain,
		Muted:      plain.Faint(true),
		Accent:     plain.Bold(true),
		Border:     plain,
		Panel:      plain.Border(lipgloss.RoundedBorder()).Padding(0, 1),
		Status:     plain,
		Title:      plain.Bold(true).Underline(true),
		ClockOn:    plain.Reverse(true).Bold(true).Padding(0, 1),
		ClockOff:   plain.Padding(0, 1),
		HelpKey:    plain.Bold(true),
		HelpDesc:   plain.Faint(true),
		Frame:      plain,
		WhitePiece: nil,
		BlackPiece: nil,
	}
}

func c(hex string) color.Color { return lipgloss.Color(hex) }
