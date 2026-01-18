# CLAI - Project Status & Development Roadmap

**Last Updated**: 2025-11-28  
**Project Type**: Local AI CLI Agent (Go + Bubble Tea TUI)  
**Primary Purpose**: Conversational AI interface with code execution capabilities

---

## 📊 Project Health Snapshot

### Codebase Size
- **Total Lines**: ~3,130 LOC (excluding tests/reference docs)
  - `cmd/clai`: 184 LOC (entry point)
  - `internal/db`: 229 LOC (SQLite persistence)
  - `internal/llm`: ~1,057 LOC (Ollama client, streaming, tool call parsing)
  - `internal/tools`: 247 LOC (code execution, legacy tool system)
  - `internal/ui`: ~1,413 LOC (Bubble Tea TUI)

### Test Coverage
- **10 test files** across packages
- Tests exist for: LLM streaming, formatting, tool parsing, UI components, layout, integration
- **Known gap**: `internal/ui/model_test.go:163` - TODO comment indicates incomplete test
- **Estimate**: ~50-60% coverage (full report needed)

### Architecture Quality
- ✅ Well-structured: Clear separation between LLM, UI, DB, and tools
- ✅ Modern UI: Bubble Tea + Bubbles + Lip Gloss
- ✅ Good patterns: Mock infrastructure exists for testing
- ⚠️ Code execution is primary tool mechanism (not standard tool calling)
- ⚠️ Theme system works but implementation is fragmented

---

## 🎯 Current State Assessment

### What Works Well
1. **Core Chat Flow**
   - Streaming responses from Ollama
   - Multi-message conversations
   - SQLite persistence with conversation history
   - Markdown rendering with Glamour

2. **Code Execution System**
   - Bash, Python, Node.js code execution
   - XML `<code language="...">` block parsing
   - Proper streaming detection and execution
   - Results returned to LLM context

3. **UI Components**
   - Two-pane layout (chat + logs)
   - Theme switching (Ctrl+D)
   - Help modal (Ctrl+H)
   - Error banner display
   - Status bar with model/host/format info
   - Focus management between panes
   - Auto-scroll to bottom on new messages (chat viewport)

4. **Tool Call Parsing**
   - Text-based tool call parsing (for Qwen models)
   - OpenAI structured format support
   - Tag stripping in UI to hide XML artifacts

5. **Testing Infrastructure**
   - Comprehensive testing strategy documented (`BUBBLETEA_TESTING_STRATEGY.md`)
   - Mock LLM and executor implementations
   - Unit, integration, and layout tests
   - Test helpers for ANSI stripping and assertions

### Known Issues & Gaps

#### 1. **Theme System Fragmentation** 🔴 **HIGH PRIORITY**
**Problem**: Theme switching works (Glamour, styles update), but implementation is scattered and inconsistent.

**Evidence**:
- Theme definitions in `styles.go` (9 themes: Dracula, Tokyo Night, Nord, Gruvbox, Monokai, Solarized Dark, Catppuccin Mocha, OneBit, Custom)
- `AvailableThemes` array and `ThemeNames` array must stay in sync manually
- `GetThemeName()` function does string lookup (lines 189-194)
- `GetGlamourStyleName()` maps themes to Glamour styles (lines 252-267)
- Theme stored in DB as string, but used as `*glitter.UI` pointer in code
- No validation that DB theme name matches available themes

