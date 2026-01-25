# Benchmark Maintenance Guide

This guide documents the process for maintaining CLAI's benchmark test suite, including updating test expectations when the codebase changes and developing new benchmark tests.

## Overview

CLAI uses a unified benchmark suite with 29 tests (12 model + 17 agentic + 5 advanced) to validate AI model performance and tool execution capabilities. Benchmarks must be updated when:

- Codebase structure changes (new/deleted files)
- Test data files are modified
- Expected behaviors evolve
- New functionality requires validation

## When to Update Test Expectations

### File Count Changes
Update expectations when `.go` files are added/removed from directories referenced in tests.

**Example:** Test 2 "Count Files by Extension"
```go
{
    Name: "Count Files by Extension",
    Query: "How many .go files are in /home/josh/clai/internal/llm directory? Give me just the number.",
    ShouldContain: []string{"20"}, // DYNAMIC: Update when .go files are added/removed from internal/llm
}
```

**Maintenance Process:**
1. Count current `.go` files: `find internal/llm -name "*.go" | wc -l`
2. Update `ShouldContain` array with new count
3. Add comment indicating this is dynamic

### JSON/Data File Changes
Update expectations when test data files (`internal/llm/test_data.json`, `internal/llm/sample.txt`) are modified.

**Example:** Test 4 "JSON Data Extraction"
```go
{
    Name: "JSON Data Extraction",
    Query: "How many users in internal/llm/test_data.json have the role 'engineer'",
    ShouldContain: []string{"2"}, // Alice and Charlie have role "engineer"
}
```

### Variable Content Tests
Some tests have intentionally empty `ShouldContain` arrays because results vary by environment.

**Example:** Test 13 "Code File Analysis"
```go
{
    Name: "Code File Analysis",
    Query: "In the file agent.go, how many times does the word 'Agent' appear? Just give me the number.",
    ShouldContain: []string{}, // Will vary, but should be a number > 0
}
```

## Update Process

### 1. Identify Outdated Tests
Run the benchmark suite to identify failures:
```bash
make build
./clai benchmark --cli
```

Look for tests failing with "Wrong answer" that should logically pass.

### 2. Analyze Failure Reason
For each failing test:
- Review the query and expected behavior
- Check if the expectation matches current codebase state
- Verify the test logic is still valid

### 3. Update Expectations
Edit `internal/llm/benchmarks.go`:

```go
// Before
ShouldContain: []string{"18"},

// After
ShouldContain: []string{"20"},
```

### 4. Rebuild and Validate
```bash
make build
./clai benchmark --test N  # Test specific benchmark
```

### 5. Commit Changes
```bash
git add internal/llm/benchmarks.go
git commit -m "Update benchmark expectations for current codebase

- Test 2: Updated .go file count from 18 to 20
- Test 13: Added maintenance comment for dynamic content"
```

## Common Maintenance Scenarios

### New Files Added
When adding new `.go` files to tested directories:
1. Run: `find internal/llm -name "*.go" | wc -l`
2. Update Test 2 `ShouldContain` with new count
3. Add comment: `// Updated: YYYY-MM-DD - added X new files`

### Test Data Changes
When modifying `internal/llm/test_data.json` or `internal/llm/sample.txt`:
1. Manually verify expected answers are still correct
2. Update `ShouldContain` arrays accordingly
3. Document the change in commit message

### Algorithm/Logic Updates
When changing how calculations or data processing work:
1. Run affected benchmarks manually
2. Update expectations to match new correct answers
3. Consider if test logic needs revision

## Developing New Benchmark Tests

### Checklist for New Tests

**Planning Phase:**
- [ ] Define clear, unambiguous query
- [ ] Determine expected behavior type: "code", "final", or "multi-step"
- [ ] Identify success criteria (what must be in response)
- [ ] Identify failure indicators (what must NOT be in response)

