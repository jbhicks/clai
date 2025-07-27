# clai
An AI interface, built for local AI, by AI. AlrAIght?

# CLAI Implementation Plan

[...unchanged content above...]

## Live Reload for Bubble Tea TUI

[...unchanged content above...]

This workflow ensures instant reloads and a robust TUI experience.

---

### Recommended: tmux-based Live Reload for Bubble Tea TUI

Bubble Tea TUIs require a real TTY for proper rendering and hot-reload. The best workflow is to use tmux with two panes:

- **Pane 0:** Runs the TUI app
- **Pane 1:** Runs a watcher that rebuilds and restarts the app on code changes

To start this workflow, run:

```sh
make dev
```

This will:
- Check for tmux installation
- Start a tmux session named `clai_dev` with two panes
- Pane 0 runs the TUI
- Pane 1 watches for code changes and restarts the TUI automatically

To attach to the session:
```sh
tmux attach -t clai_dev
```

---

### One-liner Live Reload with entr (legacy)

For a single-terminal live reload workflow using [entr](https://eradman.com/entrproject/), run this command from your project root:

```sh
find . -type f -name '*.go' | grep -v '/_build/' | grep -v '/vendor/' | entr -r ./dev_entr.sh
```

- This will rebuild and restart the app automatically whenever any Go source file changes.
- The TUI will remain visible in your terminal.
- All logs will be written to `debug.log` (overwritten on each run).

---

**Alternative (for simple projects):**

If you want to watch only the Go files in your current directory, you can use:

```sh
ls *.go | entr ./dev_entr.sh
```

> **Note:**  
> Always run these commands in a real terminal window (not in the background or via a tool that captures output), so the TUI appears as expected.

---
