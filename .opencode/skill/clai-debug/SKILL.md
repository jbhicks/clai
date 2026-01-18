---
name: clai-debug
description: Debug and inspect CLAI TUI state, viewport content, and conversation history
---

# CLAI Debug

This skill provides tools for inspecting and debugging the CLAI terminal user interface.

Use this when you need to:
- Inspect current UI state and viewport content
- Check terminal and pane dimensions
- Retrieve conversation history
- Test keyboard interactions
- Verify UI changes after code modifications
- Debug TUI rendering issues

All tools communicate with the CLAI debug server via Unix socket at `/tmp/clai.sock`.

## Available Tools

Once loaded, the following MCP tools become available:
- `ping` - Test connectivity to the debug server
- `inspect` - Get full UI inspection including viewport content, dimensions, and state
- `inspect_styles` - Get structured viewport dimensions and state info (JSON format)
- `get_history` - Get the conversation history/messages
- `send_key` - Send a keystroke to the TUI (e.g., "enter", "ctrl+h", "up", "down")
- `type_text` - Type text into the input field
- `send_message` - Inject a message into the conversation
- `switch_pane` - Switch between chat and log panes

## Workflow

1. Load this skill when you need to inspect CLAI's UI state
2. Use the `inspect` tool to see current viewport content and dimensions
3. Use `inspect_styles` for structured JSON data about layout
4. Use `send_key` or `type_text` to test interactions
5. Use `get_history` to verify conversation flow