**Implementation Phase:**
- [ ] Add test case to `ModelBenchmarkSuite` in `internal/llm/benchmarks.go`
- [ ] Set appropriate `MaxIterations` (5 for simple, 10+ for complex)
- [ ] Set reasonable `TimeoutSeconds` (30s for simple, 60-120s for complex)
- [ ] Test with multiple models to ensure expectations are model-agnostic

**Validation Phase:**
- [ ] Run new test individually: `./clai benchmark --test N`
- [ ] Verify passes with current model
- [ ] Run full suite to ensure no regressions
- [ ] Document test purpose and expectations in comments

### Test Categories

**Model Benchmarks (Structured):**
- Focus on specific tool usage and data extraction
- Have precise `ShouldContain`/`ShouldNotContain` expectations
- Test fundamental capabilities (file reading, calculations, JSON parsing)

**Agentic Benchmarks (Natural Language):**
- Test autonomous tool selection and usage
- Use flexible expectations (empty `ShouldNotContain`)
- Focus on end-to-end task completion

**Advanced Benchmarks (Challenging):**
- Multi-step reasoning and complex tool chains
- Safety and error handling scenarios
- Performance and algorithmic challenges

## Troubleshooting Common Issues

### "Wrong answer" Failures
**Symptom:** Test fails with "expected X but got Y"

**Causes:**
- Outdated expectations due to codebase changes
- Test data file modifications
- Logic changes in tool implementations

**Solution:**
1. Review what the test should legitimately return
2. Update expectations to match current correct answers
3. Verify the test logic is still valid

### Synthesis-Related Failures
**Symptom:** Empty responses after tool execution

**Causes:**
- Tool execution tracking issues
- Synthesis logic failures
- Streaming response handling problems

**Solution:**
1. Check synthesis fix implementation in `internal/llm/agent.go`
2. Verify `toolsExecutedInLastIteration` flag is set correctly
3. Test with known working tool chains

### Timeout Failures
**Symptom:** Test exceeds `TimeoutSeconds`

**Causes:**
- Model performance issues
- Overly complex test queries
- Network/API latency problems

**Solution:**
1. Increase `TimeoutSeconds` for genuinely complex tests
2. Simplify test queries if unnecessarily complex
3. Check model server performance

### Validation Logic Issues
**Symptom:** False positives/negatives in pass/fail determination

**Causes:**
- `ShouldContain` requires ANY match (case-insensitive substring)
- `ShouldNotContain` forbids ALL listed strings
- Case sensitivity issues

**Solution:**
1. Review validation logic in `internal/benchmark/runner.go`
2. Test expectations manually with sample responses
3. Adjust string arrays for proper matching

## Build and Validation Commands

### Build Benchmark Binary
```bash
make build  # Updates ./clai with new expectations
```

### Run Specific Test
```bash
./clai benchmark --test 2  # Run Test 2 (0-indexed)
```

### Run Full Suite
```bash
./clai benchmark --cli  # Run all tests with detailed output
```

### Validate Changes
```bash
make build && ./clai benchmark --cli | grep -E "(PASSED|FAILED|TOTAL)"
```

## Best Practices

1. **Always Rebuild:** Run `make build` after changing expectations
2. **Test Incrementally:** Use `--test N` to validate individual changes
3. **Document Updates:** Add comments explaining why expectations changed
4. **Version Control:** Commit benchmark changes separately from functional code
5. **Regression Testing:** Run full suite after any expectation updates

## Related Documentation

- [Tool-Calling Synthesis Fix](./TOOL_CALLING_SYNTHESIS_FIX.md) - Core synthesis implementation
- [Bubble Tea Testing Strategy](./BUBBLETEA_TESTING_STRATEGY.md) - UI testing patterns
- [AGENTS.md](../../AGENTS.md) - Agent development guidelines</content>
<parameter name="filePath">docs/testing/BENCHMARK_MAINTENANCE_GUIDE.md