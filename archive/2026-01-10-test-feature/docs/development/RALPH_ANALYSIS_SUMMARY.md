# Feature Document: Ralph-Inspired Autonomous Agent Orchestrator for CLAI

## Epic Overview

**Transform CLAI from a single-purpose CLI tool into a sophisticated autonomous agent orchestrator** that can spawn and coordinate multiple sub-CLAI processes to collaboratively implement complex software features. This refactor will implement Ralph's proven autonomous development patterns, enabling CLAI to manage large-scale feature development through parallel agent execution while maintaining memory efficiency and quality assurance.

## Problem Statement

CLAI currently operates as a single-threaded CLI tool, limiting its ability to handle complex, multi-step software development tasks. When implementing large features, users must manually coordinate multiple CLAI invocations, track progress, and ensure quality. This approach is error-prone, time-consuming, and doesn't scale well for enterprise-level development tasks.

Ralph demonstrates that autonomous agent orchestration can reliably implement complex software features through:
- Parallel agent execution with fresh contexts
- Persistent memory systems
- Quality gates and verification
- Pattern learning and knowledge transfer

## Goals

### Primary Goals
- Enable CLAI to autonomously implement complex software features through orchestrated agent loops
- Support parallel execution of development tasks while maintaining memory efficiency
- Implement Ralph's memory persistence and quality assurance patterns
- Provide CLI interface for spawning sub-CLAI processes for single task execution

### Secondary Goals
- Enhance CLAI's existing AGENTS.md system with pattern discovery
- Integrate browser verification for UI changes using existing debug server
- Maintain backward compatibility with existing CLAI functionality
- Enable scalable development workflows for large codebases

## User Stories & Acceptance Criteria

### US-001: Implement Autonomous Agent Loop Infrastructure
**As a developer**, I want CLAI to orchestrate multiple concurrent tasks using goroutines so that complex features can be implemented autonomously.

**Acceptance Criteria:**
- [ ] CLAI can spawn goroutines with AI-analyzed minimal contexts for each task
- [ ] Main orchestrator can monitor and coordinate goroutine tasks via channels
- [ ] Goroutines inherit necessary context through channel communication
- [ ] Intelligent error handling - orchestrator analyzes failures and decides retry/skip/stop
- [ ] Context summarization - goroutine results compressed back to orchestrator's context window
- [ ] Graceful shutdown and cleanup of goroutines using context cancellation
- [ ] Goroutine lifecycle management prevents leaks
- [ ] Typecheck passes

### US-002: Add Memory Persistence System
**As an autonomous agent**, I need persistent memory across iterations so that learnings and progress are maintained without in-memory state.

**Acceptance Criteria:**
- [ ] Create `.clai/progress.json` format for structured progress logging
- [ ] Implement `.clai/stories.json` for task completion tracking
- [ ] Add `.clai/patterns.log` append-only pattern learning log
- [ ] Git history integration for immutable code changes and automatic backups
- [ ] Memory system survives process restarts
- [ ] Typecheck passes

### US-003: Implement Small Task Decomposition
**As a user**, I want to break large features into executable tasks so that CLAI can implement them incrementally without context overflow.

**Acceptance Criteria:**
- [ ] CLI command `clai decompose [feature-description]` creates task breakdown using configured model
- [ ] Tasks sized for single CLAI context window using llama.cpp-style context tracking
- [ ] Basic dependency ordering (schema → backend → UI)
- [ ] Task validation prevents oversized tasks based on context limits
- [ ] JSON export of decomposed tasks to `.clai/stories.json`
- [ ] Integration with existing task view UI (tasks list, step details below, logs pane right)
- [ ] Qwen-specific formatting and tool calling support
- [ ] Typecheck passes

### US-004: Add Quality Assurance Integration
**As a developer**, I want mandatory quality checks before commits so that broken code cannot compound across iterations.

**Acceptance Criteria:**
- [ ] Integration with existing `make test` and `make lint`
- [ ] Quality check execution per task completion with parallelization where possible
- [ ] Live UI state verification through CLAI process connection for feature testing
- [ ] Efficient UI state transfer to avoid context window overflow
- [ ] Failed checks prevent commits and mark tasks incomplete
- [ ] Allow partial commits for save states before major changes/refactors
- [ ] Never commit non-working application state
- [ ] Quality metrics logged to progress tracking
- [ ] Rollback capability for failed iterations
- [ ] Typecheck passes

### US-005: Add Pattern Discovery & AGENTS.md Enhancement
**As an autonomous agent**, I want to learn and document patterns so that future iterations benefit from discovered knowledge.

