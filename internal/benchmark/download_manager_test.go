package benchmark

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDownloadResume tests that downloads automatically resume after connection reset
func TestDownloadResume(t *testing.T) {
	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "download_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test content - split into two parts to simulate interrupted download
	fullContent := []byte("This is the full file content that will be downloaded in two parts")
	part1 := fullContent[:30] // First 30 bytes
	part2 := fullContent[30:] // Remaining bytes

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Check for Range header
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			// First request - no range header
			if requestCount != 1 {
				t.Errorf("Expected first request to have no Range header")
			}
			// Simulate connection reset by only sending part of the file
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			w.Write(part1)
			// Don't send rest - simulating connection reset
		} else {
			// Resume request - should have Range header (automatic retry)
			if requestCount != 2 {
				t.Errorf("Expected second request to have Range header")
			}

			// Parse range header: "bytes=30-"
			var start int64
			_, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			if err != nil {
				t.Errorf("Failed to parse Range header: %v", err)
				http.Error(w, "Invalid range", http.StatusBadRequest)
				return
			}

			if start != 30 {
				t.Errorf("Expected Range start=30, got %d", start)
			}

			// Send HTTP 206 Partial Content with remaining bytes
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(fullContent)-1, len(fullContent)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(part2)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(part2)
		}
	}))
	defer server.Close()

	// Create download manager
	dm := NewDownloadManager(tmpDir, nil)

	// Start download (will be interrupted then auto-resume)
	download, err := dm.StartDownload(server.URL + "/testfile.txt")
	if err != nil {
		t.Fatalf("Failed to start download: %v", err)
	}

	// Wait for first attempt + auto-retry to complete
	// First attempt: ~0s, retry after 2s backoff, complete by ~3s
	time.Sleep(4 * time.Second)

	// Verify download completed successfully via auto-retry
	if download.Status != "completed" {
		t.Errorf("Download status = %s (error: %s), want 'completed'", download.Status, download.Error)
	}

	// Verify file has full content
	finalPath := filepath.Join(tmpDir, "testfile.txt")
	finalContent, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("Failed to read final file: %v", err)
	}

	if string(finalContent) != string(fullContent) {
		t.Errorf("Final content mismatch.\nGot:  %q\nWant: %q", string(finalContent), string(fullContent))
	}

	if len(finalContent) != len(fullContent) {
		t.Errorf("Final file size = %d, want %d", len(finalContent), len(fullContent))
	}

	// Verify we made exactly 2 requests (initial + auto-retry resume)
	if requestCount != 2 {
		t.Errorf("Request count = %d, want 2", requestCount)
	}

	// Verify SupportsResume flag was set
	if !download.SupportsResume {
		t.Error("Expected SupportsResume to be true")
	}
}

// TestDownloadNoResumeSupport tests downloading from a server that doesn't support Range requests
func TestDownloadNoResumeSupport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "download_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fullContent := []byte("Full content from server without range support")

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Always return 200 OK with full content, even if Range header is present
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)))
		w.WriteHeader(http.StatusOK)
		w.Write(fullContent)
	}))
	defer server.Close()

	dm := NewDownloadManager(tmpDir, nil)

	// Create a partial file manually
	partialPath := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(partialPath, []byte("partial"), 0644); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}

	// Start download with existing partial file
	download, err := dm.StartDownload(server.URL + "/testfile.txt")
	if err != nil {
		t.Fatalf("Failed to start download: %v", err)
	}

	// Wait for download to complete
	time.Sleep(1 * time.Second)

	// Verify download completed
	if download.Status != "completed" {
		t.Errorf("Download status = %s, want 'completed'", download.Status)
	}

	// Verify file was overwritten with full content (not resumed)
	finalContent, err := os.ReadFile(partialPath)
	if err != nil {
		t.Fatalf("Failed to read final file: %v", err)
	}

	if string(finalContent) != string(fullContent) {
		t.Errorf("Content mismatch.\nGot:  %q\nWant: %q", string(finalContent), string(fullContent))
	}

	// Verify SupportsResume flag is false
	if download.SupportsResume {
		t.Error("Expected SupportsResume to be false for server without range support")
	}
}

