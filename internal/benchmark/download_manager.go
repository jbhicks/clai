package benchmark

import (
	"clai/internal/logger"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Download represents an active or completed model download
type Download struct {
	ID              string     `json:"id"`
	URL             string     `json:"url"`
	Filename        string     `json:"filename"`
	Status          string     `json:"status"` // downloading, completed, failed
	Progress        float64    `json:"progress"`
	BytesDownloaded int64      `json:"bytes_downloaded"`
	TotalBytes      int64      `json:"total_bytes"`
	Speed           int64      `json:"speed"` // bytes per second
	Error           string     `json:"error,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// DownloadManager manages model downloads
type DownloadManager struct {
	downloads map[string]*Download
	mu        sync.RWMutex
	listeners []chan *Download // SSE listeners
	modelsDir string
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(modelsDir string) *DownloadManager {
	return &DownloadManager{
		downloads: make(map[string]*Download),
		listeners: make([]chan *Download, 0),
		modelsDir: modelsDir,
	}
}

// StartDownload starts downloading a model from a URL
func (dm *DownloadManager) StartDownload(downloadURL string) (*Download, error) {
	// Parse URL to extract filename
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Extract filename from URL path
	filename := filepath.Base(parsedURL.Path)
	if filename == "" || filename == "." {
		return nil, fmt.Errorf("could not extract filename from URL")
	}

	// Generate unique ID
	downloadID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create download record
	download := &Download{
		ID:        downloadID,
		URL:       downloadURL,
		Filename:  filename,
		Status:    "downloading",
		Progress:  0,
		StartedAt: time.Now(),
	}

	dm.mu.Lock()
	dm.downloads[downloadID] = download
	dm.mu.Unlock()

	// Notify listeners
	dm.notifyListeners(download)

	// Start download in background
	go dm.downloadFile(downloadID)

	return download, nil
}

// downloadFile performs the actual download
func (dm *DownloadManager) downloadFile(downloadID string) {
	dm.mu.RLock()
	download, exists := dm.downloads[downloadID]
	dm.mu.RUnlock()

	if !exists {
		return
	}

	// Create destination file
	destPath := filepath.Join(dm.modelsDir, download.Filename)
	out, err := os.Create(destPath)
	if err != nil {
		dm.markFailed(downloadID, fmt.Sprintf("Failed to create file: %v", err))
		return
	}
	defer out.Close()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 0, // No timeout for downloads
	}

	// Make request
	resp, err := client.Get(download.URL)
	if err != nil {
		dm.markFailed(downloadID, fmt.Sprintf("Failed to download: %v", err))
		os.Remove(destPath) // Clean up partial file
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dm.markFailed(downloadID, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status))
		os.Remove(destPath)
		return
	}

	// Get total size
	totalBytes := resp.ContentLength
	dm.mu.Lock()
	download.TotalBytes = totalBytes
	dm.mu.Unlock()

	// Download with progress tracking
	var bytesDownloaded int64
	lastUpdate := time.Now()
	lastBytes := int64(0)

	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				dm.markFailed(downloadID, fmt.Sprintf("Write error: %v", writeErr))
				os.Remove(destPath)
				return
			}

			bytesDownloaded += int64(n)

			// Update progress every second
			now := time.Now()
			if now.Sub(lastUpdate) >= time.Second {
				elapsed := now.Sub(lastUpdate).Seconds()
				speed := int64(float64(bytesDownloaded-lastBytes) / elapsed)

				dm.mu.Lock()
				download.BytesDownloaded = bytesDownloaded
				if totalBytes > 0 {
					download.Progress = float64(bytesDownloaded) / float64(totalBytes) * 100
				}
				download.Speed = speed
				dm.mu.Unlock()

				dm.notifyListeners(download)

				lastUpdate = now
				lastBytes = bytesDownloaded
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			dm.markFailed(downloadID, fmt.Sprintf("Read error: %v", err))
			os.Remove(destPath)
			return
		}
	}

	// Mark as completed
	completedAt := time.Now()
	dm.mu.Lock()
	download.Status = "completed"
	download.Progress = 100
	download.BytesDownloaded = bytesDownloaded
	download.CompletedAt = &completedAt
	download.Speed = 0
	dm.mu.Unlock()

	dm.notifyListeners(download)
	log.Printf("Download completed: %s (%s)", download.Filename, formatBytes(bytesDownloaded))
}

// markFailed marks a download as failed
func (dm *DownloadManager) markFailed(downloadID string, errorMsg string) {
	dm.mu.Lock()
	download, exists := dm.downloads[downloadID]
	if exists {
		download.Status = "failed"
		download.Error = errorMsg
		completedAt := time.Now()
		download.CompletedAt = &completedAt
	}
	dm.mu.Unlock()

	if exists {
		dm.notifyListeners(download)
		log.Printf("Download failed: %s - %s", download.Filename, errorMsg)
	}
}

// GetDownloads returns all downloads
func (dm *DownloadManager) GetDownloads() []*Download {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	downloads := make([]*Download, 0, len(dm.downloads))
	for _, d := range dm.downloads {
		downloads = append(downloads, d)
	}
	return downloads
}

// GetDownload returns a specific download
func (dm *DownloadManager) GetDownload(id string) (*Download, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	d, exists := dm.downloads[id]
	return d, exists
}

// AddListener adds an SSE listener
func (dm *DownloadManager) AddListener(ch chan *Download) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.listeners = append(dm.listeners, ch)
}

// RemoveListener removes an SSE listener
func (dm *DownloadManager) RemoveListener(ch chan *Download) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for i, listener := range dm.listeners {
		if listener == ch {
			dm.listeners = append(dm.listeners[:i], dm.listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// notifyListeners sends download updates to all SSE listeners
func (dm *DownloadManager) notifyListeners(download *Download) {
	dm.mu.RLock()
	listenerCount := len(dm.listeners)
	dm.mu.RUnlock()

	if listenerCount > 0 {
		log.Printf("Notifying %d SSE listeners about download update: %s (%.1f%%)",
			listenerCount, download.Filename, download.Progress)
	}

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, ch := range dm.listeners {
		select {
		case ch <- download:
		default:
			// Skip if channel is full
			log.Printf("Warning: SSE listener channel full, skipping update")
		}
	}
}

// formatBytes formats bytes to human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// RepoFile represents a file in a HuggingFace repository
type RepoFile struct {
	Name string
	URL  string
	Size int64
}

// fetchRepoFiles fetches the list of .gguf files from a HuggingFace repository
func fetchRepoFiles(repoID string) ([]RepoFile, error) {
	// HuggingFace API endpoint for listing files
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", repoID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repository not found (HTTP %d)", resp.StatusCode)
	}

	var files []struct {
		Type string `json:"type"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to parse repository files: %w", err)
	}

	// Filter for .gguf files only
	var ggufFiles []RepoFile
	for _, f := range files {
		if f.Type == "file" && strings.HasSuffix(strings.ToLower(f.Path), ".gguf") {
			ggufFiles = append(ggufFiles, RepoFile{
				Name: f.Path,
				URL:  fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, f.Path),
				Size: f.Size,
			})
		}
	}

	if len(ggufFiles) == 0 {
		return nil, fmt.Errorf("no .gguf files found in repository")
	}

	return ggufFiles, nil
}

