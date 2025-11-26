package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit        key.Binding
	Help        key.Binding
	Tab         key.Binding
	ToggleTheme key.Binding
	NewConv     key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.ToggleTheme}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit, k.Tab, k.ToggleTheme, k.NewConv},
	}
}

var DefaultKeyMap = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+q", "ctrl+c"),
		key.WithHelp("ctrl+q/ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("ctrl+h", "toggle help"),
	),
	Tab: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("ctrl+t", "switch pane"),
	),
	ToggleTheme: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "toggle theme"),
	),
	NewConv: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "new conversation"),
	),
}
