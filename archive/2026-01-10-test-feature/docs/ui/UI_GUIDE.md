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
    if m.width == 0 || m.height == 0 {
        return "Initializing..."
    }

    // Pattern: Sectional Layout (Header-Body-Footer)
    // Avoid a single global box that hits the absolute edges (m.width, m.height).
    // Instead, join sections vertically and always leave 1 row at the bottom.
    header := lipgloss.NewStyle().Width(m.width).Render("Header")
    footer := lipgloss.NewStyle().Width(m.width).Render("Footer")
    
    // The -1 to height is CRITICAL to prevent terminal scrolling
    bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 1
    body := lipgloss.NewStyle().Width(m.width).Height(bodyHeight).Render("Content")
    
    return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
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

---

## Troubleshooting: UI Exceeds Terminal Window Bounds

If your Bubble Tea UI is overflowing or not fitting within the terminal window, follow these steps:

### Common Causes
- **Not handling `tea.WindowSizeMsg`**: Your model must update its stored width/height on every terminal resize.
- **Hardcoded sizes**: Avoid fixed widths/heights; use the terminal size from your model.
- **Not passing terminal size to components**: Bubbles components (list, table, viewport, etc.) need their width/height set from your model’s terminal size.
- **Custom layout logic**: Prefer Lip Gloss and Bubble Tea’s built-in layout utilities.
- **Not running in a real TTY**: Bubble Tea TUIs require a real terminal (not a background process or captured output) for proper sizing and rendering.

### Quick Fix Checklist
1. **Handle `tea.WindowSizeMsg` in your Update function.**
2. **Store `msg.Width` and `msg.Height` in your model.**
3. **Use these values to size all components in your View function.**
4. **Set Bubbles component sizes from your model’s width/height.**
5. **Use Lip Gloss’s layout utilities (`JoinHorizontal`, `JoinVertical`, etc.) for composing layouts.**
6. **Avoid hardcoding sizes.**
7. **Use the recommended tmux-based dev workflow for live reload.**

### Example Fix
```go
// In your Update:
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        // Update Bubbles components:
        m.list.SetSize(m.width, m.height)
    }
    return m, nil
}

// In your View:
func (m model) View() string {
    return lipgloss.NewStyle().
        Width(m.width).
        Height(m.height).
        Render(m.list.View())
}
```

### Advanced Tips
- Use `MaxWidth`, `MaxHeight`, and Lip Gloss’s measuring functions to constrain or adapt layouts.
- For complex UIs, break your layout into smaller components and size each from the model’s terminal size.

### The "Scrolling UI" Mystery
- **Problem**: Your UI seems to shift up by one line, and the top border disappears.
- **Cause**: Bubble Tea always appends a mandatory `\n` to your `View()` output. If your output is exactly `m.height` lines tall, it becomes `m.height + 1`, forcing the terminal to scroll.
- **Fix**: Always subtract 1 from your total calculated height: `totalHeight := m.height - 1`.

### The "Stuttering Dimensions" bug
- **Problem**: In Lipgloss, `Width()` and `Height()` set the *total* area including borders/padding.
- **Trap**: Manual subtraction of frame sizes (e.g. `m.width - 2`) often leads to double-subtraction or off-screen drawing.
- **Rule**: Use a sectional layout (Header/Body/Footer) instead of one big outer BoxStyle. Sectional layouts are more stable across different terminal emulators.

---

## Chat Viewport Features

The chat viewport includes several user-friendly features for navigating conversation history:

### Auto-Scroll Behavior
- **Smart auto-scroll**: Automatically scrolls to show new messages when the user sends a message or when streaming responses arrive
- **Manual scroll detection**: When you scroll up to review history, auto-scroll is disabled to preserve your position
- **Re-enable auto-scroll**: Scroll to the bottom to re-enable automatic scrolling for new messages

### Scroll Controls

**Mouse:**
- Mouse wheel scrolling enabled with 3-line delta for smooth navigation

**Keyboard shortcuts (when input is not focused):**
- `↑` or `k`: Scroll up one line
- `↓` or `j`: Scroll down one line
- `Page Up`: Scroll up one page
- `Page Down`: Scroll down one page
- `Home` or `g`: Jump to top of conversation
- `End` or `G`: Jump to bottom of conversation

### Visual Indicators
- **Scroll position indicator**: Shows when not at top/bottom with:
  - Current scroll percentage (0-100%)
  - Directional arrows: `↑` (can scroll up), `↓` (can scroll down), `↕` (can scroll both ways)
- Indicator automatically hides when at top or bottom of conversation

### Message Styling
- **User messages**: Right-aligned with blue rounded border, max 70% width, dynamic left margin
- **Assistant messages**: Left-aligned with green rounded border, max 70% width
- Messages use `MaxWidth()` to allow natural sizing based on content
- Tool execution badges displayed inline with assistant messages
- Modern chat bubble appearance with rounded borders and proper padding

### Implementation Notes
The chat bubbles follow Bubble Tea best practices:
- Use `MaxWidth()` instead of `Width()` to allow natural bubble sizing
- Calculate margins dynamically based on rendered bubble width
- User messages pushed to the right via dynamic left margin
- Assistant messages naturally left-aligned with no extra margin


