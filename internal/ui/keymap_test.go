package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestKeyMapHasAllRequiredBindings(t *testing.T) {
	km := DefaultKeyMap

	bindings := map[string]key.Binding{
		"Quit":        km.Quit,
		"Help":        km.Help,
		"Tab":         km.Tab,
		"ToggleTheme": km.ToggleTheme,
		"NewConv":     km.NewConv,
		"ScrollUp":    km.ScrollUp,
		"ScrollDown":  km.ScrollDown,
		"PageUp":      km.PageUp,
		"PageDown":    km.PageDown,
		"Home":        km.Home,
		"End":         km.End,
		"Send":        km.Send,
	}

	for name, binding := range bindings {
		if len(binding.Keys()) == 0 {
			t.Errorf("binding %s has no keys defined", name)
		}
		if binding.Help().Key == "" {
			t.Errorf("binding %s has no help key defined", name)
		}
		if binding.Help().Desc == "" {
			t.Errorf("binding %s has no help description defined", name)
		}
	}
}

func TestKeyMapShortHelp(t *testing.T) {
	km := DefaultKeyMap
	shortHelp := km.ShortHelp()

	if len(shortHelp) != 3 {
		t.Errorf("expected 3 bindings in short help, got %d", len(shortHelp))
	}

	expectedShort := []key.Binding{km.Help, km.Quit, km.ToggleTheme}
	for i, expected := range expectedShort {
		if i >= len(shortHelp) {
			t.Errorf("short help missing binding at index %d", i)
			continue
		}
		if shortHelp[i].Help().Key != expected.Help().Key {
			t.Errorf("short help index %d: expected key %s, got %s",
				i, expected.Help().Key, shortHelp[i].Help().Key)
		}
	}
}

func TestKeyMapFullHelp(t *testing.T) {
	km := DefaultKeyMap
	fullHelp := km.FullHelp()

	if len(fullHelp) != 2 {
		t.Errorf("expected 2 columns in full help, got %d", len(fullHelp))
	}

	col1Expected := []key.Binding{km.Help, km.Quit, km.Tab, km.ToggleTheme, km.NewConv, km.Send}
	col2Expected := []key.Binding{km.ScrollUp, km.ScrollDown, km.PageUp, km.PageDown, km.Home, km.End}

	allCols := [][]key.Binding{col1Expected, col2Expected}

	for colIdx, expectedCol := range allCols {
		if colIdx >= len(fullHelp) {
			t.Errorf("full help missing column %d", colIdx)
			continue
		}

		actualCol := fullHelp[colIdx]
		if len(actualCol) != len(expectedCol) {
			t.Errorf("column %d: expected %d bindings, got %d", colIdx, len(expectedCol), len(actualCol))
			continue
		}

		for i, expected := range expectedCol {
			if actualCol[i].Help().Key != expected.Help().Key {
				t.Errorf("column %d, binding %d: expected key %s, got %s",
					colIdx, i, expected.Help().Key, actualCol[i].Help().Key)
			}
		}
	}
}

func TestSendKeyBinding(t *testing.T) {
	km := DefaultKeyMap

	if len(km.Send.Keys()) == 0 {
		t.Error("Send binding has no keys")
	}

	if !contains(km.Send.Keys(), "enter") {
		t.Error("Send binding should include 'enter' key")
	}

	if km.Send.Help().Key != "enter" {
		t.Errorf("expected Send help key to be 'enter', got %s", km.Send.Help().Key)
	}

	if km.Send.Help().Desc != "send message" {
		t.Errorf("expected Send help description to be 'send message', got %s", km.Send.Help().Desc)
	}
}

func TestAllImplementedCommandsInHelp(t *testing.T) {
	km := DefaultKeyMap
	fullHelp := km.FullHelp()

	allBindings := []key.Binding{}
	for _, row := range fullHelp {
		allBindings = append(allBindings, row...)
	}

	requiredKeys := []string{
		"ctrl+h",
		"ctrl+q",
		"ctrl+t",
		"ctrl+d",
		"ctrl+n",
		"enter",
		"up",
		"down",
		"pgup",
		"pgdown",
		"home",
		"end",
	}

	foundKeys := make(map[string]bool)
	for _, binding := range allBindings {
		for _, k := range binding.Keys() {
			foundKeys[k] = true
		}
	}

	for _, required := range requiredKeys {
		if !foundKeys[required] {
			t.Errorf("required key %s not found in full help", required)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
