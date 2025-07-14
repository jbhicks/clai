package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Logging helpers
func logDebug(msg string) {
	f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(f)
		log.Println(msg)
		f.Close()
	}
}

// Logging helpers
// UIComponent is an interface for prototyping any Bubble Tea component
// It must implement Update, View, and optionally Init
// This allows you to swap in any component for rapid prototyping
// Example usage: see README.md

type UIComponent interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (UIComponent, tea.Cmd)
	View() string
}

// ...rest of file unchanged...
