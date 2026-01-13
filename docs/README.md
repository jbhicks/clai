# CLAI Documentation

CLAI is a modern, local-first AI interface and agent framework for terminal power users, integrating extensible autonomous agents and a rich TUI (Bubble Tea-based) with production-grade code execution, persistence, and advanced workflow support.

---

## Quick Production Readiness Checklist

- Remove obsolete tool systems and dead code
- Add timeouts/limits to all code execution
- Centralize theme management
- Ensure test coverage for all core features
- Enhance execution UX and add sandboxing

---

## Documentation Categories

### [Performance](./performance/)
- [Background Refresh Performance Fix](./performance/BACKGROUND_REFRESH_PERFORMANCE_FIX.md): Extreme speedup by using async goroutines
- [Loading Experiment Results](./performance/LOADING_EXPERIMENT_120B_131K.md): Model performance test output

### [UI/UX](./ui/)
- [UI Guide](./ui/UI_GUIDE.md): Patterns, design, and best practices
- [UI Improvement Plan](./ui/UI_IMPROVEMENT_PLAN.md): Planned UI enhancements
- [UI Verification Complete](./ui/UI_VERIFICATION_COMPLETE.md): Testing summary
- [Color/Border Rendering Fixes](./ui/COLOR_FIX_SUMMARY.md), [./ui/BORDER_FIX_SUMMARY.md]: Technical fixes

### [Downloads](./downloads/)
- [Download Progress Display Fix](./downloads/DOWNLOAD_FIX_SUMMARY.md)
- [Download UI Fix Complete](./downloads/DOWNLOAD_UI_FIX_COMPLETE.md)
- [Download UI Fix Verified](./downloads/DOWNLOAD_UI_FIX_VERIFIED.md)
- [Download UI Testing Complete](./downloads/DOWNLOAD_UI_TESTING_COMPLETE.md)
- [Download Auto Retry Implementation](./downloads/DOWNLOAD_AUTO_RETRY_IMPLEMENTATION.md)

### [Testing](./testing/)
- [Bubble Tea Testing Strategy](./testing/BUBBLETEA_TESTING_STRATEGY.md): TUI projects
- [All Tests Fixed](./testing/ALL_TESTS_FIXED.md)
- [Model Testing Plan](./testing/MODEL_TESTING_PLAN.md)
- [Server Action Tests Complete](./testing/SERVER_ACTION_TESTS_COMPLETE.md)
- [Unit Testing Summary](./testing/UNIT_TESTING_SUMMARY.md)
- [Testing Quick Reference](./testing/TESTING_QUICK_REF.txt)

### [Development](./development/)
- [Agent Implementation](./development/AGENT_IMPLEMENTATION.md)
- [Code Parser Fix Results](./development/CODE_PARSER_FIX_RESULTS.md)
- [Dev Commands](./development/DEV_COMMANDS.md)
- [SSE Verification](./development/SSE_VERIFICATION.md)
- [HTMX Alpine Loading Integration](./development/HTMX_ALPINE_LOADING_IMPLEMENTATION.md)
- [Database Persistence Complete](./development/DATABASE_PERSISTENCE_COMPLETE.md)
- [Model Download Fix](./development/MODEL_DOWNLOAD_FIX.md)
- [Gemini & GPT OSS Integration](./development/GEMINI.md), [./development/GPT_OSS_120B_ADDITION.md]
- [LLM Code Format Research](./development/LLM_CODE_FORMAT_RESEARCH.md)
- [Systemd GPU Fix](./development/SYSTEMD_GPU_FIX_SUMMARY.md)
- [Stricks Halo Optimization](./development/STRIX_HALO_OPTIMIZATION_PLAN.md)
- [CLAI Analysis Summary](./development/CLAI_ANALYSIS_SUMMARY.md)
- [Air Setup Complete](./development/AIR_SETUP_COMPLETE.md)
- [Agent Tracking Document](./development/agent_tracking_document.md)

## Core & Reference Docs
- [AGENTS.md](../AGENTS.md) - Agent config/guidelines
- [TOOLS.md](./TOOLS.md) - Tool system documentation
- [MODEL_COMPATIBILITY.md](./MODEL_COMPATIBILITY.md) - Supported LLMs
- [HTMX_SSE_SETUP.md](./HTMX_SSE_SETUP.md) - SSE integration patterns
- [DEBUG_SERVER.md](./DEBUG_SERVER.md) - Debug server usage
- [AIR_TEMPL_SETUP.md](./AIR_TEMPL_SETUP.md) - Air template installation

## Scripts
See [scripts/](../scripts/) directory for all test, build, setup, monitor and utility scripts. All scripts are categorized by function.

## Contributing
1. Place new docs in the correct subfolder
2. Add a link to this index
3. Keep docs terse and actionable; consolidate where possible.
4. Use existing naming conventions

---