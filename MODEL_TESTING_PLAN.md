# Model Testing and Comparison Plan

## Objective
Thoroughly test different LLM models for tool calling capabilities to replace Qwen3-Coder-30B with a model better suited for autonomous agent behavior.

## Problem Statement
Qwen3-Coder-30B is optimized for code generation, not tool calling. It doesn't reliably generate `<code>` blocks for executing actions, leading to infinite loops and poor agent performance.

## Testing Infrastructure

### 1. Automated Benchmark Suite (`model_benchmark_test.go`)
Comprehensive test suite with 10 test cases covering:
- **Simple file operations** (cat, ls, grep)
- **Multi-step reasoning** (count files, analyze results)
- **Code execution** (Python, JavaScript, Bash)
- **Error handling** (nonexistent files)
- **General knowledge** (no code needed)
- **JSON parsing**
- **Pattern matching**

**Metrics tracked:**
- Pass/fail rate
- Number of iterations per task
- Execution time
- Response quality

### 2. Model Testing Script (`test_models.sh`)
Automates the process of:
1. Stopping current llama-server
2. Starting server with new model
3. Running benchmark suite
4. Collecting results
5. Generating comparison reports

**Usage:**
```bash
./test_models.sh --all                          # Test all models
./test_models.sh <model.gguf>                   # Test specific model
./test_models.sh                                # Interactive mode
```

### 3. Model Download Script (`download_models.sh`)
Downloads recommended models for tool calling:

| Model | Size | Description |
|-------|------|-------------|
| Llama 3.1 8B | 4.9GB | Meta's official tool calling support |
| Hermes 3 8B | 4.9GB | Specialized for tool calling (Nous Research) |
| Mistral Nemo 12B | 7.1GB | 128k context, designed for tools |
| Qwen 2.5 14B | 8.5GB | General purpose with tool support |
| **GPT-OSS-120B** | **68GB** | **OpenAI reasoning + tools (proven fast on Strix Halo)** |
| Llama 3.1 70B | 40GB | Most capable general model |

**Usage:**
```bash
./download_models.sh
```

## Testing Plan

### Phase 1: Quick Assessment (Today)
1. **Test current Qwen3-Coder-30B** to establish baseline
   ```bash
   ./test_models.sh Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf
   ```
   Expected result: Low success rate, many iterations

2. **Download Hermes 3 8B** (best for tool calling, small size)
   ```bash
   ./download_models.sh  # Select option 2, then 1
   ```

3. **Test Hermes 3 8B**
   ```bash
   ./test_models.sh Hermes-3-Llama-3.1-8B.Q4_K_M.gguf
   ```

### Phase 2: Comprehensive Comparison (After initial results)
1. Download all recommended models
2. Run full benchmark suite on each
3. Compare results side-by-side

### Phase 3: Integration (Best model identified)
1. Update default model in clai
2. Test with real user queries
3. Monitor for loop detection issues

## Expected Results

### Current Model (Qwen3-Coder-30B):
- **Predicted success rate**: 30-50%
- **Common failures**: 
  - Doesn't generate `<code>` blocks autonomously
  - Requires "smart fallback" hacks
  - Gets stuck in observation loops

### Target Models (Llama 3.1 / Hermes 3):
- **Expected success rate**: 70-90%
- **Benefits**:
  - Native tool calling support
  - Proper `<code>` block generation
  - Better multi-step reasoning
  - Lower iteration counts

## Success Criteria

A model is considered successful if it achieves:
- ✓ **>70% test pass rate**
- ✓ **<5 avg iterations per task**
- ✓ **<30s avg response time**
- ✓ **No infinite loops** (max 3 repeated actions)

## Results Storage

All test results saved in `./model_test_results/`:
- `<model_name>_results_<timestamp>.txt` - Full test output
- `<model_name>_server.log` - Server logs
- Comparison summaries

## Hardware Context

**AMD Strix Halo with ROCm**:
- Specialized llama.cpp build at `/home/josh/llama.cpp-rocm-wmma/`
- Full GPU offload (`-ngl 999`)
- Flash attention enabled (`-fa on`)
- 128k+ context window support
- Models stored in `/home/josh/models/`

## Next Actions

1. **Run baseline test**:
   ```bash
   cd /home/josh/clai
   ./test_models.sh Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf
   ```

2. **Download and test Hermes 3**:
   ```bash
   ./download_models.sh
   ./test_models.sh Hermes-3-Llama-3.1-8B.Q4_K_M.gguf
   ```

3. **Review results and decide**:
   - If Hermes 3 achieves >70% success rate, switch to it
   - If not, test Llama 3.1 8B
   - If neither works, investigate prompt engineering or structured output

## Additional Notes

- All tests run via `make dev` auto-reload environment
- Results can be compared using `diff` or custom analysis
- Server must be restarted between model tests
- Keep current Qwen3-Coder as fallback for code generation tasks