**Acceptance Criteria:**
- [ ] Enhanced AGENTS.md update system for important learnings only
- [ ] Document API/framework research findings and bug resolution insights
- [ ] Context-aware AGENTS.md file selection
- [ ] Pattern compaction system to prevent context window overflow
- [ ] Pattern search and retrieval for future tasks
- [ ] Typecheck passes

### US-006: Create Orchestrator CLI Commands
**As a user**, I want CLI commands to control autonomous execution so that I can start, monitor, and stop agent orchestrations.

**Acceptance Criteria:**
- [ ] `clai orchestrate start [stories.json]` - Begin autonomous execution
- [ ] `clai orchestrate status [--watch]` - Show current progress with optional live updates
- [ ] `clai orchestrate stop` - Gracefully halt execution
- [ ] `clai orchestrate resume [--from-failure]` - Continue from last checkpoint with failure handling
- [ ] `clai task execute [task-id] [--skip-checks]` - Execute single task manually with optional quality check bypass
- [ ] Typecheck passes

### US-007: Add Completion Detection & Exit Conditions
**As an autonomous system**, I want to detect completion and exit cleanly so that orchestration terminates reliably.

**Acceptance Criteria:**
- [ ] All tasks `completed: true` triggers completion
- [ ] `<COMPLETE>` signal in orchestrator output
- [ ] Clean shutdown of all child processes
- [ ] Final progress report generation
- [ ] Success/failure status reporting
- [ ] Typecheck passes

### US-008: Implement Archival & Feature Branching
**As a user**, I want completed orchestrations archived so that feature development history is preserved and isolated.

**Acceptance Criteria:**
- [ ] Automatic archival to `archive/YYYY-MM-DD-feature-name/` on branch changes
- [ ] Archive .clai/ files, progress logs, and generated code
- [ ] Tight git branch integration for feature isolation
- [ ] Archive restoration and browsing capabilities
- [ ] Configurable archive cleanup and compression
- [ ] Typecheck passes

### US-009: Implement Parallel Command Execution Within Tasks (New)
**As a sub-agent**, I want to execute multiple shell commands in parallel within a single task so that development operations complete faster.

**Acceptance Criteria:**
- [ ] Support for parallel command execution using goroutines and exec.Cmd
- [ ] Command dependency management using errgroup for coordination
- [ ] Context-based cancellation for failure handling
- [ ] Resource monitoring to prevent command conflicts
- [ ] Parallel execution results aggregation
- [ ] Typecheck passes

### US-010: Implement Parallel Task Execution with Memory Management (Moved from US-007)
**As a system**, I want to run multiple CLAI tasks simultaneously so that development speed is maximized without memory exhaustion.

**Acceptance Criteria:**
- [ ] Memory usage monitoring per goroutine
- [ ] Dynamic task spawning based on available RAM
- [ ] LLM server sharing between goroutines
- [ ] Task priority queuing for resource management
- [ ] Memory cleanup and optimization between iterations
- [ ] Typecheck passes

## Technical Architecture

### Core Components

#### 1. Orchestrator Engine
- **Purpose**: Main coordination logic for autonomous loops
- **Files**: `internal/orchestrator/`
- **Responsibilities**:
  - Process spawning and management
  - Memory coordination
  - Progress aggregation
  - Quality gate enforcement

#### 2. Memory Persistence Layer
- **Purpose**: Externalized state management
- **Files**: `internal/memory/`
- **Components**:
  - `progress.json` - Structured progress tracking
  - `stories.json` - Task completion states
  - `patterns.log` - Append-only learnings
  - Git integration for code history

#### 3. Task Decomposition Engine
- **Purpose**: Break features into executable units
- **Files**: `internal/decomposer/`
- **Features**:
  - Natural language feature analysis
  - Dependency graph generation
  - Context window sizing
  - Priority ordering

#### 4. Quality Assurance System
- **Purpose**: Automated testing and verification
- **Files**: `internal/quality/`
- **Integrations**:
  - Existing test suite
  - Debug server for UI verification
  - Code quality metrics

#### 5. Process Manager
- **Purpose**: Handle sub-CLAI process lifecycle
- **Files**: `internal/process/`
- **Capabilities**:
  - Safe process spawning
  - Resource monitoring
  - Inter-process communication
  - Cleanup and recovery

### Memory and Performance Architecture

#### Memory Management Strategy (Updated for Goroutine Architecture)

