# CLAI Roadmap

## 🎯 Current Status (January 2026)
CLAI is a production-capable local AI CLI agent with safe code execution and a robust TUI.

### Accomplished
- ✅ **Full-Screen TUI**: Header-Body-Footer sectional layout.
- ✅ **Safe Code Execution**: Timeouts (30s), output limits (1MB), and dangerous command detection.
- ✅ **Execution Auditing**: All tool/code executions are logged to SQLite.
- ✅ **Stable MCP Integration**: Native MCP server support.
- ✅ **Custom Theme System**: Support for multiple high-contrast terminal themes.

---

## 🚀 Future Roadmap

### Phase 1: Enhanced Discovery & UX
- **Command Discovery**: Add `:examples` and `:languages` for code execution.
- **Improved History**: `:history` command to list, load, and manage conversations.
- **Review Mode**: `:review on` to require confirmation before any code execution.
- **Syntax Highlighting**: Enhanced markdown rendering with language detection.

### Phase 2: Advanced Sandboxing (Target: Q2 2026)
- **Containerized Execution**: Move code execution into minimal Docker containers (Alpine-based).
- **Resource Limits**: Per-execution CPU and memory constraints.
- **Network Policies**: Opt-in/out for network-accessing tools.

### Phase 3: Intelligence & Memory
- **Long-term Memory**: RAG integration for indexing local project history.
- **Multi-Model Support**: Seamless switching between Ollama, Anthropic, and OpenAI.
- **Conversation Branching**: Ability to fork conversations from any point.

---

## 🐛 Known Issues & Technical Debt
- **Theme Fragmentation**: Centralize theme registry in `internal/ui/styles.go` to simplify adding new themes.
- **Test Coverage**: Target 70%+ coverage for `internal/ui` and `internal/llm`.
- **Error Context**: Improve "Connection Refused" messages to specifically mention if Ollama/llama.cpp is missing.

---

## 🎨 Design Principles
1. **Local-first**: Minimum dependencies, works offline.
2. **Safe by Design**: Audit logs and safety guardrails for all execution.
3. **Keyboard Centric**: Highly optimized for speed via hotkeys.
4. **Transparent**: Clear visibility into what the LLM is doing and what code is running.