// Server handlers for downloads
func (s *Server) handleDownloadModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	downloadURL := r.FormValue("url")
	if downloadURL == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div style="background: #fee2e2; border: 1px solid #ef4444; color: #991b1b; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
			<strong>Error:</strong> URL is required
		</div>`)
		return
	}

	// Check if input is a repository name (e.g., "user/repo") or a full URL
	if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		// It's a repository name - fetch available .gguf files
		files, err := fetchRepoFiles(downloadURL)
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div style="background: #fee2e2; border: 1px solid #ef4444; color: #991b1b; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
				<strong>Error:</strong> Failed to fetch files from repository: %s
			</div>`, htmlEscape(err.Error()))
			return
		}

		// Return file selection UI
		w.Header().Set("Content-Type", "text/html")
		html := `<div style="background: #1e293b; border: 1px solid #334155; padding: 16px; border-radius: 6px; margin-bottom: 15px;">
			<h4 style="margin: 0 0 12px 0; color: #e2e8f0; font-size: 15px;">Select a file to download from ` + htmlEscape(downloadURL) + `:</h4>
			<div style="display: flex; flex-direction: column; gap: 8px; max-height: 300px; overflow-y: auto;">`

		for _, file := range files {
			sizeText := formatBytes(int64(file.Size))
			html += fmt.Sprintf(`
				<button 
					hx-post="/api/models/download"
					hx-vals='{"url": "%s"}'
					hx-target="#download_status"
					hx-swap="innerHTML"
					style="background: #0f172a; border: 1px solid #475569; padding: 10px 12px; border-radius: 4px; color: #e2e8f0; text-align: left; cursor: pointer; transition: all 0.2s;"
					onmouseover="this.style.background='#1e293b'; this.style.borderColor='#64748b'"
					onmouseout="this.style.background='#0f172a'; this.style.borderColor='#475569'"
				>
					<div style="font-weight: 500;">%s</div>
					<div style="font-size: 11px; color: #94a3b8; margin-top: 4px;">%s</div>
				</button>`,
				file.URL,
				htmlEscape(file.Name),
				sizeText,
			)
		}

		html += `</div></div>`

		fmt.Fprint(w, html)
		return
	}

	// It's a direct URL - validate it's from Hugging Face
	if !strings.Contains(downloadURL, "huggingface.co") {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div style="background: #fee2e2; border: 1px solid #ef4444; color: #991b1b; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
			<strong>Error:</strong> Only Hugging Face URLs are supported
		</div>`)
		return
	}

	download, err := s.modelManager.downloadManager.StartDownload(downloadURL)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div style="background: #fee2e2; border: 1px solid #ef4444; color: #991b1b; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
			<strong>Error:</strong> %s
		</div>`, htmlEscape(err.Error()))
		return
	}

	// Return success message
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div style="background: #d1fae5; border: 1px solid #10b981; color: #065f46; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
		<strong>Success:</strong> Download started for %s
	</div>`, htmlEscape(download.Filename))
}

func (s *Server) handleGetDownloads(w http.ResponseWriter, r *http.Request) {
	allDownloads := s.modelManager.downloadManager.GetDownloads()

	// Filter to only show active downloads (not completed or failed)
	var downloads []*Download
	for _, d := range allDownloads {
		if d.Status == "downloading" {
			downloads = append(downloads, d)
		}
	}

	w.Header().Set("Content-Type", "text/html")

	if len(downloads) == 0 {
		fmt.Fprint(w, `<div id="downloads_list">
			<p style="color: #64748b; font-size: 13px; text-align: center; padding: 20px;">No active downloads</p>
		</div>`)
		return
	}

	html := `<div id="downloads_list">
		<h3 style="margin-bottom: 15px; font-size: 16px; color: #e2e8f0;">Active Downloads</h3>
		<div style="display: flex; flex-direction: column; gap: 12px;">`

	for _, d := range downloads {
		statusColor := "#3b82f6" // blue for downloading
		if d.Status == "completed" {
			statusColor = "#10b981" // green
		} else if d.Status == "failed" {
			statusColor = "#ef4444" // red
		}

		progressBar := ""
		if d.TotalBytes > 0 {
			// Calculate ETA if we have speed
			etaText := ""
			if d.Speed > 0 {
				remainingBytes := d.TotalBytes - d.BytesDownloaded
				etaSeconds := float64(remainingBytes) / float64(d.Speed)
				if etaSeconds < 60 {
					etaText = fmt.Sprintf(" • ETA: %ds", int(etaSeconds))
				} else if etaSeconds < 3600 {
					etaText = fmt.Sprintf(" • ETA: %dm %ds", int(etaSeconds/60), int(etaSeconds)%60)
				} else {
					etaText = fmt.Sprintf(" • ETA: %dh %dm", int(etaSeconds/3600), (int(etaSeconds)%3600)/60)
				}
			}

			progressBar = fmt.Sprintf(`
				<div style="background: #0f172a; border-radius: 4px; height: 8px; margin: 8px 0; overflow: hidden; position: relative;">
					<div style="background: %s; height: 100%%; width: %.1f%%; transition: width 0.5s ease; position: relative; overflow: hidden;">
						<div style="position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent); animation: shimmer 2s infinite;"></div>
					</div>
				</div>
				<style>
				@keyframes shimmer {
					0%% { transform: translateX(-100%%); }
					100%% { transform: translateX(100%%); }
				}
				</style>
				<div style="display: flex; justify-content: space-between; font-size: 11px; color: #64748b;">
					<span>%s / %s%s</span>
					<span style="color: %s; font-weight: 600;">%.1f%%</span>
				</div>`,
				statusColor,
				d.Progress,
				formatBytes(d.BytesDownloaded),
				formatBytes(d.TotalBytes),
				etaText,
				statusColor,
				d.Progress,
			)
		}

		speedText := ""
		if d.Speed > 0 {
			speedText = fmt.Sprintf(" • %s/s", formatBytes(d.Speed))
		}

		errorText := ""
		if d.Error != "" {
			errorText = fmt.Sprintf(`<div style="color: #ef4444; font-size: 11px; margin-top: 4px;">%s</div>`, htmlEscape(d.Error))
		}

		html += fmt.Sprintf(`
			<div style="background: #0f172a; border: 1px solid #334155; border-radius: 6px; padding: 12px;">
				<div style="display: flex; align-items: center; gap: 10px; margin-bottom: 6px;">
					<div style="width: 8px; height: 8px; background: %s; border-radius: 50%%;"></div>
					<span style="color: #e2e8f0; font-size: 13px; font-weight: 500;">%s</span>
					<span style="color: #64748b; font-size: 11px; margin-left: auto;">%s%s</span>
				</div>
				%s
				%s
			</div>`,
			statusColor,
			htmlEscape(d.Filename),
			d.Status,
			speedText,
			progressBar,
			errorText,
		)
	}

	html += `</div></div>`
	fmt.Fprint(w, html)
}

func (s *Server) handleDownloadsSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create channel for this client
	clientChan := make(chan *Download, 10)
	s.modelManager.downloadManager.AddListener(clientChan)
	defer s.modelManager.downloadManager.RemoveListener(clientChan)

	log.Printf("SSE client connected to downloads stream")

	// Stream updates
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial ping to establish connection
	fmt.Fprintf(w, "event: connected\ndata: ready\n\n")
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for {
		select {
		case <-clientChan:
			// Send SSE event - trigger HTMX to refresh the downloads list
			log.Printf("Sending downloads_update SSE event")
			fmt.Fprintf(w, "event: downloads_update\ndata: refresh\n\n")
			flusher.Flush()

		case <-ctx.Done():
			log.Printf("SSE client disconnected from downloads stream")
			return
		}
	}
}

func (s *Server) handleGetDetailedTestResults(w http.ResponseWriter, r *http.Request) {
	// Get run_id from query parameter
	runIDStr := r.FormValue("run_id")

	if runIDStr == "" {
		// No run selected yet
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div id="detailed_results">
			<p style="color: #94a3b8; text-align: center; padding: 40px; font-size: 14px;">
				👆 Click on a benchmark run above to view detailed test results
			</p>
		</div>`)
		return
	}

	// Query all agentic benchmark results for this specific run
	query := `
		SELECT 
			r.id, r.test_name, r.task_description, r.passed, r.duration_ms,
			r.validation_reason, r.error_message, r.created_at,
			b.model_name, b.id as run_id
		FROM agentic_benchmark_results r
		JOIN agentic_benchmark_runs b ON r.run_id = b.id
		WHERE b.id = ?
		ORDER BY r.id ASC
	`

	rows, err := s.store.DB().Query(query, runIDStr)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to query detailed results: %v", err)
		log.Printf("%s", errMsg)
		logger.Error(errMsg)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div id="detailed_results">
			<p style="color: #ef4444; font-size: 13px; text-align: center; padding: 40px;">
				Error loading results: %s
			</p>
		</div>`, err.Error())
		return
	}
	defer rows.Close()

	// Build HTML table
	w.Header().Set("Content-Type", "text/html")
	html := `<div id="detailed_results">
		<table style="width: 100%; border-collapse: collapse;">
			<thead>
				<tr style="border-bottom: 1px solid #475569;">
					<th style="padding: 10px; text-align: left; color: #94a3b8; font-size: 13px;">Test</th>
					<th style="padding: 10px; text-align: left; color: #94a3b8; font-size: 13px;">Model</th>
					<th style="padding: 10px; text-align: center; color: #94a3b8; font-size: 13px;">Status</th>
					<th style="padding: 10px; text-align: right; color: #94a3b8; font-size: 13px;">Duration</th>
					<th style="padding: 10px; text-align: left; color: #94a3b8; font-size: 13px;">Result</th>
				</tr>
			</thead>
			<tbody>
	`

	hasResults := false
	for rows.Next() {
		hasResults = true
		var (
			id, runID, durationMs          int
			testName, taskDesc, modelName  string
			passed                         bool
			validationReason, errorMessage sql.NullString
			createdAt                      string
		)

		err := rows.Scan(&id, &testName, &taskDesc, &passed, &durationMs,
			&validationReason, &errorMessage, &createdAt, &modelName, &runID)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to scan result row: %v", err)
			log.Printf("%s", errMsg)
			logger.Error(errMsg)
			continue
		}

		// Extract values from NullString, use empty string if NULL
		validationReasonStr := ""
		if validationReason.Valid {
			validationReasonStr = validationReason.String
		}
		errorMessageStr := ""
		if errorMessage.Valid {
			errorMessageStr = errorMessage.String
		}

		statusBadge := `<span style="color: #10b981; font-weight: 600;">✓ Pass</span>`
		statusColor := "#10b981"
		resultText := validationReasonStr
		if !passed {
			statusBadge = `<span style="color: #ef4444; font-weight: 600;">✗ Fail</span>`
			statusColor = "#ef4444"
			if errorMessageStr != "" {
				resultText = errorMessageStr
			}
		}

		// Truncate long result text
		if len(resultText) > 80 {
			resultText = resultText[:77] + "..."
		}

		html += fmt.Sprintf(`
			<tr 
				style="border-bottom: 1px solid #334155; cursor: pointer;"
				hx-get="/api/test/result/detail?test_id=%d"
				hx-target="#test_detail_content"
				hx-swap="innerHTML"
				onclick="document.getElementById('test_detail_modal').style.display='block'"
				onmouseover="this.style.backgroundColor='#1e293b'"
				onmouseout="this.style.backgroundColor='transparent'"
			>
				<td style="padding: 10px;">
					<div style="color: #e2e8f0; font-weight: 500; font-size: 13px;">%s</div>
					<div style="color: #64748b; font-size: 11px; margin-top: 2px;">%s</div>
				</td>
				<td style="padding: 10px; color: #cbd5e1; font-size: 12px;">%s</td>
				<td style="padding: 10px; text-align: center;">%s</td>
				<td style="padding: 10px; text-align: right; color: #94a3b8; font-size: 12px;">%dms</td>
				<td style="padding: 10px; color: %s; font-size: 12px;">%s</td>
			</tr>`,
			id,
			htmlEscape(testName),
			htmlEscape(taskDesc),
			htmlEscape(modelName),
			statusBadge,
			durationMs,
			statusColor,
			htmlEscape(resultText),
		)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating results: %v", err)
		html += fmt.Sprintf(`
			<tr>
				<td colspan="5" style="padding: 20px; text-align: center; color: #ef4444; font-size: 13px;">
					Error reading results: %s
				</td>
			</tr>
		`, htmlEscape(err.Error()))
	} else if !hasResults {
		html += `
			<tr>
				<td colspan="5" style="padding: 40px; text-align: center; color: #64748b; font-size: 13px;">
					No test results found for this benchmark run.
				</td>
			</tr>
		`
	}

	logMsg := fmt.Sprintf("Detailed results query completed: hasResults=%v, run_id=%s",
		hasResults, runIDStr)
	log.Printf("%s", logMsg)
	logger.Info(logMsg)

	html += `
			</tbody>
		</table>
	</div>`

	fmt.Fprint(w, html)
}

func (s *Server) handleGetTestDetail(w http.ResponseWriter, r *http.Request) {
	testIDStr := r.FormValue("test_id")
	if testIDStr == "" {
		http.Error(w, "Missing test_id parameter", http.StatusBadRequest)
		return
	}

	// Query the specific test result
	query := `
		SELECT 
			r.id, r.test_name, r.task_description, r.prompt, 
			r.generated_code, r.execution_output, r.expected_result,
			r.passed, r.validation_reason, r.error_message, r.duration_ms,
			b.model_name
		FROM agentic_benchmark_results r
		JOIN agentic_benchmark_runs b ON r.run_id = b.id
		WHERE r.id = ?
	`

	var (
		id, durationMs                             int
		testName, taskDesc, prompt, expectedResult string
		modelName                                  string
		passed                                     bool
		generatedCode, executionOutput             sql.NullString
		validationReason, errorMessage             sql.NullString
	)

	err := s.store.DB().QueryRow(query, testIDStr).Scan(
		&id, &testName, &taskDesc, &prompt,
		&generatedCode, &executionOutput, &expectedResult,
		&passed, &validationReason, &errorMessage, &durationMs,
		&modelName,
	)

	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch test detail: %v", err)
		log.Printf("%s", errMsg)
		logger.Error(errMsg)
		http.Error(w, "Test not found", http.StatusNotFound)
		return
	}

	// Extract values from NullString
	codeStr := ""
	if generatedCode.Valid {
		codeStr = generatedCode.String
	}
	outputStr := ""
	if executionOutput.Valid {
		outputStr = executionOutput.String
	}
	validationReasonStr := ""
	if validationReason.Valid {
		validationReasonStr = validationReason.String
	}
	errorMessageStr := ""
	if errorMessage.Valid {
		errorMessageStr = errorMessage.String
	}

	// Build HTML for modal content
	w.Header().Set("Content-Type", "text/html")

	statusBadge := `<span style="color: #10b981; font-weight: 600; font-size: 16px;">✓ PASSED</span>`
	if !passed {
		statusBadge = `<span style="color: #ef4444; font-weight: 600; font-size: 16px;">✗ FAILED</span>`
	}

	html := fmt.Sprintf(`
		<div style="margin-bottom: 20px;">
			<h4 style="color: #e2e8f0; margin-bottom: 10px; font-size: 18px;">%s</h4>
			<div style="display: flex; gap: 20px; margin-bottom: 10px;">
				<div><strong style="color: #94a3b8;">Model:</strong> <span style="color: #cbd5e1;">%s</span></div>
				<div><strong style="color: #94a3b8;">Duration:</strong> <span style="color: #cbd5e1;">%dms</span></div>
				<div>%s</div>
			</div>
			<p style="color: #94a3b8; font-size: 14px; margin-top: 10px;">%s</p>
		</div>

		<div style="margin-bottom: 20px;">
			<h5 style="color: #8b5cf6; margin-bottom: 10px; font-size: 15px;">📝 Prompt Sent to Model</h5>
			<pre style="background: #1e293b; padding: 15px; border-radius: 6px; border: 1px solid #334155; color: #e2e8f0; white-space: pre-wrap; font-size: 13px; line-height: 1.6;">%s</pre>
		</div>
	`,
		htmlEscape(testName),
		htmlEscape(modelName),
		durationMs,
		statusBadge,
		htmlEscape(taskDesc),
		htmlEscape(prompt),
	)

	if codeStr != "" {
		html += fmt.Sprintf(`
		<div style="margin-bottom: 20px;">
			<h5 style="color: #8b5cf6; margin-bottom: 10px; font-size: 15px;">💻 Generated Code</h5>
			<pre style="background: #1e293b; padding: 15px; border-radius: 6px; border: 1px solid #334155; color: #a5f3fc; white-space: pre-wrap; font-size: 13px; line-height: 1.6; font-family: 'Monaco', 'Courier New', monospace;">%s</pre>
		</div>
		`, htmlEscape(codeStr))
	}

	if outputStr != "" {
		html += fmt.Sprintf(`
		<div style="margin-bottom: 20px;">
			<h5 style="color: #8b5cf6; margin-bottom: 10px; font-size: 15px;">📤 Execution Output</h5>
			<pre style="background: #1e293b; padding: 15px; border-radius: 6px; border: 1px solid #334155; color: #6ee7b7; white-space: pre-wrap; font-size: 13px; line-height: 1.6;">%s</pre>
		</div>
		`, htmlEscape(outputStr))
	}

	if expectedResult != "" {
		html += fmt.Sprintf(`
		<div style="margin-bottom: 20px;">
			<h5 style="color: #8b5cf6; margin-bottom: 10px; font-size: 15px;">✅ Expected Result</h5>
			<pre style="background: #1e293b; padding: 15px; border-radius: 6px; border: 1px solid #334155; color: #cbd5e1; white-space: pre-wrap; font-size: 13px; line-height: 1.6;">%s</pre>
		</div>
		`, htmlEscape(expectedResult))
	}

	if passed && validationReasonStr != "" {
		html += fmt.Sprintf(`
		<div style="margin-bottom: 20px;">
			<h5 style="color: #10b981; margin-bottom: 10px; font-size: 15px;">✓ Validation Result</h5>
			<p style="color: #6ee7b7; background: #1e293b; padding: 15px; border-radius: 6px; border: 1px solid #334155; font-size: 13px;">%s</p>
		</div>
		`, htmlEscape(validationReasonStr))
	}

	if !passed && errorMessageStr != "" {
		html += fmt.Sprintf(`
		<div style="margin-bottom: 20px;">
			<h5 style="color: #ef4444; margin-bottom: 10px; font-size: 15px;">❌ Error Details</h5>
			<pre style="color: #fca5a5; background: #1e293b; padding: 15px; border-radius: 6px; border: 1px solid #334155; white-space: pre-wrap; font-size: 13px; line-height: 1.6;">%s</pre>
		</div>
		`, htmlEscape(errorMessageStr))
	}

	fmt.Fprint(w, html)
}

// handleDownloadGroup handles downloading multiple GGUF files as a group (for multi-part models)
func (s *Server) handleDownloadGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	urlsStr := r.FormValue("urls")
	modelID := r.FormValue("model_id")
	quantization := r.FormValue("quantization")

	if urlsStr == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div style="background: #fee2e2; border: 1px solid #ef4444; color: #991b1b; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
			<strong>Error:</strong> No URLs provided
		</div>`)
		return
	}

	// Split comma-separated URLs
	urls := strings.Split(urlsStr, ",")

	// Validate all URLs
	for _, url := range urls {
		if !strings.Contains(url, "huggingface.co") {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div style="background: #fee2e2; border: 1px solid #ef4444; color: #991b1b; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
				<strong>Error:</strong> Invalid URL: %s
			</div>`, htmlEscape(url))
			return
		}
	}

	// Start downloads for all URLs
	var downloads []*Download
	for _, url := range urls {
		download, err := s.modelManager.downloadManager.StartDownload(strings.TrimSpace(url))
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div style="background: #fee2e2; border: 1px solid #ef4444; color: #991b1b; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
				<strong>Error:</strong> Failed to start download for %s: %s
			</div>`, htmlEscape(url), htmlEscape(err.Error()))
			return
		}
		downloads = append(downloads, download)
	}

	// Return success message
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div style="background: #d1fae5; border: 1px solid #10b981; color: #065f46; padding: 12px; border-radius: 6px; margin-bottom: 15px;">
		<strong>Success:</strong> Started downloading %d files for %s (%s)
		<div style="margin-top: 8px; font-size: 11px; color: #047857;">
			%s
		</div>
	</div>`, len(downloads), htmlEscape(modelID), htmlEscape(quantization),
		func() string {
			var parts []string
			for _, d := range downloads {
				parts = append(parts, d.Filename)
			}
			return strings.Join(parts, "<br>• ")
		}())
}
