# Bubble Tea UI Layout & Dynamic Sizing Guide

This guide summarizes best practices for building dynamic, responsive layouts in Bubble Tea using Lip Gloss and Bubbles components.

---

## 1. Use Lip Gloss for Layout and Sizing
- **Lip Gloss** is the official layout and styling library for Bubble Tea. It provides CSS-like APIs for padding, margin, width, height, alignment, and more.
- **Dynamic sizing** is achieved by:
  - Not setting fixed `Width` or `Height` on your `lipgloss.Style` unless you want to constrain the component.
  - Using `lipgloss.JoinHorizontal` and `lipgloss.JoinVertical` to assemble layouts that adapt to content size.
  - Measuring content with `lipgloss.Width()` and `lipgloss.Height()` to dynamically size or align components.
  - Using `lipgloss.PlaceHorizontal`, `lipgloss.PlaceVertical`, and `lipgloss.Place` to position content within available space.

## 2. Let Bubble Tea Pass Terminal Size to Your Model
- Bubble Tea passes the terminal size to your model via the `tea.WindowSizeMsg` message.
- In your `Update` function, handle `tea.WindowSizeMsg` and store the width/height in your model.
- Use these values to size your components dynamically in your `View` function.

## 3. Best Practices for Dynamic Sizing
- **Do not hardcode sizes** unless necessary. Use the terminal size from `WindowSizeMsg` for layout.
- **Compose layouts** using Lip Gloss utilities (`JoinHorizontal`, `JoinVertical`, etc.) so that each component can size itself based on its content and available space.
- **For Bubbles components** (e.g., list, table, viewport), set their width/height properties from your model’s stored terminal size.
- **Avoid custom layout logic**; prefer Lip Gloss and Bubble Tea’s built-in mechanisms.

## 4. Example Pattern
```go
// In your model:
type model struct {
    width  int
    height int
    // ... other fields
}

// In your Update:
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    }
    // ... other cases
    return m, nil
}

// In your View:
func (m model) View() string {
    // Use m.width and m.height to size components
    content := lipgloss.NewStyle().Width(m.width).Height(m.height).Render("Hello, world!")
    return content
}
```

## 5. Component-Specific Sizing
- For Bubbles components (e.g., `list.Model`, `table.Model`, `viewport.Model`), set their `Width` and `Height` fields from your model’s stored terminal size.
- Most Bubbles components will automatically handle content overflow, scrolling, and resizing if you update their size on `WindowSizeMsg`.

## 6. Advanced Layouts
- Use Lip Gloss’s measuring functions (`Width`, `Height`, `Size`) to calculate how much space a component needs.
- Use `MaxWidth`, `MaxHeight`, and `Inline` to enforce constraints if needed.
- Use `PlaceHorizontal`, `PlaceVertical`, and `Place` for advanced positioning.

---

## Actionable Recommendations

1. **Always handle `tea.WindowSizeMsg` in your Update function.**
2. **Store the terminal size in your model and use it to size components in your View.**
3. **Use Lip Gloss for all layout, sizing, and alignment.**
4. **For Bubbles components, set their width/height from your model’s terminal size.**
5. **Avoid hardcoding sizes; let the terminal and content dictate layout.**
6. **Use Lip Gloss’s layout utilities for assembling complex UIs.**

---

**References:**
- [Lip Gloss README](https://github.com/charmbracelet/lipgloss)
- [Bubble Tea README](https://github.com/charmbracelet/bubbletea)
- [Bubbles README](https://github.com/charmbracelet/bubbles)
- [Lip Gloss Docs](https://pkg.go.dev/github.com/charmbracelet/lipgloss)
- [Bubble Tea Examples](https://github.com/charmbracelet/bubbletea/tree/main/examples)

---

**Summary:**  
To let Bubble Tea dynamically size things for you, always handle terminal resize events, use Lip Gloss for layout, and set component sizes from your model’s stored terminal size. Compose your UI with Lip Gloss’s layout utilities and avoid hardcoding sizes.

If you need more specific code examples or want guidance for a particular component, see this file or the official docs.