# TUI Patterns and Best Practices

This document outlines the standard patterns for building Terminal User Interfaces (TUIs) in CLAI using Bubble Tea and Lipgloss.

## Sectional Layout Pattern (Preferred)

Avoid wrapping your entire app in a single `lipgloss.Style` with borders. Instead, divide your UI into Header, Body, and Footer sections and join them vertically.

```go
func (m Model) View() string {
    header := HeaderStyle.Width(m.width).Render("Header Text")
    footer := FooterStyle.Width(m.width).Render("Footer Text")
    
    // The -1 to height is CRITICAL to prevent terminal scrolling
    bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 1
    body := BodyStyle.Width(m.width).Height(bodyHeight).Render(content)
    
    return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
```

## Critical Sizing Rules

### 1. The "Full-Screen" Height Rule
Always subtract 1 from the total terminal height (`m.height - 1`) when calculating vertical space. Bubble Tea appends a mandatory `\n` to all `View()` output; if you use the full height, the terminal will scroll and your top border will disappear.

### 2. Width and Height include Borders
In Lipgloss, `.Width(n)` sets the *total* occupancy of the component, including its borders and padding. You do not need to manually subtract frame sizes if you use a sectional layout.

### 3. Guard Width-Derived Arithmetic
Clamping any width-derived value to a non-negative integer before using it in `strings.Repeat`, slice bounds, or as arguments to lipgloss `Width()`/`Height()`.

```go
padCount := v.Width - 4
if padCount < 0 {
    padCount = 0
}
divider := strings.Repeat("━", padCount)
```

## Styling and Appearance

### Lipgloss Border Background Transparency
**ABSOLUTELY FORBIDDEN**: When using lipgloss borders, **ALWAYS** set `BorderBackground()` in addition to `BorderForeground()`. Otherwise, borders appear with transparent backgrounds, creating black gaps in the UI.

```go
style := lipgloss.NewStyle().
    Background(lipgloss.Color("#282a36")).
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#ffb86c")).
    BorderBackground(lipgloss.Color("#282a36")) // ← CRITICAL
```

### Background Coverage
Always set `.Background()` on your styles and use `lipgloss.Place` if you need to force a background color over an entire area.

## Component Guidelines

### Always Use Bubbles Library Components First
Check `github.com/charmbracelet/bubbles` before writing custom UI:
- `textinput.Model` / `textarea.Model`
- `list.Model`
- `viewport.Model`
- `table.Model`

### External Message Injection
When injecting messages from async operations, use `p.Send(msg)`. Never directly manipulate UI state from a goroutine.

```go
go func() {
    p.Send(streamChunkMsg{chunk: chunk})
}()
```

## Debugging UI Issues
- Use `clai-debug_inspect` to see actual rendered content.
- If the top of the UI is missing, you are likely rendering too many lines (check the -1 rule).
- If the UI is wrapping horizontally, you are likely hitting the absolute final column; try `m.width - 1`.