**Impact**:
- Hard to add new themes (requires edits in 3+ places)
- Brittle: Arrays can get out of sync
- No clear "theme registry" or central definition
- User reports "some things aren't changing much on theme switch" (likely because some UI elements don't respect theme or themes look similar)

**Root Cause**:
- Missing centralized theme management system
- No theme interface or registry pattern
- Manual mapping between multiple representations (string name, glitter.UI pointer, Glamour style name)

#### 2. **Code Execution Safety & Cleanup** 🔴 **HIGH PRIORITY**
**Problem**: Code execution lacks safety guardrails, and legacy tool system creates confusion.

**Evidence**:
- Code execution has no sandboxing (runs with user permissions)
- No timeout or resource limits on code execution
- No output size limits (can hang terminal)
- Legacy tool system exists in `internal/tools/tools.go` (unused, confusing)
- Legacy XML `<tool_call>` parsing still active in codebase
- Security warning in `AGENTS.md`: "avoid destructive operations without user confirmation"

**Impact**:
- 🔴 Security risk (arbitrary code execution without limits)
- 🔴 UX risk (can hang forever, crash terminal)
- 🟡 Code complexity (two tool systems)
- 🟡 Maintenance burden (unused code paths)

**Decision**: **Embrace code execution as core feature, remove legacy tools**
- ✅ Flexible: Can do anything code can do
- ✅ Simple: One clear execution model
- ✅ Differentiating: Unique value proposition vs. competitors

#### 3. **Incomplete Test Suite** 🟡 **MEDIUM PRIORITY**
**Problem**: Tests exist but coverage is incomplete.

**Evidence**:
- TODO comment in `model_test.go:163`
- No coverage report in CI/CD
- Testing strategy doc exists but not fully implemented
- No visual regression tests (`visual_test.go` doesn't exist)
- Integration test exists but may not cover all flows

**Impact**:
- Regressions can slip through
- Hard to refactor with confidence
- UI changes might break unexpectedly

#### 4. **Development Workflow Complexity** 🟢 **LOW PRIORITY**
**Problem**: Multiple dev workflow docs/scripts with inconsistent instructions.

**Evidence**:
- `README.md` mentions tmux, entr, `make dev`
- `AGENTS.md` says "ASSUME the user is running make dev"
- Log files: `debug.log`, `run_output.log`
- Multiple scripts: `dev.sh`, references to `dev_entr.sh`, `dev_watch.sh`, `dev_run.sh`

**Impact**:
- Confusing for new contributors
- Easy to run wrong command and get unexpected results

#### 5. **Legacy Tool System Removal** 🟡 **MEDIUM PRIORITY**
**Problem**: Old XML `<tool_call>` system exists alongside code execution, creating confusion.

**Evidence**:
- `internal/tools/tools.go` defines unused tool schemas (`calculator`, `echo`, `web_search`)
- Text-based tool call parser exists for Qwen models (lines 836-898 in llm.go)
- `stripToolCallTags()` functions in model.go and llm.go
- Test files reference `<tool_call>` format (toolparse_test.go, integration_test.go)
- 29 references to `tool_call` across codebase

**Impact**:
- 🔴 Code complexity (two systems, ~400 LOC of unused code)
- 🔴 Confusion about which system is active
- 🟡 Maintenance burden (tests for unused features)
- 🟡 Dead code paths in production

**Action**: Remove legacy tool system entirely (see Phase 2 below)

---

## 🗺️ Recommended Path Forward

### Phase 1: Stabilize Core (Immediate - 1-2 weeks)

#### 1.1 Centralize Theme Management 🔴 **CRITICAL**
**Goal**: Make theme system maintainable, extensible, discoverable.

**Tasks**:
- [ ] Create `Theme` struct with fields: `Name string`, `GlitterUI *glitter.UI`, `GlamourStyle string`
- [ ] Build central theme registry: `var ThemeRegistry = []Theme{...}`
- [ ] Replace `AvailableThemes` + `ThemeNames` + `GetThemeName()` + `GetGlamourStyleName()` with registry lookups
- [ ] Add `GetThemeByName(name string) (*Theme, error)` with validation
- [ ] Add `ListThemes() []string` for UI display
- [ ] Update DB schema to validate theme names on load
- [ ] Add `theme add` command for custom themes (optional)
- [ ] **Success criteria**: Adding a theme requires editing only the registry definition

**Files to change**:
- `internal/ui/styles.go` - Create registry, refactor theme functions
- `internal/ui/model.go` - Use registry for theme lookups
- `internal/ui/chat.go` - Use registry for Glamour style
- `internal/db/db.go` - Add validation on theme load

**Estimated effort**: 4-6 hours

#### 1.2 Improve Test Coverage 🟡 **HIGH**
**Goal**: Reach 70%+ coverage on core packages.

**Tasks**:
- [ ] Fix TODO in `model_test.go:163`
- [ ] Add missing tests for theme switching flow
- [ ] Add tests for code execution with timeout/errors
- [ ] Run `go test -coverprofile=coverage.out ./...` and analyze gaps
- [ ] Add tests for edge cases: empty conversations, malformed DB data, network errors
- [ ] Document coverage in README or CI badge

**Estimated effort**: 6-8 hours

#### 1.3 Code Execution Safety 🔴 **CRITICAL**
**Goal**: Add safety guardrails to code execution BEFORE production use.

**Tasks**:
- [ ] Add configurable timeout (default: 30s, max: 5min) to `ExecuteCode()`
- [ ] Add output size limit (default: 1MB, truncate with warning)
- [ ] Add execution count rate limiting (max 10/min per conversation)
- [ ] Add confirmation prompt for dangerous patterns:
  - `rm -rf` (recursive delete)
  - `sudo` (privilege escalation)
  - `dd` (disk operations)
  - `mkfs` (filesystem operations)
  - `:(){ :|:& };:` (fork bombs)
- [ ] Add "dangerous command detection" regex patterns
- [ ] Add execution audit log to DB (time, language, code hash, result, error)
- [ ] Add `:safe-mode on/off` command to toggle confirmations
- [ ] Document security model in README and SECURITY.md

**Files to change**:
- `internal/tools/code_executor.go` - Add timeout, size limits, dangerous pattern detection
- `internal/ui/model.go` - Add confirmation prompts, safe mode state
- `internal/db/db.go` - Add `execution_logs` table with migration

**Estimated effort**: 6-8 hours

#### 1.4 Documentation Audit 🟢 **MEDIUM**
**Goal**: Clean up conflicting docs, ensure accuracy.

**Tasks**:
- [ ] Merge `UI_GUIDE.md` and `UI_IMPROVEMENT_PLAN.md` into single doc (UI_IMPROVEMENT_PLAN is done, UI_GUIDE is reference material - keep both but link them)
- [ ] Update README with clear "Getting Started" section
- [ ] Document all keybindings in one place (Ctrl+Q, Ctrl+H, Ctrl+D, Ctrl+T, Ctrl+N)
- [ ] Add architecture diagram (draw.io or mermaid)
- [ ] Update `AGENTS.md` with current dev workflow (clarify `make dev` setup)
- [ ] Create `CONTRIBUTING.md` with testing requirements and code style

**Estimated effort**: 2-3 hours

---

### Phase 2: Code Execution Focus (Short-term - 2-4 weeks)

#### 2.0 Remove Legacy Tool System 🔴 **HIGH PRIORITY**
**Goal**: Delete unused tool calling code, simplify architecture.

**Tasks**:
- [ ] Delete `internal/tools/tools.go` (calculator, echo, web_search schemas)
- [ ] Remove `parseTextBasedToolCalls()` from `internal/llm/llm.go` (lines 836-898)
- [ ] Remove `stripToolCallTags()` from `internal/llm/llm.go` and `internal/ui/model.go`
- [ ] Remove `ToolCallFormat` enum values for `Text` and `Structured` (keep only `Auto` or remove entirely)
- [ ] Remove tool call tests from `internal/llm/toolparse_test.go` (keep only code execution tests if any)
- [ ] Remove tool call references from `internal/ui/integration_test.go`
- [ ] Update `buildToolCallSystemPrompt()` to remove format-specific instructions
- [ ] Update `AGENTS.md` to remove "Legacy Tool System" section
- [ ] Search codebase for remaining `tool_call` references and clean up

**Files to delete**:
- `internal/tools/tools.go` (105 lines)

**Files to modify**:
- `internal/llm/llm.go` - Remove parsing functions, simplify prompt building
- `internal/ui/model.go` - Remove tag stripping
- `internal/llm/toolparse_test.go` - Remove or rewrite tests
- `internal/ui/integration_test.go` - Remove tool call test cases
- `AGENTS.md` - Update documentation

**Verification**:
- [ ] `rg "tool_call" --type go` returns 0 results (except comments)
- [ ] `rg "GetAvailableTools|GetToolsByNames" --type go` returns 0 results
- [ ] All tests pass after removal
- [ ] Binary size reduction (estimate: 50KB)

**Estimated effort**: 3-4 hours

---

### Phase 2.1: Enhance UX (Short-term - 2-4 weeks)

#### 2.2 Code Execution Discoverability 🔴 **HIGH PRIORITY**
**Goal**: Help users discover what code execution can do.

**Tasks**:
- [ ] Add `:examples` command to show code execution examples by category:
  - File operations (search, read, write)
  - Web requests (curl, python requests)
  - System info (uname, df, ps)
  - Data processing (jq, awk, sed)
  - Git operations (status, log, diff)
  - Package management (apt, pip, npm)
- [ ] Add code templates: `:template search-files`, `:template fetch-url`, etc.
- [ ] Add `:languages` command to show available interpreters + versions
- [ ] Show execution hints in status bar ("Tip: Use <code language='bash'>...")
- [ ] Add example gallery to README with GIFs/screenshots
- [ ] Create "Code Execution Cookbook" doc with 20+ recipes

**Files to change**:
- `internal/ui/model.go` - Add new commands
- `internal/tools/code_executor.go` - Add template registry
- `docs/COOKBOOK.md` - New file with examples
- `README.md` - Add examples section

**Estimated effort**: 4-6 hours

#### 2.3 Code Execution UX Enhancements
**Goal**: Make code execution visible, debuggable, and controllable.

**Tasks**:
- [ ] Show code execution in progress:
  - Spinner with language icon (⚙ bash, 🐍 python, 📦 node)
  - Display truncated code snippet (first 50 chars)
  - Show elapsed time during execution
- [ ] Add execution result styling:
  - Success: green badge with ✓ and execution time
  - Error: red badge with ✗ and error message
  - Timeout: yellow badge with ⏱ and timeout message
- [ ] Add "Review before execution" mode (`:review on`):
  - Show code in modal with syntax highlighting
  - Prompt: [E]xecute / [S]kip / [C]ancel
  - Remember choice per session
- [ ] Add execution metadata display:
  - Language and interpreter version
  - Exit code (if non-zero)
  - Execution time (ms)
  - Output size (bytes/KB/MB)
- [ ] Add `:exec <lang> <code>` command for manual code execution
- [ ] Add `:last-exec` command to show last execution details
- [ ] Add execution history in log pane (language, time, status)

**Files to change**:
- `internal/ui/chat.go` - Add execution indicators, result badges
- `internal/ui/model.go` - Add review mode, last-exec command
- `internal/tools/code_executor.go` - Return execution metadata
- `internal/ui/styles.go` - Add execution result styles

**Estimated effort**: 6-8 hours

#### 2.4 Theme Discovery & Customization
- [ ] Add `:theme list` command to show all available themes
- [ ] Add `:theme preview <name>` to show theme colors/style
- [ ] Add `:theme export <name>` to save current theme config
- [ ] Add `:theme import <file>` to load custom theme
- [ ] Show theme name in UI (not just status bar)

#### 2.5 Markdown Rendering Improvements
- [ ] Add syntax highlighting language detection for code blocks
- [ ] Add copy-to-clipboard for code blocks (requires X11/clipboard support)
- [ ] Add horizontal scrolling for wide code blocks
- [ ] Test Glamour rendering with complex markdown (tables, nested lists, images)

#### 2.6 Conversation Management
- [ ] Add `:history` command to list past conversations
- [ ] Add `:load <id>` command to switch conversations
- [ ] Add `:delete <id>` command to remove conversation
- [ ] Add `:export <id>` to save conversation as markdown/JSON
- [ ] Show conversation title in UI (editable with `:rename`)

#### 2.7 Better Error Handling
- [ ] Distinguish error types: network, LLM, parse, DB, execution
- [ ] Show actionable error messages (e.g., "Ollama not running - run `ollama serve`")
- [ ] Add retry button in error banner
- [ ] Log errors with context (conversation ID, timestamp, stack trace)



---

### Phase 3: Advanced Code Execution (Medium-term - 1-2 months)

#### 3.0 Advanced Sandboxing & Security 🔴 **CRITICAL FOR PRODUCTION**
**Goal**: Production-grade isolation for code execution.

**Option 1: Docker Containers** (recommended)
- [ ] Create minimal execution containers (alpine-based, 10-50MB each):
  - `clai-bash`: bash + curl + jq + common CLI tools
  - `clai-python`: python3 + requests + common packages
  - `clai-node`: node + npm + common packages
- [ ] Add container resource limits:
  - CPU: 1 core max
  - Memory: 512MB max
  - Disk: 100MB temp storage
  - Network: optional (configurable)
  - Time: 30s max (enforced by container)
- [ ] Mount read-only filesystem for user code
- [ ] Add network policies (allow/deny lists for domains)
- [ ] Container cleanup after execution
- [ ] Add `:sandbox on/off` command to toggle (default: on)

**Option 2: gVisor/Firecracker** (more complex)
- Lightweight VM-based isolation
- Better performance than Docker
- More setup complexity

**Option 3: WebAssembly** (future)
- True sandboxing at interpreter level
- No container overhead
- Limited language support currently

**Files to change**:
- `internal/tools/code_executor.go` - Add container execution mode
- `internal/tools/sandbox/` - New package for container management
- `Dockerfile.executor` - Executor container images
- `docker-compose.yml` - Local development setup
- `README.md` - Document container requirements

**Estimated effort**: 12-16 hours (Option 1), 20+ hours (Option 2)

#### 3.1 Resource Monitoring & Limits
**Goal**: Track and limit resource usage per execution and per conversation.

**Tasks**:
- [ ] Track execution metrics:
  - Total executions per conversation
  - Total CPU time consumed
  - Total memory used
  - Total disk I/O
  - Total network bytes transferred
- [ ] Add conversation-level limits:
  - Max 100 executions per hour
  - Max 5 minutes total CPU time per hour
  - Max 100MB total output per hour
- [ ] Show resource usage in status bar or `:stats` command
- [ ] Add warning when approaching limits
- [ ] Reset limits per conversation or time window

**Files to change**:
- `internal/tools/code_executor.go` - Collect metrics
- `internal/db/db.go` - Store execution stats
- `internal/ui/model.go` - Display stats, enforce limits

**Estimated effort**: 6-8 hours

### Phase 3.2: Advanced Features (Medium-term - 1-2 months)

#### 3.1 Tool System Redesign
**Decision**: **Option B - Code Execution Only** ✅

**Lean into code execution as the defining feature:**
- Remove legacy pre-defined tools (`calculator`, `echo`, `web_search`)
- Remove legacy XML `<tool_call>` format parsing
- Market as "AI with full command-line access"
- Focus on making code execution safe, fast, and discoverable

**Why Option B:**
- Simplifies codebase (remove dual tool systems)
- More flexible than structured tools
- Differentiates from competitors
- User can accomplish anything via code
- Reduces maintenance burden

**Implementation phases:**
1. **Phase 1**: Add safety guardrails (timeouts, sandboxing, limits)
2. **Phase 2**: Remove legacy tool code
3. **Phase 3**: Enhance discoverability (examples, autocomplete, templates)
4. **Phase 4**: Add advanced sandboxing (containers, resource limits)

#### 3.3 Multi-Model Support
- [ ] Support multiple LLM providers (Ollama, OpenAI, Anthropic, local models)
- [ ] Add model switching: `:model list`, `:model use <name>`
- [ ] Store model choice per conversation
- [ ] Add model capabilities detection (tools, vision, function calling)
- [ ] Show model info in status bar

#### 3.4 Plugin System (Code-Based)
- [ ] Define plugin interface (Go plugins or Lua scripts)
- [ ] Add plugin discovery (`~/.clai/plugins/`)
- [ ] Add `:plugin list`, `:plugin enable`, `:plugin disable` commands
- [ ] Example plugins: weather, calculator, web search, git helper
- [ ] Document plugin development

#### 3.5 Advanced UI Features
- [ ] Tabs for multiple conversations
- [ ] Vim-style keybindings option
- [ ] Fuzzy search in conversation history
- [ ] Message editing (press `e` on message to edit and resubmit)
- [ ] Branch conversations (fork at any message)
- [ ] Syntax-highlighted system prompts

#### 3.6 Collaboration & Sync
- [ ] Export/import conversation database
- [ ] Sync conversations to remote (S3, GitHub Gist, custom server)
- [ ] Share conversation by URL (read-only view)
- [ ] Collaborative chat (multiplayer mode - stretch goal)

---

### Phase 4: Production Readiness (Long-term - 2-3 months)

#### 4.1 Performance Optimization
- [ ] Profile CPU/memory usage with `pprof`
- [ ] Optimize Glamour rendering (cache rendered markdown)
- [ ] Lazy load conversation history
- [ ] Paginate message rendering (viewport only renders visible messages)
- [ ] Database query optimization (add indexes, analyze slow queries)

#### 4.2 Packaging & Distribution
- [ ] Create release process (GitHub Actions, semantic versioning)
- [ ] Build binaries for Linux, macOS, Windows
- [ ] Publish to package managers (Homebrew, apt, AUR, Scoop)
- [ ] Create Docker image
- [ ] Add update checker (`:update` command)

#### 4.3 Configuration Management
- [ ] Create config file (`~/.clai/config.yaml` or `~/.config/clai/config.yaml`)
- [ ] Support env vars (`CLAI_MODEL`, `CLAI_THEME`, etc.)
- [ ] Add `:config` command to view/edit settings
- [ ] Config schema validation
- [ ] Migrate existing hardcoded settings

#### 4.4 Monitoring & Analytics (Optional)
- [ ] Add telemetry (opt-in, privacy-focused)
- [ ] Track: feature usage, model performance, error rates, response times
- [ ] Send to local server or opt-in remote
- [ ] Show stats: `:stats` command (conversations, messages, execution time, tokens used)

#### 4.5 Security Audit
- [ ] Review code execution security (especially bash commands)
- [ ] Add input sanitization for DB queries (prepared statements)
- [ ] Review file system access (prevent path traversal)
- [ ] Add rate limiting for LLM calls (prevent runaway costs)
- [ ] Document threat model and mitigations

---

## 🔥 High-Impact Quick Wins (Do These First!)

### Week 1: Safety & Cleanup (Critical Path) ✅ COMPLETE
1. ✅ **Add Execution Timeout** (2h) - 30s default, 5min max via `ExecuteCodeWithTimeout()`
2. ✅ **Add Output Size Limits** (1h) - 1MB max with truncation warning
3. ✅ **Add Dangerous Command Detection** (2h) - Regex patterns for `rm -rf`, `mkfs`, etc.
4. ✅ **Remove Legacy Tool System** (3-4h) - ~400 LOC removed, code execution is primary mechanism
5. ✅ **Add Execution Audit Log** (2h) - `execution_logs` table fully wired in `code_executor.go`

**Subtotal**: ✅ ~10-11 hours complete - **Production-ready safety achieved**

### Week 2: Core UX (High Value)
6. **Theme Registry** (4-6h) - 🟡 HIGH - Fixes fragmentation, makes system extensible
7. **Fix Model Test TODO** (1h) - 🟡 MEDIUM - Low-hanging fruit, improves test suite
8. **Document Keybindings** (1h) - 🟡 MEDIUM - Huge UX improvement for new users
9. **Add `:history` Command** (2-3h) - 🟡 MEDIUM - Makes conversation management usable
10. **Add `:examples` Command** (2h) - 🟡 MEDIUM - Helps users discover capabilities

**Subtotal**: ~10-13 hours for **massive UX improvement**

**Total time**: ~20-24 hours across 2 weeks for production-ready, safe, user-friendly release.

---

## 🧪 Testing Strategy (Per BUBBLETEA_TESTING_STRATEGY.md)

### Testing Priorities
1. **Unit tests** (fast, isolated) - PRIMARY defense
2. **Integration tests** (full program, mocked I/O) - Catch interaction bugs
3. **Visual regression tests** (golden files) - Optional, for layout verification

### Coverage Goals
- **Minimum**: 70% for `internal/ui`, `internal/llm`
- **Target**: 85% for `internal/ui`, `internal/llm`
- **Current**: ~50-60% (estimated, needs measurement)

### CI/CD Integration
- [ ] Add GitHub Actions workflow for tests
- [ ] Run tests on every PR
- [ ] Check coverage and fail if below 70%
- [ ] Run `go test -race` to catch race conditions

---

## 📝 Technical Debt

### High Priority
- [ ] Theme system refactor (fragmented, hard to maintain)
- [ ] Code execution safety (no timeout, no limits)
- [ ] Test coverage gaps (missing tests for critical flows)

### Medium Priority
- [ ] Legacy tool system removal/clarification (two systems confusing)
- [ ] Inconsistent error handling (mix of logs, UI banners, silent failures)
- [ ] Database migrations not versioned (schema changes are ad-hoc)

### Low Priority
- [ ] Multiple dev workflow scripts (consolidate or document)
- [ ] Hardcoded strings (move to constants or config)
- [ ] No logging levels (everything is log.Printf, no DEBUG/INFO/WARN/ERROR)

---

## 🎨 Design Principles

1. **Local-first**: No cloud dependencies, works offline
2. **Fast by default**: Streaming responses, instant UI updates
3. **Hackable**: Plain Go, standard libraries, easy to fork/modify
4. **Powerful UX**: Keyboard-driven, composable commands, scriptable
5. **Safe**: Confirmations for dangerous operations, audit logs, sandboxing

---

## 🤝 Contributing

### Current Contribution Needs
1. **Testing**: Write unit/integration tests for uncovered code
2. **Documentation**: Improve getting started guide, add examples
3. **Themes**: Create and submit new theme definitions
4. **Bug reports**: Use the tool, report issues, suggest improvements
5. **Security review**: Audit code execution and input handling

### How to Get Started
1. Read `AGENTS.md` for development workflow
2. Read `BUBBLETEA_TESTING_STRATEGY.md` for testing guidelines
3. Run `make dev` to start live reload environment
4. Pick an issue from "High-Impact Quick Wins" above
5. Submit PR with tests

---

## 📚 Key References

- **Architecture**: `AGENTS.md` - Build, test, dev workflow, code execution system
- **UI Patterns**: `UI_GUIDE.md` - Bubble Tea layout, sizing, Lip Gloss usage
- **Testing**: `BUBBLETEA_TESTING_STRATEGY.md` - Comprehensive testing guide
- **UI Progress**: `UI_IMPROVEMENT_PLAN.md` - Feature checklist (mostly complete)
- **Model Compatibility**: `docs/MODEL_COMPATIBILITY.md` - LLM model testing results
- **Tool Docs**: `docs/TOOLS.md` - Tool system documentation

---

## 🎯 Success Metrics

### Short-term (1 month)
- [ ] Test coverage ≥70%
- [ ] Zero high-priority bugs
- [ ] Theme system refactored
- [ ] Code execution has timeout + limits
- [ ] 5+ themes available
- [ ] Documentation complete

### Medium-term (3 months)
- [ ] 100+ GitHub stars
- [ ] 5+ external contributors
- [ ] Multi-model support working
- [ ] Plugin system MVP
- [ ] Published to Homebrew

### Long-term (6 months)
- [ ] 500+ GitHub stars
- [ ] Featured in "awesome" lists
- [ ] Used in production by >100 users
- [ ] CI/CD with automated releases
- [ ] Comprehensive tool library (20+ tools)

---

## 🐛 Known Bugs

### Critical
- None currently identified

### High
- [ ] Model test TODO (`internal/ui/model_test.go:163`) - test incomplete

### Medium
- [ ] User reports "some things aren't changing much on theme switch" - unclear which elements
- [ ] Code execution has no timeout (can hang forever)
- [ ] Error messages sometimes unclear (e.g., "connection refused" with no context)

### Low
- [ ] Log file grows unbounded (`debug.log` not rotated)
- [ ] No confirmation for destructive code operations
- [ ] Status bar sometimes flickers on rapid resizes

---

## 💡 Future Ideas (Backlog)

- Voice input/output (TTS/STT integration)
- Image support (display images in terminal with iTerm2/Kitty protocols)
- Web UI companion (browser-based viewer for conversations)
- Mobile app (read-only conversation viewer)
- AI code review mode (specialized prompt + tools for code analysis)
- Notebook mode (mix markdown, code, and AI responses like Jupyter)
- Agent workflows (multi-step autonomous task execution)
- Memory system (long-term context beyond conversation)
- RAG integration (embed + search knowledge base)

---

## 🏁 Conclusion

**CLAI is a solid foundation** with ~3k LOC of well-structured Go code, modern TUI, and unique code-execution-as-tools approach. The core works, tests exist, and the UI is polished.

**Biggest gaps**:
1. 🔴 Code execution safety - NO TIMEOUTS OR LIMITS (fixable in ~10h)
2. 🔴 Legacy tool system confusion - dual systems, dead code (fixable in 3-4h)  
3. 🟡 Theme system fragmentation (fixable in 4-6h)
4. 🟡 Test coverage (needs ongoing work, target 70%+)

**Strategic direction**: **Lean into code execution as differentiating feature**
- Remove legacy tool system entirely
- Add production-grade safety (timeouts, sandboxing, limits)
- Enhance discoverability (examples, templates, cookbook)
- Market as "AI with full terminal access"

**Recommended next steps**:
1. **Week 1**: Execute "Safety & Cleanup" quick wins (~10-11h) - CRITICAL
2. **Week 2**: Execute "Core UX" quick wins (~10-13h) - HIGH VALUE
3. **Week 3-4**: Phase 2 (code execution focus) - enhance UX
4. **Month 2-3**: Phase 3 (Docker sandboxing, resource monitoring) - production hardening

**This is a viable product** ready for users with clear path to production-ready.

---

**Questions? Feedback? Suggestions?**  
Open an issue or PR at: https://github.com/yourusername/clai
