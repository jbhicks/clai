package types

import "os"
import "time"

// ServiceStatus represents the current status of the background service
type ServiceStatus struct {
	Status    string `json:"status"`
	Activity  string `json:"activity"`
	BuildID   string `json:"build_id"`
	PID       int    `json:"pid"`
	Timestamp int64  `json:"timestamp"`
}

// ModelServer represents a running or available model server
type ModelServer struct {
	ModelPath       string  `json:"model_path"`
	ModelName       string  `json:"model_name"`
	Port            int     `json:"port"`
	PID             int     `json:"pid"`
	Status          string  `json:"status"`        // "running", "loading", "stopped", "starting", "error"
	ErrorMessage    string  `json:"error_message"` // Error details when status is "error"
	URL             string  `json:"url"`
	APIType         string  `json:"api_type"`
	Backend         string  `json:"backend"` // "rocm" or "vulkan"
	LastChecked     int64   `json:"last_checked"`
	VRAMUsageBytes  int64   `json:"vram_usage_bytes"` // VRAM used by this server in bytes
	CPUPercent      float64 `json:"cpu_percent"`      // CPU usage percentage
	MemoryBytes     int64   `json:"memory_bytes"`     // RAM (RSS) used by this process in bytes
	ContextSize     int     `json:"context_size"`     // Active context window size (n_ctx)
	ContextTrain    int     `json:"context_train"`    // Training context size (n_ctx_train)
	ParametersCount int64   `json:"parameters_count"` // Total parameters (n_params)
	ModelSizeBytes  int64   `json:"model_size_bytes"` // Model file size in bytes
	VocabSize       int     `json:"vocab_size"`       // Vocabulary size (n_vocab)
	EmbeddingDim    int     `json:"embedding_dim"`    // Embedding dimensions (n_embd)
	NGL             int     `json:"ngl"`              // Number of GPU layers (n_gpu_layers)

	// Split model metadata
	IsSplitModel    bool     `json:"is_split_model"`    // True if this is a multi-part GGUF model
	SplitPartNumber int      `json:"split_part_number"` // Current part number (1-based)
	SplitTotalParts int      `json:"split_total_parts"` // Total number of parts
	SplitPartsFound int      `json:"split_parts_found"` // Number of parts found on disk
	SplitAllParts   []string `json:"split_all_parts"`   // Paths to all parts (for tracking)
	SplitIsComplete bool     `json:"split_is_complete"` // True if all parts are present
}

// BackendInfo holds information about a llama-server backend
type BackendInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Type    string `json:"type"` // "rocm" or "vulkan"
}

// NewServiceStatus creates a new ServiceStatus with current process info
func NewServiceStatus() ServiceStatus {
	return ServiceStatus{
		Status:    "running",
		Activity:  "Idle",
		PID:       os.Getpid(),
		Timestamp: time.Now().Unix(),
	}
}
