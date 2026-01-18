# CLAI Roadmap: The Ralph Orchestrator

## 👁️ Vision
Transform CLAI from a chat interface into a **Bubble Tea/Lipgloss Autonomous Orchestrator** based on the **Ralph Method**. 

CLAI will be the "Mission Control" for AI-driven development, where a main TUI loop manages high-level PRDs (Product Requirements Documents), visualizes progress, and spawns concurrent sub-agent threads to execute specific user stories.

---

## 🛠️ Phase 1: Ralph Core (TUI & Data Foundations)
*   **PRD Specification Engine**: Implement native support for `prd.json`.
    *   Dynamic parsing and rendering of User Stories.
    *   Interactive TUI view to prioritize, add, or edit stories.
*   **Stateful Progress Tracking**:
    *   Persist "Passed/Pending/Blocked" status per story.
    *   Visualize the "Iteration Loop" state (which story is active, total passes).
*   **Ralph-Style Context Management**:
    *   Maintain `progress.txt` as a persistent, append-only memory for the project.
    *   Automated updates to `AGENTS.md` based on sub-agent learnings.

## 🤖 Phase 2: The Go Ralph Loop (Concurrency & sub-agents)
*   **Main Loop Orchestrator**: The Bubble Tea `Update()` function acts as the Ralph Loop.
    *   Detects completion of stories and automatically selects the next highest priority task.
*   **Concurrent Worker Threads**:
    *   Kick off sub-agent operations as background Go goroutines.
    *   Sub-agents execute a single story, run Go tests, and verify builds before reporting back.
*   **Sub-agent Multiplexing**:
    *   Side-pane in TUI for real-time sub-agent "thought streams" (logs/stdout).
    *   Thread-safe state updates back to the main TUI model.

## 🛡️ Phase 3: Verification & Safeguards
*   **Automated Quality Gates**:
    *   Sub-agents must pass `go test ./...` and `go build` before a story is marked as `passed`.
    *   Integrated `clai-debug` MCP tools for sub-agent self-verification.
*   **Git Automation**:
    *   Automated feature branch management (`ralph/feature-name`).
    *   Structured commits per story: `feat: [Story ID] - [Story Title]`.
*   **Review Mode**:
    *   Optional human-in-the-loop "Checkpoints" where the orchestrator pauses for user approval.

## 🚀 Phase 4: Intelligence & Parallelism
*   **Multi-Agent Coordination**:
    *   Spawning parallel sub-agents for independent user stories.
    *   Managing dependency graphs between stories.
*   **Dynamic Re-Planning**:
    *   Ability for the orchestrator to detect if a PRD needs adjustment based on implementation discoveries and update `prd.json` autonomously.

---

## 🎨 TUI Design Principles (Ralph Focus)
1.  **High-Visibility Status**: The current task and its progress should be the center of the UI.
2.  **Pane-Based Layout**: 
    *   **Left**: PRD/Story List (The Mission).
    *   **Right (Top)**: Main Orchestrator Chat/Logs.
    *   **Right (Bottom)**: Active Sub-agent stream (The Work).
3.  **Low Latency**: Use Go's concurrency to keep the UI responsive while heavy LLM operations occur in the background.
4.  **Auditability**: Every sub-agent decision must be logged to the project's persistent memory.

---

## 🧩 Reference Implementation
Refer to `docs/reference/library_reference/ralph` for the original bash-based Ralph patterns. CLAI's goal is to modernize this pattern into a robust, concurrent Go application.
