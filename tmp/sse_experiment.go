package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

var pattern2Counter int64
var pattern3Counter int64

func main() {
	http.HandleFunc("/sse/pattern1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher := w.(http.Flusher)
		counter := 0

		for {
			counter++
			fmt.Fprintf(w, "event: update\n")
			fmt.Fprintf(w, "data: %d\n\n", counter)
			flusher.Flush()
			time.Sleep(2 * time.Second)

			if r.Context().Err() != nil {
				break
			}
		}
	})

	http.HandleFunc("/sse/pattern2-events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher := w.(http.Flusher)
		counter := 0

		for {
			counter++
			fmt.Fprintf(w, "event: refresh\n")
			fmt.Fprintf(w, "data:\n\n")
			flusher.Flush()
			time.Sleep(2 * time.Second)

			if r.Context().Err() != nil {
				break
			}
		}
	})

	http.HandleFunc("/sse/pattern2", func(w http.ResponseWriter, r *http.Request) {
		newCounter := atomic.AddInt64(&pattern2Counter, 1)
		fmt.Printf("Pattern 2 hx-get: counter=%d\n", newCounter)
		fmt.Fprintf(w, `<span class="counter">%d</span>`, newCounter)
	})

	http.HandleFunc("/sse/pattern3", func(w http.ResponseWriter, r *http.Request) {
		newCounter := atomic.AddInt64(&pattern3Counter, 1)
		fmt.Printf("Pattern 3 poll: counter=%d\n", newCounter)
		fmt.Fprintf(w, "<span class=\"counter\" id=\"p3-counter\">%d</span>", newCounter)
	})

	http.HandleFunc("/sse/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
    <title>SSE Patterns Test - HTMX 2.0.8 + SSE 2.2.4</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.8/dist/htmx.min.js" integrity="sha384-/TgkGk7p307TH7EXJDuUlgG3Ce1UVolAOFopFekQkkXihi5u/6OCvVKyz1W+idaz" crossorigin="anonymous"></script>
    <script>
        const registeredExtensions = {};
        const originalDefineExtension = htmx.defineExtension.bind(htmx);
        htmx.defineExtension = function(name, extension) {
            const result = originalDefineExtension(name, extension);
            registeredExtensions[name] = true;
            console.log('[EXT] Registered:', name);
            return result;
        };
    </script>
    <script src="https://cdn.jsdelivr.net/npm/htmx-ext-sse@2.2.4/sse.js" integrity="sha384-QA9wXqexhwzXTuTvuF5QP82pddm3R2hy81UzXi7ioNTqNF2b75hlkkSGjafohhL3" crossorigin="anonymous"></script>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            padding: 20px; 
            max-width: 900px; 
            margin: 0 auto; 
            background: #1a1a2e; 
            color: #eee; 
        }
        h1 { color: #00d9ff; margin-bottom: 10px; }
        h2 { color: #ff6b6b; margin-top: 30px; margin-bottom: 15px; }
        .subtitle { color: #888; margin-bottom: 30px; }
        .pattern { 
            border: 2px solid #444; 
            padding: 20px; 
            margin: 20px 0;
            border-radius: 8px;
            background: #16213e;
        }
        .status { padding: 15px; margin: 20px 0; border-radius: 8px; font-family: monospace; font-size: 14px; }
        .loaded { background: #1e3a2f; color: #4ade80; border: 1px solid #4ade80; }
        .not-loaded { background: #3a1e1e; color: #f87171; border: 1px solid #f87171; }
        .info { background: #1a2a3a; color: #60a5fa; border: 1px solid #60a5fa; }
        .counter { font-size: 32px; font-weight: bold; color: #00d9ff; min-width: 100px; display: inline-block; text-align: center; }
        .label { color: #888; font-size: 14px; margin-right: 10px; }
        code { background: #0f0f23; padding: 3px 8px; border-radius: 4px; font-size: 13px; color: #a78bfa; }
        .event-log { background: #0d1117; border: 1px solid #30363d; border-radius: 6px; padding: 12px; font-family: monospace; font-size: 12px; max-height: 150px; overflow-y: auto; white-space: pre-wrap; color: #4ade80; }
        .instructions { background: #1a2a3a; border: 1px solid #60a5fa; border-radius: 6px; padding: 15px; margin: 20px 0; }
        .instructions h3 { margin-top: 0; color: #60a5fa; }
        .instructions ul { margin: 10px 0; padding-left: 20px; }
        .instructions li { margin: 5px 0; color: #ccc; }
    </style>
</head>
<body hx-ext="sse">
    <h1>SSE + HTMX Integration Test</h1>
    <p class="subtitle">Using <code>htmx.org@2.0.8</code> + <code>htmx-ext-sse@2.2.4</code></p>
    
    <div id="status" class="status info">Checking extension status...</div>

    <div class="instructions">
        <h3>Debug Steps</h3>
        <ul>
            <li>Open browser Console (F12) to see SSE events</li>
            <li>Look for: <code>[EXT] Registered: sse</code></li>
            <li>Look for: <code>[SSE] Message received</code></li>
            <li>Check Network tab for pending SSE connections</li>
        </ul>
    </div>

    <div id="event-log" class="event-log">SSE Event Log:</div>

    <h2>Pattern 1: Native sse-swap</h2>
    <p>Server sends: <code>event: update\ndata: &lt;counter value&gt;</code> - Direct content swap</p>
    <div class="pattern" hx-ext="sse" sse-connect="/sse/pattern1" sse-swap="update">
        <span class="label">Counter:</span>
        <span class="counter">Connecting...</span>
    </div>

    <h2>Pattern 2: SSE with hx-trigger</h2>
    <p>Server sends: <code>event: refresh\ndata: (empty)</code> - Triggers hx-get to fetch content</p>
    <div class="pattern" hx-ext="sse" sse-connect="/sse/pattern2-events">
        <span class="label">Counter:</span>
        <span hx-get="/sse/pattern2" hx-trigger="sse:refresh" hx-swap="innerHTML" class="counter">Connecting...</span>
    </div>

    <h2>Pattern 3: Regular Polling (Works)</h2>
    <div class="pattern" id="pattern3">
        <span class="label">Counter:</span>
        <div hx-get="/sse/pattern3" hx-trigger="load, every 3s" hx-swap="innerHTML">
            <span class="counter">Loading...</span>
        </div>
    </div>
    <p><small>Regular HTMX polling - this works!</small></p>

    <script>
        function logEvent(msg) {
            const log = document.getElementById('event-log');
            const time = new Date().toLocaleTimeString();
            log.textContent += '\n[' + time + '] ' + msg;
            log.scrollTop = log.scrollHeight;
        }
        
        document.body.addEventListener('htmx:sseOpen', function(e) {
            logEvent('SSE Connection OPENED');
        });
        document.body.addEventListener('htmx:sseError', function(e) {
            logEvent('SSE Error: ' + (e.detail.error ? e.detail.error.message : 'unknown'));
        });
        document.body.addEventListener('htmx:sseClose', function(e) {
            logEvent('SSE Connection CLOSED: ' + e.detail.type);
        });
        document.body.addEventListener('htmx:sseMessage', function(e) {
            logEvent('SSE Message received');
        });
        
        window.addEventListener('load', function() {
            setTimeout(function() {
                const statusEl = document.getElementById('status');
                console.log('registeredExtensions:', registeredExtensions);
                console.log('htmx.version:', htmx.version);
                
                if (registeredExtensions.sse) {
                    statusEl.innerHTML = '✓ SSE extension loaded<br>Extensions: ' + Object.keys(registeredExtensions).join(', ');
                    statusEl.className = 'status loaded';
                    logEvent('Extensions registered: ' + Object.keys(registeredExtensions).join(', '));
                } else {
                    statusEl.innerHTML = '✗ SSE NOT loaded<br>Registered: ' + Object.keys(registeredExtensions).join(', ');
                    statusEl.className = 'status not-loaded';
                    logEvent('SSE NOT registered - Extensions: ' + Object.keys(registeredExtensions).join(', '));
                }
            }, 1000);
        });
    </script>
</body>
</html>`)
	})

	fmt.Println("SSE test server running on http://localhost:9999")
	fmt.Println("Test page: http://localhost:9999/sse/test")
	http.ListenAndServe(":9999", nil)
}
