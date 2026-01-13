# Ralph-OpenCode 🏔️

Fancy Ralph-driven development loop with OhMyOpenCode multi-agent orchestration.

## Overview

Ralph-OpenCode combines the **Ralph pattern** (autonomous agent loops with persistent memory) with **OhMyOpenCode's multi-agent orchestration** to create a powerful autonomous development system.

### Key Features

- 🎨 **Beautiful TUI** - Progress bars, spinners, and real-time updates
- 🚀 **Parallel Context Gathering** - Background agents prepare context before main task
- 🔍 **Quality Gates** - Typecheck, tests, and lint before commit
- 📚 **Pattern Learning** - Automatically updates AGENTS.md with discovered patterns
- 🌐 **Multi-Model Orchestration** - Uses Claude, Gemini, and Sonnet for different tasks
- 💾 **Persistent State** - Survives restarts via `.clai/stories.json`

## Installation

```bash
# Clone and install dependencies
cd scripts/ralph-opencode
bun install

# Build the CLI
bun run build

# Make the wrapper script executable
chmod +x ralph-omo.sh
```

## Quick Start

```bash
# Run with defaults (reads from .clai/stories.json)
./ralph-omo.sh

# Run with custom options
./ralph-omo.sh --max-iterations 100 --model opencode/claude-opus-4-5 --verbose
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  RalphLoop Orchestrator                                     │
│  - Coordinates the entire loop                              │
│  - Manages state and progress                               │
│  - Handles quality gates                                    │
└─────────────────────────────────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│ OpenCodeAgent│ │ ContextGather│ │ QualityGates │
│ - Executes   │ │ - Parallel   │ │ - Typecheck  │
│ - Multi-model│ │   research   │ │ - Tests      │
│ - Commits    │ │ - Patterns   │ │ - Lint       │
└──────────────┘ └──────────────┘ └──────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Persistence Layer                                          │
│  - .clai/stories.json (task state)                          │
│  - .clai/progress.json (iteration history)                  │
│  - AGENTS.md (learned patterns)                             │
└─────────────────────────────────────────────────────────────┘
```

## File Structure

```
scripts/ralph-opencode/
├── src/
│   ├── index.ts              # CLI entry point
│   ├── orchestrator.ts       # Main RalphLoop class
│   ├── types.ts              # TypeScript types
│   ├── agents/
│   │   ├── opencode.ts       # OpenCode agent integration
│   │   └── context-gatherer.ts # Parallel context gathering
│   ├── quality/
│   │   └── gates.ts          # Quality gate checks
│   ├── persistence/
│   │   └── patterns.ts       # AGENTS.md pattern learning
│   ├── tui/
│   │   └── index.ts          # Beautiful terminal UI
│   ├── config/
│   │   └── index.ts          # Configuration loader
│   └── utils/
│       ├── args.ts           # CLI argument parser
│       ├── banner.ts         # ASCII banner
│       └── logger.ts         # Pretty logging
├── ralph-omo.sh              # Wrapper script
├── package.json
└── tsconfig.json
```

## Configuration

Create a `.ralphrc.json` file:

```json
{
  "storiesFile": "./.clai/stories.json",
  "progressFile": "./.clai/progress.json",
  "agentsMdFile": "./AGENTS.md",
  "maxIterations": 50,
  "model": "opencode/claude-opus-4-5",
  "branchName": "ralph/feature",
  "verbose": false,
  "autoHandoff": true
}
```

## Usage with CLAI

This project is designed to work with CLAI's existing `.clai/stories.json`:

```bash
# From the CLAI project root
../scripts/ralph-opencode/ralph-omo.sh --max-iterations 20
```

## How It Works

1. **Load stories** from `.clai/stories.json`
2. **Pick next story** where `passes: false`
3. **Gather context** in parallel (language, build commands, patterns)
4. **Execute task** with OpenCode Sisyphus agent
5. **Run quality gates** (typecheck, tests, lint)
6. **Commit** if checks pass
7. **Learn patterns** - update AGENTS.md
8. **Update story** - set `passes: true`
9. **Repeat** until all stories complete or max iterations reached

## Multi-Agent Superpowers

Ralph-OpenCode leverages OhMyOpenCode's specialized agents:

- **Sisyphus** - Main coding agent (Claude Opus 4.5)
- **Librarian** - Documentation and research (Claude Sonnet)
- **Frontend Engineer** - UI/UX work (Gemini)
- **Oracle** - Architecture decisions (GPT 5.2)
- **Explore** - Codebase patterns (Grok)

## Quality Gates

Before each commit, the system runs:

1. **Typecheck** - `go build`, `bun typecheck`, `tsc`, or `cargo check`
2. **Tests** - `go test`, `bun test`, `npm test`, or `cargo test`
3. **Lint** - `golangci-lint`, `bun lint`, `eslint`, or `cargo clippy`

## Pattern Learning

After each successful story, Ralph-OpenCode:

1. Extracts learned patterns from the task
2. Checks if pattern already exists in AGENTS.md
3. Adds new patterns in Markdown format
4. Compacts old patterns when reaching threshold (50)

Example AGENTS.md entry:

```markdown
## Codebase Patterns

### **Use sql<number> template for aggregations**
All aggregation queries use the sql template pattern for consistency.

### **Always use IF NOT EXISTS for migrations**
Database migrations should be idempotent.
```

## Keyboard Controls

- `Ctrl+C` - Graceful shutdown (saves state)
- `p` - Pause/resume (TUI mode)

## Requirements

- Bun 1.1+
- OpenCode CLI with OhMyOpenCode plugin
- Git repository
- Type-checking and test commands available

## See Also

- [Ralph Pattern](https://ghuntley.com/ralph/) - Original Ralph pattern
- [OhMyOpenCode](https://github.com/code-yeongyu/oh-my-opencode) - Multi-agent orchestration
- [Hona/ralph-cli](https://github.com/Hona/opencode-ralph) - Alternative Ralph CLI
