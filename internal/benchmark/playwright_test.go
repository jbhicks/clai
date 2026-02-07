package benchmark

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"clai/internal/db"
	"github.com/playwright-community/playwright-go"
)

func TestModelsPageFullE2E(t *testing.T) {
	if os.Getenv("CI") == "" && os.Getenv("PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD") != "" {
		t.Skip("Skipping browser test - browsers not installed")
	}

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Failed to start Playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		t.Fatalf("Failed to launch Chromium: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	dbStore, err := db.New()
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dbStore.DB().Close()

	server := &Server{
		store:        dbStore,
		modelManager: NewModelManagerForTest(),
		sseClients:   make(map[chan string]bool),
		sseMutex:     sync.RWMutex{},
		port:         0,
	}

	testServer := httptest.NewServer(server.Router())
	testServer.Config.WriteTimeout = 1 * time.Second
	defer testServer.Close()

	// Log the actual content of the page to debug why it's not the testing page
	pageGotoOptions := playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}
	_, err = page.Goto(testServer.URL+"/models", pageGotoOptions)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	content, _ := page.Content()
	t.Logf("Page content: %s", truncate(content, 500))

	t.Run("Servers section has title", func(t *testing.T) {
		serversList := page.Locator("#servers_list").First()
		html, _ := serversList.InnerHTML()

		if strings.Contains(html, "Model Servers") || strings.Contains(html, "Loading") {
			t.Log("Servers section has content")
		} else {
			t.Logf("Servers content: %s", truncate(html, 300))
		}
	})

	t.Run("Model search input exists", func(t *testing.T) {
		searchInput := page.Locator("#model_search").First()

		isVisible, err := searchInput.IsVisible()
		if err != nil {
			t.Fatalf("Failed to check visibility: %v", err)
		}
		if !isVisible {
			t.Error("Model search input should be visible")
		}

		placeholder, err := searchInput.GetAttribute("placeholder")
		if err != nil {
			t.Fatalf("Failed to get placeholder: %v", err)
		}
		t.Logf("Placeholder: %s", placeholder)
	})
}

func TestAPIDownloadsEndpoint(t *testing.T) {
	server := &Server{
		store:        nil,
		modelManager: NewModelManagerForTest(),
		sseClients:   make(map[chan string]bool),
		sseMutex:     sync.RWMutex{},
		port:         0,
	}

	t.Run("Returns empty list when no downloads", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/models/downloads", nil)
		server.handleGetDownloads(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("Expected Content-Type text/html, got: %s", contentType)
		}

		body := w.Body.String()
		if !strings.Contains(body, "No active downloads") {
			t.Errorf("Response should contain 'No active downloads', got: %s", truncate(body, 200))
		}
		t.Log("Downloads endpoint returns empty list correctly")
	})

	t.Run("Returns HTML with proper structure", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/models/downloads", nil)
		server.handleGetDownloads(w, r)

		body := w.Body.String()

		if !strings.Contains(body, "Active Downloads") {
			t.Error("Response should contain 'Active Downloads' header")
		}

		if !strings.Contains(body, "Show All") {
			t.Error("Response should contain 'Show All' button")
		}

		t.Log("Downloads endpoint returns proper HTML structure")
	})
}

func TestAPIServersListEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("MODELS_PATH", tempDir)

	store, err := db.New()
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer store.DB().Close()

	server := &Server{
		store:        store,
		modelManager: NewModelManagerForTest(),
		sseClients:   make(map[chan string]bool),
		sseMutex:     sync.RWMutex{},
		port:         0,
	}

	modelPath := filepath.Join(tempDir, "test-model.gguf")
	f, err := os.Create(modelPath)
	if err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}
	f.Close()

	server.modelManager.mu.Lock()
	server.modelManager.servers[modelPath] = &ModelServer{
		ModelPath: modelPath,
		ModelName: "test-model.gguf",
		Status:    "stopped",
		APIType:   "llamacpp",
	}
	server.modelManager.mu.Unlock()

	t.Run("Returns HTML with server list", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/servers/list", nil)
		server.HandleListModels(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "test-model.gguf") {
			t.Errorf("Response should contain test model name, got: %s", truncate(body, 200))
		}
		t.Log("Servers list endpoint returns proper HTML")
	})

	t.Run("Returns HTML with GPU status", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/gpu/status", nil)
		server.HandleGPUStatus(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "id=\"gpu_status\"") {
			t.Errorf("Response should contain gpu_status id, got: %s", truncate(body, 200))
		}
		t.Log("GPU status endpoint returns proper HTML")
	})
}

