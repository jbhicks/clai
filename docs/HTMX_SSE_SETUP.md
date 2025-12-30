# HTMX SSE Extension Setup

## Version Requirements

**CRITICAL**: The HTMX SSE extension has strict version requirements:

| Package | Version | Notes |
|---------|---------|-------|
| htmx.org | **2.0.2** | MUST match - 2.0.4 breaks SSE extension |
| htmx-ext-sse | **2.2.3** | Depends on htmx.org ^2.0.2 |

## Why These Versions?

From `htmx-ext-sse@2.2.3/package.json`:
```json
"dependencies": {
  "htmx.org": "^2.0.2"
}
```

HTMX 2.0.4 introduced internal API changes that break the SSE extension (see [GitHub #3337](https://github.com/bigskysoftware/htmx/issues/3337)).

## Correct Script Loading Order

```html
<!-- 1. Load HTMX first - MUST use full path -->
<script src="https://unpkg.com/htmx.org@2.0.2/dist/htmx.min.js"></script>

<!-- 2. Load SSE extension AFTER HTMX is available globally -->
<script src="https://cdn.jsdelivr.net/npm/htmx-ext-sse@2.2.3/sse.js"></script>

<!-- 3. Optional: Load other extensions (Idiomorph, etc.) -->
<script src="https://unpkg.com/idiomorph@0.3.0/dist/idiomorph-ext.min.js"></script>
```

**Important**: Use the full path `dist/htmx.min.js` - the bare `@2.0.2` URL redirects and can cause issues.

## Checking if SSE Extension is Loaded

**DO NOT** check `htmx.ext.sse` - `htmx.ext` doesn't exist in HTMX 2.0.x.

Instead, monkey-patch `defineExtension` to track registered extensions:

```javascript
const registeredExtensions = {};
const originalDefineExtension = htmx.defineExtension.bind(htmx);
htmx.defineExtension = function(name, extension) {
    const result = originalDefineExtension(name, extension);
    registeredExtensions[name] = true;
    console.log('Extension "' + name + '" registered');
    return result;
};

// Later check:
if (registeredExtensions.sse) {
    console.log('✓ SSE extension loaded');
}
```

## SSE Attribute Usage

### Method 1: Native sse-swap (Recommended)
```html
<div sse-connect="/api/updates" sse-swap="eventName">
  <!-- Content will be replaced by SSE data -->
</div>
```

Server sends:
```
event: eventName
data: <div>New content</div>
```

### Method 2: SSE with hx-trigger
```html
<div sse-connect="/api/updates" sse-swap="refresh">
  <div hx-trigger="load, sse:refresh" hx-swap="outerHTML">
    Initial content
  </div>
</div>
```

Server sends:
```
event: refresh
data: (any content - triggers hx-trigger)
```

### Method 3: OOB Swap with hx-swap-oob
```html
<div id="item_123">Original</div>
```

Server sends:
```
event: update
data: <div id="item_123" hx-swap-oob="true">Updated</div>
```

## SSE Event Format

Server must send properly formatted SSE events:

```go
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
w.Header().Set("Access-Control-Allow-Origin", "*")

// Named event
fmt.Fprintf(w, "event: download_update\n")
fmt.Fprintf(w, "data: <div id=\"item_123\">Content</div>\n\n")

flusher.Flush()
```

## SSE Event Names

| Event | Purpose |
|-------|---------|
| `sse:eventName` | Triggers hx-trigger when eventName is received |
| `htmx:sseOpen` | Connection established |
| `htmx:sseClose` | Connection closed |
| `htmx:sseError` | Error occurred |
| `htmx:sseMessage` | Message received |

## Testing SSE

Run the test server:
```bash
cd /home/josh/clai/tmp
go run sse_experiment.go
```

Then open: http://localhost:9999/sse/test

The test page shows:
1. Pattern 1: Native sse-swap (direct DOM swap)
2. Pattern 2: SSE with hx-trigger (event-based)
3. Pattern 3: Regular polling (fallback)

**Expected console output:**
```
defineExtension monkey-patched
=== defineExtension called: sse ===
Extension "sse" registered successfully
=== defineExtension called: morph ===
Extension "morph" registered successfully
✓ SSE extension loaded
```

## CLAI Implementation

In CLAI, SSE is used for real-time download progress:

```html
<div 
  id="downloads_list"
  hx-get="/api/models/downloads"
  hx-trigger="load"
  hx-swap="morph:outerHTML"
  hx-ext="morph"
  sse-connect="/api/models/downloads/stream"
  sse-swap="download_update"
>
  Loading downloads...
</div>
```

The server sends OOB swaps for individual download items to update progress bars without refreshing the entire list.

## Troubleshooting

### SSE extension not registering
- Ensure HTMX 2.0.2 is loaded BEFORE the SSE extension
- Check browser console for JavaScript errors
- Verify no network errors loading the scripts

### SSE connected but no updates
- Check server is sending events with correct format
- Ensure `event:` and `data:` lines are properly formatted
- Each event must end with double newline `\n\n`

### Connections being closed
- Server must keep connection open (don't return after sending)
- Use `http.Flusher` to flush after each event
- Check for timeouts on either client or server
