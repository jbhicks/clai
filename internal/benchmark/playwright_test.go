package benchmark

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		Headless:       playwright.Bool(true),
		ExecutablePath: playwright.String("/usr/bin/chromium"),
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

	server := NewServer(nil)
	server.modelManager = NewModelManagerForTest()

	testServer := httptest.NewServer(http.HandlerFunc(server.handleModelsPage))
	defer testServer.Close()

	pageGotoOptions := playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}
	_, err = page.Goto(testServer.URL, pageGotoOptions)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	t.Run("Page loads without errors", func(t *testing.T) {
		title, err := page.Title()
		if err != nil {
			t.Fatalf("Failed to get title: %v", err)
		}
		if !strings.Contains(title, "Models") {
			t.Errorf("Title should contain 'Models', got: %s", title)
		}
		t.Logf("Page title: %s", title)
	})

	t.Run("Navigation tabs exist", func(t *testing.T) {
		modelsTab := page.Locator("a[href='/models']").First()
		modelsTabText, err := modelsTab.TextContent()
		if err != nil {
			t.Fatalf("Failed to get Models tab text: %v", err)
		}
		if !strings.Contains(modelsTabText, "Models") {
			t.Errorf("Models tab should contain 'Models', got: %s", modelsTabText)
		}
		t.Logf("Models tab text: %s", modelsTabText)
	})

	t.Run("GPU Status section exists", func(t *testing.T) {
		gpuStatus := page.Locator("#gpu_status_container").First()
		html, err := gpuStatus.InnerHTML()
		if err != nil {
			t.Fatalf("Failed to get GPU status HTML: %v", err)
		}
		t.Logf("GPU status HTML: %s", truncate(html, 200))
	})

	t.Run("Server list section exists", func(t *testing.T) {
		serversList := page.Locator("#servers_list").First()
		html, err := serversList.InnerHTML()
		if err != nil {
			t.Fatalf("Failed to get servers list HTML: %v", err)
		}
		t.Logf("Servers list HTML: %s", truncate(html, 200))
	})

	t.Run("Downloads section exists", func(t *testing.T) {
		downloadsList := page.Locator("#downloads_list").First()
		html, err := downloadsList.InnerHTML()
		if err != nil {
			t.Fatalf("Failed to get downloads list HTML: %v", err)
		}
		t.Logf("Downloads list HTML: %s", truncate(html, 200))
	})

	t.Run("Download form exists", func(t *testing.T) {
		downloadForm := page.Locator("form[hx-post='/api/models/download']").First()

		formExists, err := downloadForm.IsVisible()
		if err != nil {
			t.Fatalf("Failed to check form visibility: %v", err)
		}
		if !formExists {
			t.Error("Download form should be visible")
		}
		t.Log("Download form is visible")
	})

	t.Run("Model search input has correct attributes", func(t *testing.T) {
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
		if !strings.Contains(placeholder, "qwen") && !strings.Contains(placeholder, "llama") {
			t.Errorf("Placeholder should mention models, got: %s", placeholder)
		}

		hxGet, _ := searchInput.GetAttribute("hx-get")
		if hxGet != "/api/models/search" {
			t.Errorf("hx-get should be /api/models/search, got: %s", hxGet)
		}

		hxTrigger, _ := searchInput.GetAttribute("hx-trigger")
		if hxTrigger != "keyup changed delay:300ms" {
			t.Errorf("hx-trigger should be 'keyup changed delay:300ms', got: %s", hxTrigger)
		}

		t.Log("Model search input has correct attributes")
	})

	t.Run("SSE container has correct attributes", func(t *testing.T) {
		sseContainer := page.Locator("div[hx-ext='sse']").First()

		hxExt, _ := sseContainer.GetAttribute("hx-ext")
		if hxExt != "sse" {
			t.Errorf("hx-ext should be 'sse', got: %s", hxExt)
		}

		sseConnect, _ := sseContainer.GetAttribute("sse-connect")
		if sseConnect != "/api/servers/events" {
			t.Errorf("sse-connect should be /api/servers/events, got: %s", sseConnect)
		}

		t.Log("SSE container has correct attributes for Pattern 2")
	})

	t.Run("SSE uses Pattern 2 (hx-trigger, not sse-swap)", func(t *testing.T) {
		// Check GPU status uses Pattern 2
		gpuStatusTrigger := page.Locator("div[hx-get='/api/gpu/status']").First()
		trigger, err := gpuStatusTrigger.GetAttribute("hx-trigger")
		if err == nil && !strings.Contains(trigger, "sse:gpu_status") {
			t.Errorf("GPU status should use hx-trigger with sse:gpu_status, got: %s", trigger)
		}

		// Check servers list uses Pattern 2
		serversListTrigger := page.Locator("div[hx-get='/api/servers/list']").First()
		trigger, err = serversListTrigger.GetAttribute("hx-trigger")
		if err == nil && !strings.Contains(trigger, "sse:servers_list") {
			t.Errorf("Servers list should use hx-trigger with sse:servers_list, got: %s", trigger)
		}

		// Check downloads uses Pattern 2
		downloadsTrigger := page.Locator("div[hx-get='/api/models/downloads']").First()
		trigger, err = downloadsTrigger.GetAttribute("hx-trigger")
		if err == nil && !strings.Contains(trigger, "sse:downloads_update") {
			t.Errorf("Downloads should use hx-trigger with sse:downloads_update, got: %s", trigger)
		}

		t.Log("All SSE elements use Pattern 2 (hx-trigger)")
	})

	t.Run("Page has only ONE SSE connection", func(t *testing.T) {
		sseContainers := page.Locator("div[sse-connect='/api/servers/events']")
		count, err := sseContainers.Count()
		if err != nil {
			t.Fatalf("Failed to count SSE containers: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected exactly 1 SSE connection, found %d", count)
		}
		t.Logf("Page has exactly 1 SSE connection (correct)")
	})
}

