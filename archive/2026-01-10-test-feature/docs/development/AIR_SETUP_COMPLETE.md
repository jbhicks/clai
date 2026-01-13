# Air + Templ Setup - COMPLETE ✅

## Current Status

The benchmark server is **fully operational** with Air live-reload and Templ template compilation.

**Running:**
- Benchmark server: http://localhost:8080
- Air process: Background with logging to `/tmp/air-output.log`
- Auto-reload: Working (via health check + browser polling)

## What Was Fixed

### 1. **Air `pre_cmd` Bug Workaround** ✅
**Problem:** Air v1.63.0 doesn't properly pass command arguments in `pre_cmd` array.
```toml
# This FAILS - runs "templ" without "generate"
pre_cmd = ["templ", "generate"]
```

**Solution:** Created wrapper script:
```bash
# /tmp/templ-gen.sh
#!/bin/bash
templ generate
```

```toml
# .air.toml
pre_cmd = ["/tmp/templ-gen.sh"]
```

### 2. **Infinite Rebuild Loop** ✅
**Problem:** Air watches `*_templ.go` → `templ generate` updates them → Air rebuilds → loop forever.

**Solution:** Exclude generated files from watching (but still compile them):
```toml
exclude_regex = ["_templ\\.go$"]
```

### 3. **`full_bin` Not Working** ✅
**Problem:** Air's `full_bin = "./tmp/clai benchmark"` wasn't being respected.

**Solution:** Use `args_bin` instead:
```toml
bin = "tmp/clai"
args_bin = ["benchmark"]
```

### 4. **Model Detection Working** ✅
The server correctly detects models from llama.cpp servers:
- http://localhost:8081 → Hermes-3-Llama-3.1-8B.Q4_K_M.gguf
- http://localhost:8082 → nomic-embed-text-v1.5.Q8_0.gguf

Both models show with:
- Full model name
- Server URL
- API type (llamacpp)
- No duplicates

## Current Configuration

**File:** `/home/josh/clai/.air.toml`
```toml
[build]
  pre_cmd = ["/tmp/templ-gen.sh"]
  cmd = "go build -o ./tmp/clai ./cmd/clai"
  bin = "tmp/clai"
  args_bin = ["benchmark"]
  include_ext = ["go", "tpl", "tmpl", "html", "templ"]
  exclude_regex = ["_templ\\.go$"]
```

**Wrapper Script:** `/tmp/templ-gen.sh`
```bash
#!/bin/bash
templ generate
```

## Workflow

1. Edit `.templ` file → Air detects change
2. Air runs `/tmp/templ-gen.sh` → Templates compiled to `*_templ.go`
3. Air runs `go build` → Binary rebuilt with new templates
4. Air runs `./tmp/clai benchmark` → Server starts
5. Browser polls `/health` → Auto-reloads when server restarts

## Testing

**Start Development Server:**
```bash
make dev-benchmark
```

**Check Status:**
```bash
# Server running?
curl http://localhost:8080/api/models/options

# Air logs
tail -f /tmp/air-output.log

# Process status
ps aux | grep "tmp/clai benchmark"
```

**Test Template Changes:**
```bash
# Make a change to any .templ file
echo "<!-- test -->" >> internal/benchmark/templates/new_test.templ

# Air should:
# 1. Run templ generate (in logs)
# 2. Rebuild binary
# 3. Restart server
# 4. Browser auto-reloads
```

## Files Modified

1. `/home/josh/clai/.air.toml`
   - Fixed `pre_cmd` to use wrapper script
   - Changed `full_bin` to `args_bin`
   - Added `exclude_regex` for generated files

2. `/tmp/templ-gen.sh` (created)
   - Wrapper script for `templ generate`

3. `/home/josh/clai/internal/benchmark/server.go`
   - Already has proper model detection logic (no changes needed)

4. `/home/josh/clai/docs/AIR_TEMPL_SETUP.md`
   - Updated with actual working configuration
   - Documented bugs and workarounds

## Known Issues

None! Everything is working as expected.

## Next Steps

The benchmark server is ready for development:
1. Edit templates in `internal/benchmark/templates/*.templ`
2. Edit handlers in `internal/benchmark/server.go`
3. Air will auto-reload on changes
4. Browser will auto-refresh

No manual steps required - full live-reload workflow is operational.
