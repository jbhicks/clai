# CLAI UI Component Standards

## Bubble Tea Component Usage Requirements

### Table Component for Tabular Data
**ALWAYS use `bubbles/table` for displaying tabular data** instead of manual formatting:

```go
import "github.com/charmbracelet/bubbles/table"

func createAgentTable(agents []Agent) string {
    columns := []table.Column{
        {Title: "ID", Width: 12},
        {Title: "Type", Width: 10},
        {Title: "Status", Width: 12},
        {Title: "Task", Width: 25},
    }

    rows := make([]table.Row, len(agents))
    for i, agent := range agents {
        rows[i] = table.Row{
            agent.ID,
            agent.Type,
            agent.Status,
            truncateString(agent.Task, 25),
        }
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithRows(rows),
        table.WithFocused(false),
    )

    return t.View()
}
```

**Benefits:**
- Consistent styling and navigation
- Keyboard navigation (arrow keys, vim bindings)
- Responsive column sizing
- Built-in sorting and filtering capabilities
- Professional appearance

### Progress Component for Long Operations
**ALWAYS use `bubbles/progress` for showing progress** of long-running operations:

```go
import "github.com/charmbracelet/bubbles/progress"

func showLLMProgress(percent float64, operation string) string {
    p := progress.New(progress.WithDefaultGradient())
    progressView := p.ViewAs(percent)

    return fmt.Sprintf("%s\n%.0f%% complete", operation, percent*100) + "\n" + progressView
}
```

**When to use:**
- LLM API calls
- File processing
- Agent task execution
- Any operation taking >2 seconds

### List Component for Item Selection
**ALWAYS use `bubbles/list` for selectable item lists**:

```go
import "github.com/charmbracelet/bubbles/list"

func createAgentList(agents []Agent) list.Model {
    items := make([]list.Item, len(agents))
    for i, agent := range agents {
        items[i] = AgentListItem{
            title:       agent.Name,
            description: fmt.Sprintf("%s - %s", agent.Type, agent.Status),
        }
    }

    delegate := list.NewDefaultDelegate()
    l := list.New(items, delegate, width, height)
    l.Title = "🤖 Active Agents"

    return l
}
```

### Textarea Component for Multi-line Input
**ALWAYS use `bubbles/textarea` for multi-line text input** instead of single-line text input:

```go
import "github.com/charmbracelet/bubbles/textarea"

func createMultilineInput() textarea.Model {
    ta := textarea.New()
    ta.Placeholder = "Enter your multi-line prompt here..."
    ta.CharLimit = 0 // No limit for LLM prompts
    ta.MaxHeight = 8 // Allow reasonable height

    return ta
}
```

## Implementation Checklist

### UI-001: Table Component ✅
- [x] Use `bubbles/table` for agent status display
- [x] Implement proper column sizing
- [x] Add table styling with borders and colors
- [x] Ensure keyboard navigation works

### UI-002: Progress Component ✅
- [x] Use `bubbles/progress` for long operations
- [x] Show percentage completion
- [x] Integrate with existing UI layout
- [x] Provide status messages

### Future Enhancements
- [ ] Add table sorting capabilities
- [ ] Implement table filtering
- [ ] Add progress bar animations
- [ ] Create custom table delegates for agent-specific rendering

## Migration Guide

### Replace Manual Tables
**Before:**
```go
fmt.Printf("%-10s %-15s %-12s\n", "ID", "Name", "Status")
for _, agent := range agents {
    fmt.Printf("%-10s %-15s %-12s\n", agent.ID, agent.Name, agent.Status)
}
```

**After:**
```go
columns := []table.Column{
    {Title: "ID", Width: 10},
    {Title: "Name", Width: 15},
    {Title: "Status", Width: 12},
}
rows := []table.Row{{"agent-1", "CodeAgent", "running"}}
t := table.New(table.WithColumns(columns), table.WithRows(rows))
fmt.Print(t.View())
```

### Replace Manual Progress
**Before:**
```go
fmt.Printf("Processing... %d/%d\n", current, total)
```

**After:**
```go
percent := float64(current) / float64(total)
p := progress.New()
fmt.Print(p.ViewAs(percent))
```

This ensures CLAI maintains a professional, consistent, and accessible UI using Bubble Tea's battle-tested components.