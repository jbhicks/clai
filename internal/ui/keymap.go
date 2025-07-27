package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit key.Binding
	Help key.Binding
	Tab  key.Binding
	ToggleTheme key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.ToggleTheme}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit, k.Tab, k.ToggleTheme},
	}
}

var DefaultKeyMap = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch view"),
	),
	ToggleTheme: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle theme"),
	),
}