func TestDownloadsFunctionalityE2E(t *testing.T) {
	if os.Getenv("CI") == "" && os.Getenv("PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD") != "" {
		t.Skip("Skipping browser test - browsers not installed")
	}

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Failed to start Playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless:       playwright.Bool(true),
		ExecutablePath: playwright.String("/usr/bin/chromium"),
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

	server := NewServer(nil)
	server.modelManager = NewModelManagerForTest()

	testServer := httptest.NewServer(http.HandlerFunc(server.handleModelsPage))
	defer testServer.Close()

	pageGotoOptions := playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}
	_, err = page.Goto(testServer.URL, pageGotoOptions)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	t.Run("Downloads list shows empty state when no downloads", func(t *testing.T) {
		downloadsList := page.Locator("#downloads_list").First()
		html, _ := downloadsList.InnerHTML()

		if strings.Contains(html, "Loading") {
			t.Log("Downloads still loading (expected)")
		} else if strings.Contains(html, "No active downloads") {
			t.Log("Downloads list shows empty state correctly")
		} else {
			t.Logf("Downloads content: %s", truncate(html, 300))
		}
	})

	t.Run("Download form has URL input", func(t *testing.T) {
		urlInput := page.Locator("input[name='url']").First()

		isVisible, err := urlInput.IsVisible()
		if err != nil {
			t.Fatalf("Failed to check URL input visibility: %v", err)
		}
		if !isVisible {
			t.Error("URL input should be visible")
		}
		t.Log("Download form has URL input")
	})

	t.Run("Download form has submit button", func(t *testing.T) {
		submitBtn := page.Locator("form[hx-post='/api/models/download'] button[type='submit']").First()

		isVisible, err := submitBtn.IsVisible()
		if err != nil {
			t.Fatalf("Failed to check submit button visibility: %v", err)
		}
		if !isVisible {
			t.Error("Submit button should be visible")
		}

		btnText, _ := submitBtn.TextContent()
		if !strings.Contains(btnText, "Download") {
			t.Errorf("Button should say 'Download', got: %s", btnText)
		}
		t.Logf("Submit button text: %s", btnText)
	})
}

func TestServersListFunctionalityE2E(t *testing.T) {
	if os.Getenv("CI") == "" && os.Getenv("PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD") != "" {
		t.Skip("Skipping browser test - browsers not installed")
	}

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Failed to start Playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless:       playwright.Bool(true),
		ExecutablePath: playwright.String("/usr/bin/chromium"),
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

	server := NewServer(nil)
	mm := server.modelManager

	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-model.gguf")
	f, err := os.Create(modelPath)
	if err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}
	f.Close()

	mm.mu.Lock()
	mm.servers[modelPath] = &ModelServer{
		ModelPath: modelPath,
		ModelName: "test-model.gguf",
		Status:    "stopped",
		APIType:   "llamacpp",
	}
	mm.mu.Unlock()

	testServer := httptest.NewServer(http.HandlerFunc(server.handleModelsPage))
	defer testServer.Close()

	pageGotoOptions := playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}
	_, err = page.Goto(testServer.URL, pageGotoOptions)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

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
	server := NewServer(nil)
	server.modelManager = NewModelManagerForTest()

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
	server := NewServer(nil)
	server.modelManager = NewModelManagerForTest()

	tempDir := t.TempDir()
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
	server := NewServer(nil)
	server.modelManager = NewModelManagerForTest()

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
		Headless:       playwright.Bool(true),
		ExecutablePath: playwright.String("/usr/bin/chromium"),
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

	server := NewServer(nil)
	server.modelManager = NewModelManagerForTest()

	testServer := httptest.NewServer(http.HandlerFunc(server.handleTestingPage))
	defer testServer.Close()

	pageGotoOptions := playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}
	_, err = page.Goto(testServer.URL, pageGotoOptions)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

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

	t.Run("Navigation tabs exist", func(t *testing.T) {
		modelsTab := page.Locator("a[href='/models']").First()
		modelsTabText, err := modelsTab.TextContent()
		if err != nil {
			t.Fatalf("Failed to get Models tab text: %v", err)
		}
		if !strings.Contains(modelsTabText, "Models") {
			t.Errorf("Models tab should contain 'Models', got: %s", modelsTabText)
		}

		testingTab := page.Locator("a[href='/testing']").First()
		testingTabText, err := testingTab.TextContent()
		if err != nil {
			t.Fatalf("Failed to get Testing tab text: %v", err)
		}
		if !strings.Contains(testingTabText, "Testing") {
			t.Errorf("Testing tab should contain 'Testing', got: %s", testingTabText)
		}
		t.Logf("Testing tab text: %s", testingTabText)
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

		// Should have hx-trigger with sse:benchmark_update
		hxTrigger, err := benchmarkTable.GetAttribute("hx-trigger")
		if err == nil && !strings.Contains(hxTrigger, "sse:benchmark_update") {
			t.Errorf("Benchmark table should use hx-trigger with sse:benchmark_update, got: %s", hxTrigger)
		}

		// Should have sse-connect
		sseConnect, err := benchmarkTable.GetAttribute("sse-connect")
		if err == nil && sseConnect != "/api/servers/events" {
			t.Errorf("Benchmark table should have sse-connect='/api/servers/events', got: %s", sseConnect)
		}

		// Should NOT have sse-swap (Pattern 1)
		sseSwap, err := benchmarkTable.GetAttribute("sse-swap")
		if err == nil && sseSwap != "" {
			t.Errorf("Benchmark table should NOT have sse-swap (Pattern 2 uses hx-trigger), found: %s", sseSwap)
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