// TestDownloadFromScratch tests a normal download without any partial file
func TestDownloadFromScratch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "download_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("Complete file content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check no Range header on fresh download
		if r.Header.Get("Range") != "" {
			t.Error("Expected no Range header on fresh download")
		}

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(content))
	}))
	defer server.Close()

	dm := NewDownloadManager(tmpDir, nil)
	download, err := dm.StartDownload(server.URL + "/newfile.txt")
	if err != nil {
		t.Fatalf("Failed to start download: %v", err)
	}

	// Wait for download
	time.Sleep(500 * time.Millisecond)

	// Verify completed
	if download.Status != "completed" {
		t.Errorf("Download status = %s, want 'completed'", download.Status)
	}

	// Verify content
	finalContent, err := os.ReadFile(filepath.Join(tmpDir, "newfile.txt"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(finalContent) != string(content) {
		t.Errorf("Content mismatch")
	}
}

// TestDownloadAutoRetry tests that downloads automatically retry on connection reset
func TestDownloadAutoRetry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "download_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test content - will be delivered in 3 chunks across retries
	fullContent := []byte("This is the full file content downloaded across multiple retries with connection resets")
	chunk1 := fullContent[:30]   // First chunk before reset
	chunk2 := fullContent[30:60] // Second chunk (resumed from 30, reset at 60)
	chunk3 := fullContent[60:]   // Final chunk (resumed from 60)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		rangeHeader := r.Header.Get("Range")

		if requestCount == 1 {
			// First request - no range, send chunk1 then reset
			if rangeHeader != "" {
				t.Errorf("Request 1: expected no Range header, got %s", rangeHeader)
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			w.Write(chunk1)
			// Connection reset - don't send rest
		} else if requestCount == 2 {
			// First retry - should resume from byte 30
			if rangeHeader != "bytes=30-" {
				t.Errorf("Request 2: expected Range: bytes=30-, got %s", rangeHeader)
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 30-%d/%d", len(fullContent)-1, len(fullContent)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(chunk2)+len(chunk3)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(chunk2)
			// Another connection reset
		} else if requestCount == 3 {
			// Second retry - should resume from byte 60
			if rangeHeader != "bytes=60-" {
				t.Errorf("Request 3: expected Range: bytes=60-, got %s", rangeHeader)
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 60-%d/%d", len(fullContent)-1, len(fullContent)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(chunk3)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(chunk3)
			// Success - full content delivered
		} else {
			t.Errorf("Unexpected request count: %d", requestCount)
		}
	}))
	defer server.Close()

	dm := NewDownloadManager(tmpDir, nil)
	download, err := dm.StartDownload(server.URL + "/retrytest.txt")
	if err != nil {
		t.Fatalf("Failed to start download: %v", err)
	}

	// Wait for download to complete with retries
	// First attempt: ~0s, retry 1: +2s, retry 2: +4s = ~6s total
	time.Sleep(8 * time.Second)

	// Verify completed successfully
	if download.Status != "completed" {
		t.Errorf("Download status = %s (error: %s), want 'completed'", download.Status, download.Error)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 HTTP requests (1 initial + 2 retries), got %d", requestCount)
	}

	// Verify full content was downloaded
	finalContent, err := os.ReadFile(filepath.Join(tmpDir, "retrytest.txt"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(finalContent) != string(fullContent) {
		t.Errorf("Content mismatch. Got %d bytes, want %d bytes", len(finalContent), len(fullContent))
		t.Errorf("Got: %q", string(finalContent))
		t.Errorf("Want: %q", string(fullContent))
	}

	// Verify partial file was preserved during retries
	if !download.SupportsResume {
		t.Error("Expected server to support resume")
	}
}

// TestDownloadRetryExhaustion tests that downloads fail after max retries
func TestDownloadRetryExhaustion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "download_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Always send partial content then reset - simulate persistent network issue
		w.Header().Set("Content-Length", "1000")
		w.Header().Set("Accept-Ranges", "bytes")

		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		w.Write([]byte("partial"))
		// Connection reset every time
	}))
	defer server.Close()

	dm := NewDownloadManager(tmpDir, nil)
	download, err := dm.StartDownload(server.URL + "/failtest.txt")
	if err != nil {
		t.Fatalf("Failed to start download: %v", err)
	}

	// Wait for all retries to exhaust
	// Initial + 5 retries with backoffs: 0s, +2s, +4s, +8s, +16s, +32s = ~62s
	// Use shorter timeout and check periodically
	timeout := time.After(65 * time.Second)
	ticker := time.Tick(1 * time.Second)

	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for download to fail after retries")
		case <-ticker:
			if download.Status == "failed" {
				goto done
			}
		}
	}

done:
	// Should have failed after max retries (1 initial + 5 retries = 6 total)
	if download.Status != "failed" {
		t.Errorf("Download status = %s, want 'failed' after retry exhaustion", download.Status)
	}

	if requestCount != 6 {
		t.Errorf("Expected 6 HTTP requests (1 initial + 5 retries), got %d", requestCount)
	}

	// Error should mention retry exhaustion
	if download.Error == "" {
		t.Error("Expected error message on failed download")
	}

	// Partial file should still exist
	partialFile := filepath.Join(tmpDir, "failtest.txt")
	if _, err := os.Stat(partialFile); os.IsNotExist(err) {
		t.Error("Expected partial file to be preserved after retry exhaustion")
	}
}
