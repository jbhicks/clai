# CLAI Agent Orchestrator UI Enhancement Plan

## Current UI Structure
CLAI uses Bubble Tea with 2-pane layout:
- **Chat Pane**: Main interaction area
- **Log Pane**: Shows system logs and agent activity
- **Status Bar**: Shows current status and keyboard shortcuts
- **Agent Status View**: Shows current agent reasoning and actions

## Proposed UI Enhancements

### 1. Agent Orchestrator Status Bar Enhancement
**Current**: Basic status text
**Enhanced**: Show agent orchestration metrics

```go
// Status bar shows: "3 agents running | 12 tasks completed | 2 conflicts resolved"
statusText := fmt.Sprintf("%d agents running | %d tasks completed | %d conflicts",
    orchStats["running"], orchStats["completed"], conflictsResolved)
```

### 2. Agent Management Commands in Chat
**New slash commands integrated into chat interface:**

```go
// In chat input, user can type:
/spawn code "implement authentication"
// /agents list
// /agents status code-agent-123
// /agents stop code-agent-123
// /dashboard  // Opens agent dashboard
```

### 3. Enhanced Agent Status Panel
**Current**: Shows single agent reasoning
**Enhanced**: Show multi-agent orchestration status

```go
type OrchestratorStatusView struct {
    agentOrch     *orchestrator.AgentOrchestrator
    refreshTicker *time.Ticker
    lastUpdate    time.Time
}

func (v *OrchestratorStatusView) View() string {
    stats := v.agentOrch.GetAgentStatus()

    return lipgloss.JoinVertical(lipgloss.Left,
        v.renderSummary(stats),
        v.renderActiveAgents(stats),
        v.renderRecentActivity(),
    )
}
```

### 4. New Keyboard Shortcuts
**Add to existing keymap:**

```go
// Ctrl+A: Toggle agent orchestration pane
// Ctrl+Shift+A: Open agent dashboard
// Ctrl+Shift+S: Spawn new agent (prompts for type/task)
```

### 5. Agent Activity Log Integration
**Enhance log pane to show agent orchestration events:**

```
[23:45:12] 🤖 Spawned code-agent-abc for "implement auth"
[23:45:15] 📊 Ralph loop started for code-agent-abc
[23:46:02] ✅ Agent code-agent-abc completed task
[23:46:05] 🔄 Aggregating results from 3 completed agents
```

## Implementation Plan

### Phase 1: Status Bar Enhancement
- Add agent metrics to existing status bar
- Show running agent count and recent activity

### Phase 2: Chat Command Integration
- Add `/agent` slash commands to chat interface
- Parse and execute agent orchestration commands
- Show results in chat pane

### Phase 3: Enhanced Agent Status View
- Extend current AgentStatusView to show orchestrator status
- Add agent list with real-time status updates
- Show conflict resolution status

### Phase 4: Dedicated Agent Pane
- Add third pane option for detailed agent management
- Toggle between: Chat+Log, Chat+Agents, Log+Agents
- Full agent lifecycle management interface

## UI Mockup

```
┌─ CLAI Agent Orchestrator ─ 3 agents running | 12 tasks completed ─┐
│                                                                    │
│ 🤖 Agent Status: 2 running, 1 completed, 0 failed                 │
│ ┌─ Active Agents ──────────────────────────────────────────────┐   │
│ │ code-agent-abc    RUNNING  implement auth                45s │   │
│ │ test-agent-def    RUNNING  create test suite             32s │   │
│ │ research-agent-ghi COMPLETED analyze security best practices │   │
│ └─────────────────────────────────────────────────────────────┘   │
│                                                                    │
│ > /spawn review "review authentication implementation"             │
│                                                                    │
│ [23:45:12] 🤖 Spawned review-agent-jkl for "review auth"           │
│ [23:45:15] 📊 Ralph loop started for review-agent-jkl              │
└────────────────────────────────────────────────────────────────────┘
```

## Benefits

1. **Integrated Experience**: Agent orchestration feels natural in the existing UI
2. **Real-time Monitoring**: Live status updates without leaving the chat
3. **Command-line Power**: Full CLI capabilities accessible through chat commands
4. **Progressive Enhancement**: Existing functionality unchanged, new features additive
5. **Consistent Design**: Uses existing Bubble Tea components and styling

This enhancement plan maintains CLAI's clean, efficient TUI while adding comprehensive agent orchestration capabilities that feel natural and integrated.