**Single LLM Server Sharing (RECOMMENDED - MVP):**
```
┌─────────────────┐
│   CLAI          │
│   Orchestrator  │ ←── Memory coordination
│   (Main Process)│
└────────┬────────┘
         │
    ┌────▼────┐
    │  Shared │ ←── Single LLM server instance
    │  Memory │    (Qwen-30B context)
    └────┷────┘
         │
    ┌────▼────┐
    │ Sub-Agent│ ←── Isolated goroutine contexts
    │ Goroutines│
    └─────────┘
```

**Pros:**
- **Minimal memory overhead** (no duplicate LLM servers)
- **Faster startup** (no process spawning)
- **Perfect isolation** through goroutines
- **Shared context** between agents when needed
- **Easier debugging** (single process)

**Cons:**
- Sequential LLM inference per agent (but parallel across agents)
- Single process failure affects all
- Memory shared between all agents

**Multiple LLM Server Parallelization (ADVANCED - Post-MVP):**
```
┌─────────────────┐
│   CLAI          │
│   Orchestrator  │ ←── Load balancing
│   (Main Process)│
└────────┬────────┘
         │
    ┌────▼────┐
    │  Memory │ ←── Shared state coordination
    │  Manager │
    └────┷────┘
         │
    ┌────┼────┐
    │    │    │
┌───▼──┐ └─▼─┐ └─▼─┐
│LLM   │   │  │   │  │
│Server│   │  │   │  │ ←── Parallel inference
│1     │   │  │   │  │
└──────┘   │  │   │  │
           │  │   │  │
      ┌────▼──▼───▼──▼─┐
      │   Sub-Agent     │
      │   Goroutines    │
      └────────────────┘
```