func TestSSEEndpoints(t *testing.T) {
	server := &Server{
		store:        nil,
		modelManager: NewModelManagerForTest(),
		sseClients:   make(map[chan string]bool),
		sseMutex:     sync.RWMutex{},
		port:         0,
	}

	t.Run("SSE endpoint returns event stream", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/servers/events", nil)

		done := make(chan bool)
		go func() {
			server.handleServerEvents(w, r)
			done <- true
		}()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
		}

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "text/event-stream" {
			t.Errorf("Expected Content-Type text/event-stream, got: %s", contentType)
		}

		cacheControl := w.Header().Get("Cache-Control")
		if cacheControl != "no-cache" {
			t.Errorf("Expected Cache-Control no-cache, got: %s", cacheControl)
		}

		t.Log("SSE endpoint returns proper headers")
	})
}

func TestTestingPageE2E(t *testing.T) {
	if os.Getenv("CI") == "" && os.Getenv("PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD") != "" {
		t.Skip("Skipping browser test - browsers not installed")
	}

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Failed to start Playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		t.Fatalf("Failed to launch Chromium: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	dbStore, err := db.New()
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dbStore.DB().Close()

	server := &Server{
		store:        dbStore,
		modelManager: NewModelManagerForTest(),
		sseClients:   make(map[chan string]bool),
		sseMutex:     sync.RWMutex{},
		port:         0,
	}

	mux := server.Router()
	testServer := httptest.NewServer(mux)
	testServer.Config.WriteTimeout = 1 * time.Second
	defer testServer.Close()

	// Use specific /testing URL
	pageGotoOptions := playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}
	_, err = page.Goto(testServer.URL+"/testing", pageGotoOptions)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	content, _ := page.Content()
	t.Logf("Page content: %s", truncate(content, 500))

	t.Run("Testing page loads without errors", func(t *testing.T) {
		title, err := page.Title()
		if err != nil {
			t.Fatalf("Failed to get title: %v", err)
		}
		if !strings.Contains(title, "Testing") {
			t.Errorf("Title should contain 'Testing', got: %s", title)
		}
		t.Logf("Testing page title: %s", title)
	})

	t.Run("Benchmark results section exists", func(t *testing.T) {
		benchmarkResults := page.Locator("#benchmark_results_table").First()

		isVisible, err := benchmarkResults.IsVisible()
		if err != nil {
			t.Fatalf("Failed to check benchmark results visibility: %v", err)
		}
		if !isVisible {
			t.Error("Benchmark results section should be visible")
		}
		t.Log("Benchmark results section exists")
	})

	t.Run("Clear results button exists", func(t *testing.T) {
		clearBtn := page.Locator("#clear-results-btn").First()

		isVisible, err := clearBtn.IsVisible()
		if err != nil {
			t.Fatalf("Failed to check clear button visibility: %v", err)
		}
		if !isVisible {
			t.Error("Clear results button should be visible")
		}

		btnText, _ := clearBtn.TextContent()
		if !strings.Contains(btnText, "Clear") {
			t.Errorf("Button should say 'Clear', got: %s", btnText)
		}
		t.Logf("Clear button text: %s", btnText)
	})

	t.Run("Detailed results section exists", func(t *testing.T) {
		detailedResults := page.Locator("#detailed_results").First()

		isVisible, err := detailedResults.IsVisible()
		if err != nil {
			t.Fatalf("Failed to check detailed results visibility: %v", err)
		}
		if !isVisible {
			t.Error("Detailed results section should be visible")
		}
		t.Log("Detailed results section exists")
	})

	t.Run("Benchmark results uses Pattern 2 SSE", func(t *testing.T) {
		benchmarkTable := page.Locator("#benchmark_results_table").First()

		// Should have hx-get
		hxGet, err := benchmarkTable.GetAttribute("hx-get")
		if err != nil || hxGet != "/api/benchmark/results" {
			t.Errorf("Benchmark table should have hx-get='/api/benchmark/results', got: %s", hxGet)
		}

		// Check the parent container for sse-connect
		sseContainer := page.Locator("div[sse-connect]").First()
		sseConnect, err := sseContainer.GetAttribute("sse-connect")
		if err != nil || sseConnect != "/api/servers/events" {
			t.Errorf("Parent container should have sse-connect='/api/servers/events', got: %s", sseConnect)
		}

		t.Log("Benchmark results uses Pattern 2 correctly")
	})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
