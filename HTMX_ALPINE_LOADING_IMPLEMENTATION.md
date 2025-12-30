# HTMX + Alpine.js Loading Indicators Implementation

## Summary

Successfully added loading spinners to Start/Stop buttons using Alpine.js alongside HTMX. This demonstrates how HTMX and Alpine.js work together seamlessly for enhanced UI/UX.

## Critical Bug Fixed: Deadlock in StopServer/StartServer

### Problem
The `StopServer` and `StartServer` methods in `model_manager.go` were holding mutex locks while performing I/O operations (file reads, external commands, HTTP requests). This caused a **deadlock** when `RefreshServerStatus()` was called after stopping/starting, as it tried to acquire the same lock.

### Root Cause (lines 478-530)
```go
func (mm *ModelManager) StopServer(modelPath string) error {
    mm.mu.Lock()           // ❌ Lock acquired
    defer mm.mu.Unlock()
    
    // ... validation ...
    
    mm.isSystemdManaged(server.PID)        // ❌ File I/O while holding lock
    mm.getSystemdServiceName(server.PID)   // ❌ File I/O while holding lock
    exec.Command("systemctl", ...).Run()   // ❌ External command while holding lock
    process.Kill()                         // ❌ Syscall while holding lock
    
    // Update state
    server.Status = "stopped"
}
```

When `HandleStopServer` called `RefreshServerStatus()` on line 825, it tried to acquire the same lock → **deadlock** → server crash.

### Solution: I/O First, Then Lock
Following the pattern from AGENTS.md ("Never Hold Locks During I/O Operations"):

```go
func (mm *ModelManager) StopServer(modelPath string) error {
    // Step 1: Brief lock to get data needed for I/O
    mm.mu.Lock()
    server, exists := mm.servers[modelPath]
    // ... validation ...
    pid := server.PID
    modelName := server.ModelName
    mm.mu.Unlock()  // ✅ Release lock BEFORE I/O
    
    // Step 2: Do ALL I/O operations WITHOUT holding lock
    if mm.isSystemdManaged(pid) {
        serviceName := mm.getSystemdServiceName(pid)
        exec.Command("systemctl", ...).Run()
    } else {
        process.Kill()
    }
    
    // Step 3: Brief lock to update state
    mm.mu.Lock()
    server.Status = "stopped"
    mm.mu.Unlock()
}
```

**Same fix applied to `StartServer()`** (lines 379-447) which had the same issue with `os.Create()` and `cmd.Start()`.

### Benefits
- Lock held for microseconds (memory operations) instead of seconds (I/O)
- No deadlock risk from nested lock attempts
- Other goroutines can read data while I/O is happening
- Much better application responsiveness

## Alpine.js + HTMX Integration

### Files Modified

1. **`/home/josh/clai/internal/benchmark/templates/layout.templ`**
   - Added Alpine.js CDN (line 13): `<script defer src="https://unpkg.com/alpinejs@3.x.x/dist/cdn.min.js"></script>`
   - Added spinner CSS (lines 330-356)
   - Added loading state styles

2. **`/home/josh/clai/internal/benchmark/model_manager.go`**
   - Updated Stop button HTML (lines 751-763)
   - Updated Start button HTML (lines 771-783)

### How It Works

**Alpine.js manages loading state**, while **HTMX handles the HTTP request**:

```html
<form 
    hx-post="/api/servers/start" 
    hx-target="#servers_list" 
    hx-swap="innerHTML" 
    x-data="{ loading: false }"
>
    <input type="hidden" name="model_path" value="..." />
    <button 
        type="submit"
        @click="loading = true"
        :disabled="loading"
        :class="{ 'loading': loading }"
    >
        <span class="spinner"></span>
        <span class="btn-text">Start</span>
    </button>
</form>
```

**Flow:**
1. User clicks button
2. `@click="loading = true"` → Alpine.js sets `loading` state
3. `:class="{ 'loading': loading }"` → Adds `.loading` class to button
4. CSS shows spinner, hides text:
   ```css
   .loading .btn-text { display: none; }
   .loading .spinner { display: inline-block; }
   ```
5. `:disabled="loading"` → Button becomes disabled (prevents double-clicks)
6. HTMX sends POST request
7. Server responds with updated HTML
8. HTMX swaps content, which resets Alpine.js state (new elements rendered)

### Spinner CSS

```css
.spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    border-top-color: white;
    animation: spin 0.6s linear infinite;
    margin-right: 6px;
    vertical-align: middle;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}
```

