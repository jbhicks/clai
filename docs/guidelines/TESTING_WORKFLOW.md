# Testing Workflow and Mandatory Requirements

All code changes in CLAI must be verified following these testing standards.

## ⚠️ MANDATORY TESTING REQUIREMENT
**NO EXCEPTIONS**: Agents MUST use `clai-debug` MCP tools to verify ALL changes that affect user interaction, UI, or functionality.
- **FAILURE TO TEST = AUTOMATIC FAILURE**. No code may be committed without verification.
- Agents may NOT ask users to test changes; you must verify them yourself.

### Required Workflow
1. **CAPTURE BASELINE**: Use `clai-debug_inspect` and `clai-debug_inspect_styles` before changes.
2. **VERIFY AUTO-RESTART**: Confirm the process restarted after code edits (check Build ID in status bar).
3. **INSPECT AFTER**: Use `clai-debug` tools to verify the change is visible and correct.
4. **SIMULATE INTERACTION**: Use `clai-debug_send_key` to test navigation and input.

## Test-Driven Development (TDD)
Required for:
- New API endpoints (HTTP handlers).
- Complex logic functions.
- Bug fixes (reproducible failing test first).

## Automated Testing with tmux
Spawn a detached tmux session to verify UI rendering without blocking your current thread.

```bash
tmux new-session -d -s clai-test 'go run ./cmd/clai'
sleep 3
tmux capture-pane -t clai-test -p
tmux kill-session -t clai-test
```

## Debugging Tools Summary
- `clai-debug_inspect`: Full UI inspection.
- `clai-debug_inspect_styles`: Structured viewport dimensions (JSON).
- `clai-debug_get_history`: Examine conversation state.
- `clai-debug_send_key`: Simulate keystrokes.
- `clai-debug_type_text`: Simulate typing.
