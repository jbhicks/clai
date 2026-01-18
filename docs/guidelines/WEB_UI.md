# Web UI (HTMX and Alpine.js) Guidelines

This document covers best practices for CLAI's web-based dashboard using HTMX and Alpine.js.

## HTMX and Idiomorph

### Prevent Flickering with morph:outerHTML
Always prefer `hx-swap="morph:outerHTML"` over innerHTML swaps to preserve DOM structure and prevent flickering.
- The server handler MUST return the complete replacement element with the same ID as the target.

### Listening for SSE Events
```html
<div hx-ext="sse" sse-connect="/api/updates" sse-swap="update">
  <span class="counter">Connecting...</span>
</div>
```
- Content added via morphing is NOT automatically processed by HTMX. Use a global `htmx:afterSwap` listener to call `htmx.process(target)` if needed.

## Alpine.js + HTMX Integration
Use Alpine.js for local UI state and HTMX for server communication.

### Loading Indicators Pattern
Use Alpine.js to manage button loading states while HTMX handles the request.
```html
<button 
    @click="loading = true"
    :disabled="loading"
    :class="{ 'loading': loading }"
>
    <span class="spinner"></span>
    <span class="btn-text">Start</span>
</button>
```

### Library Versions
- **HTMX**: 2.0.8 (required for SSE extension stability).
- **HTMX SSE Extension**: 2.2.4.
- **Idiomorph**: 0.3.0.
