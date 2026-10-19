package ui

import "charm.land/bubbles/v2/key"

// KeyMap is the keyboard contract. The help bar is generated from it.
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Select key.Binding
	Cancel key.Binding

	Undo   key.Binding
	Flip   key.Binding
	Theme  key.Binding
	Save   key.Binding
	Open   key.Binding
	New    key.Binding
	Resign key.Binding
	Draw   key.Binding
	Focus  key.Binding
	Help   key.Binding
	Quit   key.Binding

	// over swaps the short help to the game-over actions.
	over bool
}

func newKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("enter", "select"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Undo: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "undo"),
		),
		Flip: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "flip"),
		),
		Theme: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "theme"),
		),
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new game"),
		),
		Resign: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "resign"),
		),
		Draw: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "draw"),
		),
		Focus: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "history"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp is the always-visible hint bar.
func (k KeyMap) ShortHelp() []key.Binding {
	if k.over {
		return []key.Binding{k.New, k.Save, k.Help, k.Quit}
	}
	return []key.Binding{k.Select, k.Cancel, k.Undo, k.Resign, k.Flip, k.Focus, k.Help, k.Quit}
}

// FullHelp is the list toggled with ?.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Select, k.Cancel},
		{k.Undo, k.Flip, k.Theme, k.Save, k.Open},
		{k.New, k.Resign, k.Draw, k.Focus, k.Help, k.Quit},
	}
}