### Why Alpine.js + HTMX?

**Alpine.js is perfect for local UI state:**
- Loading indicators
- Show/hide toggles
- Form validation
- Dropdowns, tabs, modals

**HTMX is perfect for server communication:**
- Fetching HTML from server
- Form submissions
- Polling/SSE for real-time updates
- Swapping DOM content

**Together, they complement each other:**
- Alpine.js handles ephemeral UI state (loading, open/closed, selected)
- HTMX handles server state (data fetching, mutations)
- No need for a heavy framework like React/Vue for simple interactions

## Testing

### Verify Alpine.js is Loaded
```bash
curl -s http://localhost:8080/api/servers/list | grep 'x-data="{ loading: false }"'
```

Expected output:
```
x-data="{ loading: false }"
x-data="{ loading: false }"
```

### Verify Spinner Elements
```bash
curl -s http://localhost:8080/api/servers/list | grep 'class="spinner"'
```

Expected output:
```html
<span class="spinner"></span>
<span class="btn-text">Start</span>
```

### Manual Testing
1. Navigate to `http://localhost:8080/`
2. Click "Start" on a stopped model
3. **Expected behavior:**
   - Button shows spinner immediately
   - Button is disabled (prevents double-clicks)
   - Text "Start" disappears
   - After server responds (~500ms-2s), button disappears and "Stop" button appears
4. Click "Stop" on running model
5. **Expected behavior:**
   - Same loading indicator behavior
   - After response, status changes to "Stopped"

## Best Practices

### When to Use Alpine.js with HTMX

**✅ DO use Alpine.js for:**
- Loading spinners on async requests
- Client-side form validation before submission
- Show/hide UI elements (dropdowns, modals, collapsible sections)
- Local search/filtering (before sending to server)
- Tab/accordion UI state

**❌ DON'T use Alpine.js for:**
- Making HTTP requests (use HTMX)
- Replacing HTMX functionality
- Complex state management (use a framework like React/Vue instead)

### Alpine.js + HTMX Event Integration

HTMX emits events that Alpine.js can listen to:

```html
<div 
    x-data="{ loading: false }"
    @htmx:before-request="loading = true"
    @htmx:after-swap="loading = false"
>
    <button hx-get="/data">Fetch</button>
    <span x-show="loading">Loading...</span>
</div>
```

**Available HTMX events:**
- `htmx:before-request` - Before request is sent
- `htmx:after-request` - After request completes
- `htmx:before-swap` - Before content is swapped
- `htmx:after-swap` - After content is swapped
- `htmx:response-error` - On HTTP error

### Loading State Alternatives

**Option 1: Button-level state (current implementation)**
```html
<form x-data="{ loading: false }">
    <button @click="loading = true" :disabled="loading">
        <span class="spinner"></span>
        <span class="btn-text">Start</span>
    </button>
</form>
```

**Option 2: Global loading overlay**
```html
<div 
    x-data="{ loading: false }"
    @htmx:before-request.window="loading = true"
    @htmx:after-swap.window="loading = false"
>
    <div x-show="loading" class="loading-overlay">
        <div class="spinner-large"></div>
    </div>
</div>
```

**Option 3: HTMX indicator attribute**
```html
<button hx-indicator="#spinner">Start</button>
<span id="spinner" class="htmx-indicator spinner"></span>
```

We chose **Option 1** because it:
- Provides clear visual feedback per button
- Prevents double-clicks automatically
- Resets automatically when content swaps
- Requires minimal JavaScript

## Current State

### What's Working ✅
- Alpine.js loaded and functional
- Loading spinners on Start/Stop buttons
- Buttons disable during loading
- Auto-reset when HTMX swaps new content
- Deadlock bug fixed in `StopServer` and `StartServer`
- Server auto-reload via `make dev`

### Pending Tasks ⏳
1. **Test Delete button** - Has `hx-confirm` dialog, should work similarly
2. **Verify SSE events** - Ensure real-time updates still work after changes
3. **Consider adding loading to Delete button** - Same pattern as Start/Stop

## Resources

- [Alpine.js Documentation](https://alpinejs.dev)
- [HTMX Documentation](https://htmx.org)
- [HTMX + Alpine.js Guide](https://htmx.org/examples/alpine-js/)
- [AGENTS.md Concurrency Guidelines](/home/josh/clai/AGENTS.md#concurrency-and-mutex-guidelines)
