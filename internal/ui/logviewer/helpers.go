package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// safeClamp ensures we never return negative widths
func safeClamp(width, sub int) int {
	w := width - sub
	if w < 0 {
		return 0
	}
	return w
}

// truncateRunes truncates s to max runes, rune-aware
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max > 3 {
		return string(r[:max-3]) + "..."
	}
	return string(r[:max])
}

// padLineWithBackground renders s with style and ensures the result is exactly width columns wide
func padLineWithBackground(s string, width int, style lipgloss.Style) string {
	if width <= 0 {
		return style.Render(s)
	}
	out := style.Render(s)
	// If lipgloss.Width is unavailable, fall back to simple approach
	w := lipgloss.Width(out)
	if w >= width {
		return out
	}
	pad := width - w
	// Render padding using style to avoid transparent spaces
	padding := style.Width(pad).Render(" ")
	return out + padding
}

// glamorRenderIfNeeded renders markdown body to ANSI using Glamour when bodyFormat == "markdown".
// wordWrapWidth controls the Glamour renderer wrap width.
func glamorRenderIfNeeded(body string, bodyFormat string, wordWrapWidth int) string {
	if body == "" || bodyFormat != "markdown" {
		return body
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithWordWrap(wordWrapWidth),
		glamour.WithAutoStyle(),
	)
	out, err := r.Render(body)
	if err != nil {
		// fallback to raw body
		return body
	}
	return out
}
