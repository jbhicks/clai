---
name: htmx-best-practices
description: Enforced HTMX guidelines for SSE, morph swaps, and anti-patterns
---

# HTMX Best Practices

Use this skill when reviewing or implementing HTMX features. It codifies rules
that prevent flicker, double polling, and broken swaps.

## Critical Rules

### 1. Single SSE connection per page
Do NOT open multiple SSE connections.

Bad:
```html
<div hx-ext="sse" sse-connect="/api/updates1"></div>
<div hx-ext="sse" sse-connect="/api/updates2"></div>
```

Good:
```html
<div hx-ext="sse" sse-connect="/api/events">
  <div hx-trigger="sse:updates1" hx-get="/api/data1" hx-swap="morph:innerHTML"></div>
  <div hx-trigger="sse:updates2" hx-get="/api/data2" hx-swap="morph:innerHTML"></div>
</div>
```

### 2. Use morph:innerHTML for pure content
Avoid morphing containers that include HTMX controls or stateful elements.

Bad:
```html
<div hx-get="/api/content" hx-swap="morph:innerHTML">
  <button hx-post="/api/action">Click</button>
</div>
```

Good:
```html
<div hx-get="/api/content" hx-swap="morph:innerHTML"><!-- pure content only --></div>
```

### 3. No manual polling
Do not use `setInterval` for periodic refreshes.

Bad:
```javascript
setInterval(() => htmx.ajax('GET', '/api/status'), 1000);
```

Good:
```html
<div hx-get="/api/status" hx-trigger="load, sse:status_update"></div>
```

### 4. No manual htmx.process()
Do not call `htmx.process()` directly.

Bad:
```javascript
htmx.process(form);
```

Good:
```html
<form hx-post="/api/submit"><!-- HTMX processes automatically --></form>
```

## Review Checklist

- Single SSE connection per page
- No setInterval polling
- No manual htmx.process()
- Proper morph swap usage
- Event-driven updates only

## References

- `internal/benchmark/templates/` for correct HTMX usage patterns
