# Air + Templ Development Setup

## The Problem

When using Air with Templ templates, there's a critical workflow issue:

1. You edit a `.templ` file
2. Air detects the change
3. Air runs `go build`
4. **But the `*_templ.go` files are stale** (never regenerated!)
5. Old code gets compiled
6. You see no changes in the browser

## The Solution

Air needs to run `templ generate` **before** building. This is configured in `.air.toml`:

```toml
[build]
  # Pre-build command (runs before build)
  # IMPORTANT: Must use wrapper script due to Air v1.63.0 bug with command args
  pre_cmd = ["/tmp/templ-gen.sh"]
  
  # The command to build your application
  cmd = "go build -o ./tmp/clai ./cmd/clai"
  
  # The path to the binary to run
  bin = "tmp/clai"
  
  # Arguments to pass to the binary (use args_bin, not full_bin)
  args_bin = ["benchmark"]
  
  # Watch .templ files
  include_ext = ["go", "tpl", "tmpl", "html", "templ"]
  
  # CRITICAL: Exclude generated files to prevent rebuild loops
  # The files are still compiled, just not watched for changes
  exclude_regex = ["_templ\\.go$"]
```

**Create the wrapper script:**
```bash
cat > /tmp/templ-gen.sh << 'EOF'
#!/bin/bash
templ generate
EOF
chmod +x /tmp/templ-gen.sh
```

## How It Works

1. You edit `new_test.templ`
2. Air detects the change (`.templ` is in `include_ext`)
3. Air runs `pre_cmd`: **`/tmp/templ-gen.sh`** (which runs `templ generate`)
4. Templ compiles `.templ` → `*_templ.go`
5. **Generated files are ignored** (`exclude_regex` prevents rebuild loop)
6. Air runs `cmd`: `go build`
7. Binary is rebuilt with fresh templates
8. Air runs binary with `args_bin`: `tmp/clai benchmark`
9. Browser auto-reloads (via our health check script)

## Testing the Setup

1. **Start Air:**
   ```bash
   make dev-benchmark
   ```

2. **Edit a template:**
   ```bash
   # Make a visible change to test
   echo "<!-- test change -->" >> internal/benchmark/templates/new_test.templ
   ```

3. **Watch Air logs:**
   - Should see: `Running pre_cmd: templ generate`
   - Then: `Building...`
   - Then: `running...`

4. **Verify in browser:**
   - Page should auto-reload
   - Changes should be visible

## Alternative: Manual Workflow

If Air is causing issues, you can use manual workflow:

```bash
# Terminal 1: Watch and compile templates
while true; do
  inotifywait -e modify -r internal/benchmark/templates/
  templ generate
done

# Terminal 2: Run Air (will detect *_templ.go changes)
make dev-benchmark
```

## Comparison: Air vs Manual

| Feature | Air + pre_cmd | Manual |
|---------|---------------|--------|
| Auto template compile | ✅ | ✅ |
| Auto Go rebuild | ✅ | ✅ |
| Auto server restart | ✅ | ✅ |
| Browser auto-reload | ✅ | ✅ |
| Setup complexity | Simple | Complex (2 terminals) |
| **Recommended** | **YES** | Only if Air fails |

## Common Issues

### Issue: Changes not appearing

**Symptom:** You edit `.templ` file but see no changes in browser.

**Debug:**
```bash
# Check if templ is running
grep "templ generate" tmp/air.log

# Check generated file timestamp
stat internal/benchmark/templates/new_test_templ.go

# Check binary timestamp
stat tmp/clai

# Generated file should be NEWER than your edit
# Binary should be NEWER than generated file
```

**Fix:** Ensure `pre_cmd = ["templ generate"]` is in `.air.toml`

### Issue: Air not detecting `.templ` files

**Symptom:** Edit `.templ` file, Air doesn't rebuild.

**Fix:** Check `include_ext` in `.air.toml`:
```toml
include_ext = ["go", "tpl", "tmpl", "html", "templ"]
```

### Issue: Infinite rebuild loop

**Symptom:** Air keeps rebuilding over and over, server never stays running.

**Cause:** Air watches `*_templ.go` files. When `templ generate` updates them, Air detects a change and rebuilds. This triggers `pre_cmd` again, which regenerates templates, causing another rebuild... infinite loop.

**Fix:** Exclude generated files from watching:
```toml
exclude_regex = ["_templ\\.go$"]
```

This prevents the loop while still compiling the generated files into the binary.

### Issue: Air doesn't pass command arguments properly

**Symptom:** Air runs `templ` instead of `templ generate`, or uses `full_bin` incorrectly.

**Cause:** Air v1.63.0 has bugs with:
- Array command arguments in `pre_cmd`
- The `full_bin` directive not being respected

**Fix:** 
1. Use wrapper script for `pre_cmd`: `pre_cmd = ["/tmp/templ-gen.sh"]`
2. Use `args_bin` instead of `full_bin`: `args_bin = ["benchmark"]`

## Recommendation

**Keep using Air with `pre_cmd`** - it's the cleanest solution. The configuration is now correct in `.air.toml`.

## Testing After This Change

After updating `.air.toml`, restart Air:

```bash
# Kill existing Air
pkill air

# Start fresh
make dev-benchmark
```

Edit any `.templ` file and verify changes appear in browser.
