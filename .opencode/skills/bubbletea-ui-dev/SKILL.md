---
name: bubbletea-ui-dev
description: Comprehensive Bubble Tea TUI development guidance with best practices, responsive layouts, and debugging strategies
---

# Bubble Tea UI Development

This skill provides expert guidance for building terminal user interfaces with Bubble Tea and Lipgloss, focusing on full-screen layouts, responsive design, and debugging strategies.

## When to Use This Skill

Use this when you need to:
- Create full-screen TUI layouts that fill terminal window
- Handle window resize events properly  
- Design responsive layouts that adapt to terminal size
- Debug TUI rendering issues and layout problems
- Implement multi-panel layouts with proper sizing
- Calculate dimensions for components with borders and padding
- Create scrollable content areas with viewports
- Implement proper keyboard navigation and focus management

## Core Concepts

### Full-Screen UI Setup
1. **Enable Alt Screen Mode**: Use `tea.WithAltScreen()` for full-screen experience
2. **Handle Window Size**: Always handle `tea.WindowSizeMsg` in your Update method
3. **Store Dimensions**: Keep width/height in your model for layout calculations
4. **Clear on Resize**: Return `tea.ClearScreen` command when window resizes

### Responsive Layout Patterns
1. **Percentage-based Sizing**: Calculate component sizes as percentages of terminal dimensions
2. **Frame Size Awareness**: Account for borders and padding when calculating inner dimensions
3. **Minimum Dimensions**: Guard against very small terminals that cause negative calculations
4. **Adaptive Layouts**: Change layout structure based on available terminal size

### Dimension Calculations
```go
// Safe frame size calculation
innerWidth := outerWidth - style.GetHorizontalFrameSize()
innerHeight := outerHeight - style.GetVerticalFrameSize()

// Percentage-based split
leftWidth := int(float64(totalWidth) * 0.7)
rightWidth := totalWidth - leftWidth

// Guard against negative values
if padCount < 0 {
    padCount = 0
}
```

## Common Layout Patterns

### Two-Panel Layout
```go
func (m model) View() string {
    leftWidth := int(float64(m.width) * 0.8)
    rightWidth := m.width - leftWidth
    
    left := lipgloss.NewStyle().
        Width(leftWidth).
        Height(m.height).
        Border(lipgloss.NormalBorder()).
        Render(m.leftContent())
    
    right := lipgloss.NewStyle().
        Width(rightWidth).
        Height(m.height).
        Border(lipgloss.NormalBorder()).
        Render(m.rightContent())
    
    return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}
```

### Scrollable Content Area
```go
func (m *model) updateLayout(width, height int) {
    m.viewport = viewport.New(width-4, height-4) // Account for borders
    m.viewport.SetContent(m.longContent)
}

func (m model) View() string {
    return lipgloss.NewStyle().
        Width(m.width).
        Height(m.height).
        Border(lipgloss.RoundedBorder()).
        BorderBackground(lipgloss.Color("#282a36")).
        Render(m.viewport.View())
}
```

## Debugging TUI Issues

### Dimension Inspection
- Add logging of width/height values in Update method
- Use `lipgloss.Width()` and `lipgloss.Height()` to verify rendered sizes
- Check that calculated dimensions match expected values

### Common Problems & Solutions
1. **Truncated Text**: Ensure frame size is subtracted from outer dimensions
2. **Overflow**: Use percentage-based calculations instead of fixed sizes
3. **Broken Layout on Resize**: Always return `tea.ClearScreen` on `tea.WindowSizeMsg`
4. **Transparent Gaps**: Set both `BorderForeground()` and `BorderBackground()` on styled elements

### Color and Border Best Practices
```go
// Always set border background to prevent transparency gaps
style := lipgloss.NewStyle().
    Background(lipgloss.Color("#282a36")).
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#ffb86c")).
    BorderBackground(lipgloss.Color("#282a36")) // Critical!
```

## Advanced Topics

### Multi-Component Layouts
1. Calculate all dimensions in a single `updateLayout` method
2. Store component widths/heights in the model
3. Use `lipgloss.JoinHorizontal` and `lipgloss.JoinVertical` for layout assembly
4. Test with various terminal sizes to ensure responsive behavior

### Performance Optimization
- Minimize style recalculations in View() method
- Cache style objects when possible
- Avoid expensive operations in the render loop
- Use viewports for large content areas

### Accessibility
- Implement keyboard navigation for all interactive elements
- Provide visual feedback for focused components
- Use high-contrast colors for better readability
- Test with different terminal color schemes

## Testing Strategy

### Terminal Size Testing
```go
// Test various terminal sizes
for w := 40; w <= 120; w += 20 {
    for h := 10; h <= 40; h += 10 {
        m.updateLayout(w, h)
        // Verify no panics and reasonable layout
    }
}
```

### Window Resize Testing
- Test with manual terminal resizing during development
- Verify layout adapts smoothly to size changes
- Check for rendering artifacts or overflow issues
- Ensure scroll areas maintain content properly

## Integration with CLAI

When working on CLAI's TUI components:
- Use existing debug tools for inspection
- Follow CLAI's established color scheme and styling patterns
- Test changes with `make dev` and auto-reload
- Verify functionality with clai-debug tools before considering complete
- Check for conflicts with existing layout and focus management

## Available Tools

Once loaded, this skill provides:
- Documentation access for Bubble Tea and Lipgloss APIs
- Layout calculation helpers and examples
- Debugging strategies for common TUI issues
- Best practices for responsive terminal UI design
- Integration patterns for CLAI's existing TUI components

## Workflow

1. Load this skill when starting Bubble Tea UI work
2. Use provided patterns for common layout needs
3. Follow dimension calculation best practices
4. Test with various terminal sizes
5. Use debugging strategies when issues arise
6. Verify changes with proper testing methodology