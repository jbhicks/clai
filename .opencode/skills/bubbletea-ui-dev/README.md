# Bubble Tea UI Development Skill

This skill provides comprehensive guidance and helper utilities for building terminal user interfaces with Bubble Tea and Lipgloss.

## Quick Start

### Full-Screen UI Setup

```go
type model struct {
    width  int
    height int
    // your components...
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.updateLayout(msg.Width, msg.Height)
        return m, tea.ClearScreen // Clear screen on resize
    }
    return m, nil
}

func main() {
    p := tea.NewProgram(
        &model{},
        tea.WithAltScreen(),    // Full screen mode
        tea.WithMouseCellMotion(), // Optional: mouse support
    )
    
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v", err)
        os.Exit(1)
    }
}
```

### Responsive Two-Panel Layout

```go
func (m model) View() string {
    // Use LayoutHelper for calculations
    helper := bubbletea.NewLayoutHelper(tea.WindowSizeMsg{Width: m.width, Height: m.height})
    leftWidth, rightWidth := helper.TwoColumnLayout(0.7) // 70% left, 30% right
    
    // Use ColorScheme for consistent styling
    colors := bubbletea.CLAIColorScheme()
    
    leftPanel := colors.PanelStyle().
        Width(leftWidth).
        Height(m.height).
        Render(m.leftContent())
    
    rightPanel := colors.PanelStyle().
        Width(rightWidth).
        Height(m.height).
        Render(m.rightContent())
    
    return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}
```

## Helper Components

### LayoutHelper
- **TwoColumnLayout(percentage)** - Split screen into two columns
- **ThreeColumnLayout(left%, middle%)** - Split screen into three columns  
- **InnerDimensions(style)** - Calculate inner dimensions accounting for borders
- **IsSmallTerminal()** - Check if terminal is too small for complex layouts
- **GetLayoutType()** - Returns "mobile", "tablet", or "desktop" based on size

### ColorScheme
- **CLAIColorScheme()** - CLAI's Dracula-based color theme
- **DarkColorScheme()** - Standard dark theme
- **PanelStyle()** - Styled panel with rounded borders
- **FocusedPanelStyle()** - Highlighted panel for focused elements
- **BorderStyle()** - Border with transparent gap prevention

## Best Practices

### 1. Always Handle Window Resize
```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.updateLayout(msg.Width, msg.Height)
    return m, tea.ClearScreen
```

### 2. Use Safe Dimension Calculations
```go
// Account for frame size
innerWidth := outerWidth - style.GetHorizontalFrameSize()

// Guard against negative values
if width < 0 {
    width = 0
}
```

### 3. Prevent Border Transparency Gaps
```go
// ALWAYS set both border foreground AND background
style := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#ffb86c")).
    BorderBackground(lipgloss.Color("#282a36")) // Critical!
```

### 4. Test Responsive Behavior
```go
// Test different terminal sizes
func TestLayout(t *testing.T) {
    for w := 40; w <= 120; w += 20 {
        for h := 10; h <= 40; h += 10 {
            helper := NewLayoutHelper(tea.WindowSizeMsg{Width: w, Height: h})
            left, right := helper.TwoColumnLayout(0.7)
            
            assert.True(t, left > 0)
            assert.True(t, right > 0)
            assert.Equal(t, w, left+right)
        }
    }
}
```

## Common Layout Patterns

### Main Content + Sidebar
```go
mainWidth, sidebarWidth := helper.TwoColumnLayout(0.8)
mainContent := colors.PanelStyle().Width(mainWidth).Render(m.mainView())
sidebar := colors.PanelStyle().Width(sidebarWidth).Render(m.sidebarView())
return lipgloss.JoinHorizontal(lipgloss.Top, mainContent, sidebar)
```

### Header + Content + Footer
```go
headerHeight := 3
footerHeight := 2
contentHeight := m.height - headerHeight - footerHeight

header := colors.AccentStyle().Height(headerHeight).Render(m.headerView())
content := colors.BaseStyle().Height(contentHeight).Render(m.contentView())
footer := colors.MutedStyle().Height(footerHeight).Render(m.footerView())

return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
```

### Scrollable Content Area
```go
func (m *model) updateLayout(width, height int) {
    innerWidth, innerHeight := helper.InnerDimensions(colors.PanelStyle())
    m.viewport = viewport.New(innerWidth, innerHeight)
    m.viewport.SetContent(m.longContent)
}

func (m model) View() string {
    return colors.PanelStyle().
        Width(m.width).
        Height(m.height).
        Render(m.viewport.View())
}
```

## Integration with CLAI

When modifying CLAI's TUI:
1. **Use clai-debug tools** to inspect UI state before/after changes
2. **Follow CLAI's existing patterns** and color scheme
3. **Test with make dev** to leverage auto-reload
4. **Verify with multiple terminal sizes** to ensure responsive behavior
5. **Check for transparency gaps** using `clai-debug inspect`

## Debugging Tips

### Add Dimension Logging
```go
func (m model) View() string {
    fmt.Printf("Terminal: %dx%d, Calculated: left=%d, right=%d\n", 
        m.width, m.height, m.leftWidth, m.rightWidth)
    // ... render logic
}
```

### Use Viewport for Large Content
```go
// Instead of truncating text, use viewports
m.viewport = viewport.New(width-4, height-4)
m.viewport.SetContent(longText)
m.viewport.GotoBottom() // Auto-scroll to latest
```

### Test Edge Cases
- Very narrow terminals (< 40 columns)
- Very short terminals (< 10 rows) 
- Window resizing during runtime
- Unicode content with wide characters
- Long lines without spaces

## Resources

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
- CLAI's existing TUI implementation in `internal/ui/`

## Troubleshooting

**UI doesn't update on resize**: Ensure you return `tea.ClearScreen` command
**Text is truncated**: Subtract frame size from outer dimensions  
**Transparent gaps in borders**: Set both `BorderForeground()` and `BorderBackground()`
**Layout breaks on small terminals**: Guard against negative calculations with minimum values