**Pros:**
- True parallel LLM inference
- Fault tolerance (one server failure doesn't stop all)
- Better multi-core utilization

**Cons:**
- High memory usage (multiple 4-8GB models)
- Complex load balancing
- Resource contention

#### Recommended Approach: Single Server for MVP

**MVP**: One shared Qwen-30B server with goroutine isolation
**Post-MVP**: Multiple servers for parallel inference when memory allows

**Memory Requirements (Goroutine Model):**

- **Base CLAI**: ~500MB RAM
- **Single Qwen-30B Server**: 4-8GB RAM
- **Per Goroutine Overhead**: ~10MB (vs 100MB for processes)
- **Total MVP**: ~5GB RAM for full orchestration

**Example Configurations:**
- **Lightweight (8GB RAM)**: 1 shared server, unlimited goroutines (memory permitting)
- **Standard (16GB RAM)**: 1 shared server, full orchestration capability
- **Heavy (32GB+ RAM)**: Multiple servers possible for parallel inference

### Process Communication Architecture (Updated for Goroutines)

#### Goroutine Communication (Simplified)
- **Go Channels**: For task handoff and result reporting (type-safe, blocking)
- **Context Cancellation**: Selective cancellation (failing agent only)
- **Shared Memory**: For progress coordination via files (simpler than IPC)

#### State Synchronization
- **File-Based Updates**: Progress changes use atomic file writes
- **Channel Coordination**: Completion signals trigger next task assignment
- **Memory Consistency**: Single process ensures state consistency

## Implementation Details

### Phase 1: Core Infrastructure (US-001, US-002, US-007)
1. Implement goroutine orchestrator with channel communication
2. Add memory persistence system (.clai/ files)
3. Create orchestrator CLI commands
4. Add goroutine lifecycle management

### Phase 2: Task Management (US-003, US-008)
1. Build task decomposition engine with Qwen integration
2. Implement dependency ordering and validation
3. Add archival system with git integration
4. Create UI integration for task display

### Phase 3: Quality & Verification (US-004, US-009)
1. Integrate quality assurance pipeline with live UI testing
2. Implement parallel command execution within tasks
3. Add rollback capabilities and partial commits
4. Add verification result tracking

### Phase 4: Intelligence Layer (US-005, US-006)
1. Enhance AGENTS.md system with pattern compaction
2. Add pattern discovery for API research and bug fixes
3. Implement completion detection with intelligent error handling
4. Create CLI orchestration controls with status monitoring

### Phase 5: Advanced Features (US-010 - Post-MVP)
1. Implement parallel task execution with memory management
2. Add dynamic task spawning based on RAM availability
3. Create resource-aware scheduling with PRD priority ordering
4. Optimize for different hardware configurations

## CLI Interface Design

### New Commands
```bash
# Task decomposition
clai decompose "Add user authentication with OAuth" --output stories.json

# Orchestration control
clai orchestrate start stories.json
clai orchestrate status [--watch]
clai orchestrate stop
clai orchestrate resume [--from-failure]

# Manual task execution
clai task execute US-001 [--skip-checks]

# Memory and pattern management
clai memory show-progress stories.json
clai memory show-patterns
clai memory archive-feature auth-feature
```

### Configuration
```json
{
  "orchestrator": {
    "memoryLimitMB": 8192,
    "qualityGates": ["typecheck", "test"],
    "autoArchive": true,
    "patternCompaction": true
  }
}
```

## Risks and Mitigations

### Technical Risks

#### Context Window Overflow
- **Risk**: Multiple agents filling Qwen's context window simultaneously
- **Mitigation**:
  - Context window monitoring and limits per agent
  - Summarization of completed agent work
  - Agent context compaction before new task assignment
  - Qwen-specific context management

#### Goroutine Leaks and Deadlocks
- **Risk**: Goroutines not properly cleaned up, causing memory leaks
- **Mitigation**:
  - Context cancellation for all goroutines
  - Goroutine lifecycle monitoring
  - Panic recovery with cleanup
  - Comprehensive testing of cancellation scenarios

#### Quality Degradation Across Iterations
- **Risk**: Poor code quality compounds without proper gates
- **Mitigation**:
  - Mandatory quality checks before commits
  - Automatic rollback on quality failures
  - Pattern validation for AGENTS.md updates
  - Partial commit capability for save states

### Operational Risks

#### Long-Running Orchestration Management
- **Risk**: Orchestrations run indefinitely or become unresponsive
- **Mitigation**:
  - Configurable timeouts and progress monitoring
  - Manual intervention commands (`clai orchestrate stop`)
  - Automatic cleanup on system signals
  - Progress heartbeat monitoring

#### Pattern Pollution and Context Bloat
- **Risk**: AGENTS.md grows too large, filling context windows
- **Mitigation**:
  - Pattern compaction system to reduce file size
  - Selective pattern updates (only high-value learnings)
  - AGENTS.md size monitoring and cleanup
  - Confidence-based pattern inclusion

#### Single Process Failure Point
- **Risk**: One failing agent can affect the entire orchestration
- **Mitigation**:
  - Isolated goroutine failures don't crash main process
  - Intelligent error handling (retry/skip/stop decisions)
  - Graceful degradation when agents fail
  - Process restart capability for recovery

## Success Metrics

### Quantitative Metrics
- **Completion Rate**: Percentage of orchestrated features completed successfully
- **Quality Score**: Average code quality metrics (test pass rate, lint score)
- **Performance**: Time to complete web app features from PRD overnight
- **Memory Efficiency**: Peak RAM usage within 5GB limit
- **Context Management**: Effective context window usage without overflow

### Qualitative Metrics
- **Developer Experience**: Ease of use for orchestration features
- **Reliability**: Frequency of orchestration failures requiring intervention
- **Knowledge Transfer**: Effectiveness of pattern learning across features
- **Code Quality**: Reduction in post-orchestration refactoring needs
- **Overnight Success**: Ability to wake up to working web applications

### Target Benchmarks
- **95%** feature completion rate
- **90%** test pass rate on orchestrated code
- **Overnight web app completion** from well-defined PRD
- **<5GB** peak RAM usage for full orchestration
- **Zero** context window overflows
- **Zero** unrecoverable orchestration failures

## Migration Strategy

### Backward Compatibility
- All existing CLAI functionality preserved
- New commands are opt-in
- Existing workflows continue to work
- Gradual adoption path

### Rollout Phases
Each phase requires full unit testing and integration testing before advancement:

1. **Alpha**: Core orchestration with sequential goroutines (Implementation Phases 1-2)
2. **Beta**: Quality assurance and intelligence layer (Implementation Phases 3-4)
3. **GA**: Full orchestration capability with overnight web app completion
4. **Enterprise**: Parallel task execution and advanced memory management (Implementation Phase 5)

### Training and Documentation
- Updated AGENTS.md with orchestration patterns
- CLI help system enhancements
- Example orchestrations for common use cases
- Troubleshooting guides for orchestration issues

## Conclusion

This refactor transforms CLAI from a single-purpose tool into a sophisticated autonomous development platform. By implementing Ralph's proven patterns with CLAI-specific optimizations, we create a system that can reliably implement complex software features through orchestrated agent collaboration.

The hybrid memory approach balances performance with resource efficiency, enabling parallel execution when beneficial while maintaining reliability. The comprehensive quality assurance and pattern learning systems ensure that orchestrated development maintains high code quality and improves over time.

This foundation enables CLAI to scale from simple code changes to enterprise-level feature development, making autonomous software development accessible to developers at any level.</content>
<parameter name="filePath">RALPH_ANALYSIS_SUMMARY.md