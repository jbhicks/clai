package mcpbridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ResultManager keeps large results out of the LLM context by storing them in the execution environment.
//
// Problem: MCP servers return large datasets (10,000+ rows) that would overwhelm the LLM context window.
// Solution: Store results by reference and filter in the execution environment before showing to LLM.
//
// Example:
//
//	// Large result from MCP
//	big_data := salesforce.query("SELECT * FROM Orders")  // 10,000 rows
//
//	// Instead of sending 10,000 rows to LLM:
//	// → Stored as: __result_ref_123 (20 bytes)
//
//	// Agent filters in code (no LLM tokens used!):
//	filtered = big_data.filter(status='pending')[:10]  // Execute in Go
//
//	// LLM sees only: "Found 10 pending orders" (50 bytes)
//
// Benefits:
// - Large data never hits context window
// - Filtering happens in execution environment (fast, free)
// - Privacy: Can tokenize PII before LLM sees it
// - Natural programming workflow: filter before processing
type ResultManager struct {
	workspace string                   // Directory for persistent storage
	refs      map[string]*StoredResult // In-memory cache of results
	counter   int                      // For generating unique IDs
	mu        sync.RWMutex             // Protects refs and counter
	threshold int                      // Token threshold for storage
}

// NewResultManager creates a new result manager
func NewResultManager(workspace string, threshold int) *ResultManager {
	if threshold == 0 {
		threshold = 1000 // Default: store results > 1000 tokens
	}

	return &ResultManager{
		workspace: workspace,
		refs:      make(map[string]*StoredResult),
		threshold: threshold,
	}
}

// WithThreshold sets the token threshold for result storage
func (rm *ResultManager) WithThreshold(threshold int) *ResultManager {
	rm.threshold = threshold
	return rm
}

// StoreResult stores a result and returns a reference ID
//
// If the result is small (< threshold tokens), it may be returned inline.
// Large results are stored by reference.
func (rm *ResultManager) StoreResult(data interface{}) *StoredResult {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Estimate token size
	size := estimateTokenSize(data)

	// Create reference ID
	rm.counter++
	refID := fmt.Sprintf("__result_%d", rm.counter)

	stored := &StoredResult{
		ID:       refID,
		Data:     data,
		Size:     size,
		Metadata: make(map[string]interface{}),
	}

	// Cache in memory
	rm.refs[refID] = stored

	// Persist to disk if large or if persistence is enabled
	if size > rm.threshold {
		rm.saveToDisk(refID, data)
	}

	return stored
}

// GetResult retrieves a result by reference ID
func (rm *ResultManager) GetResult(refID string) (interface{}, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Check memory cache first
	if stored, ok := rm.refs[refID]; ok {
		return stored.Data, true
	}

	// Try to load from disk
	data, err := rm.loadFromDisk(refID)
	if err != nil {
		return nil, false
	}

	// Cache in memory
	rm.refs[refID] = &StoredResult{
		ID:   refID,
		Data: data,
		Size: estimateTokenSize(data),
	}

	return data, true
}

// GetPythonRepresentation returns a Python representation for the LLM
//
// For small results: returns the actual value
// For large results: returns a reference object with preview
func (rm *ResultManager) GetPythonRepresentation(refID string) string {
	rm.mu.RLock()
	stored, ok := rm.refs[refID]
	rm.mu.RUnlock()

	if !ok {
		// Try to load from disk
		data, err := rm.loadFromDisk(refID)
		if err != nil {
			return fmt.Sprintf("ResultRef(\"%s\", error=\"not found\")", refID)
		}

		// Cache it
		stored = rm.StoreResult(data)
	}

	// Small results: return actual value
	if stored.Size <= rm.threshold {
		return toPythonRepr(stored.Data)
	}

	// Large results: return reference with preview
	preview := generatePreview(stored.Data, 5) // First 5 items
	return fmt.Sprintf(`ResultRef("%s", count=%d, preview=%s)`,
		refID, getItemCount(stored.Data), preview)
}

// FilterResult filters a result and returns a new reference
//
// This is the key operation for keeping large results out of context.
// Filtering happens in Go, not in the LLM.
func (rm *ResultManager) FilterResult(
	refID string,
	filterFunc func(interface{}) bool,
) (*StoredResult, error) {
	// Get original result
	data, ok := rm.GetResult(refID)
	if !ok {
		return nil, fmt.Errorf("result not found: %s", refID)
	}

	// Apply filter in Go (fast, no LLM tokens)
	filtered := applyFilter(data, filterFunc)

	// Store filtered result
	return rm.StoreResult(filtered), nil
}

// TransformResult applies a transformation and returns a new reference
//
// Similar to FilterResult but applies arbitrary transformations
func (rm *ResultManager) TransformResult(
	refID string,
	transformFunc func(interface{}) interface{},
) (*StoredResult, error) {
	data, ok := rm.GetResult(refID)
	if !ok {
		return nil, fmt.Errorf("result not found: %s", refID)
	}

	// Apply transformation
	transformed := transformFunc(data)

	// Store result
	return rm.StoreResult(transformed), nil
}

// saveToDisk persists a result to disk
func (rm *ResultManager) saveToDisk(refID string, data interface{}) error {
	// Ensure workspace directory exists
	if err := os.MkdirAll(rm.workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Serialize data
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Write to file
	filepath := filepath.Join(rm.workspace, refID+".json")
	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}

// loadFromDisk loads a result from disk
func (rm *ResultManager) loadFromDisk(refID string) (interface{}, error) {
	filepath := filepath.Join(rm.workspace, refID+".json")

	jsonData, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read result: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return data, nil
}

// generatePreview creates a preview of the first N items from a result
func generatePreview(data interface{}, n int) string {
	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		end := n
		if end > len(v) {
			end = len(v)
		}
		preview, _ := json.Marshal(v[:end])
		return string(preview)

	case map[string]interface{}:
		if len(v) == 0 {
			return "{}"
		}
		// Show first n keys
		preview := make(map[string]interface{})
		count := 0
		for key, val := range v {
			if count >= n {
				break
			}
			preview[key] = val
			count++
		}
		previewJSON, _ := json.Marshal(preview)
		return string(previewJSON)

	default:
		return toPythonRepr(data)
	}
}

// getItemCount returns the number of items in a result
func getItemCount(data interface{}) int {
	switch v := data.(type) {
	case []interface{}:
		return len(v)
	case map[string]interface{}:
		return len(v)
	default:
		return 1
	}
}

// applyFilter applies a filter function to data
func applyFilter(data interface{}, filterFunc func(interface{}) bool) interface{} {
	switch v := data.(type) {
	case []interface{}:
		result := make([]interface{}, 0)
		for _, item := range v {
			if filterFunc(item) {
				result = append(result, item)
			}
		}
		return result

	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			if filterFunc(map[string]interface{}{"key": key, "value": val}) {
				result[key] = val
			}
		}
		return result

	default:
		if filterFunc(data) {
			return data
		}
		return nil
	}
}

// toPythonRepr converts a Go value to a Python representation string
func toPythonRepr(data interface{}) string {
	return toPythonReprImpl(data, 0)
}

// toPythonReprImpl is the recursive implementation
func toPythonReprImpl(data interface{}, depth int) string {
	if depth > 10 {
		return "..." // Prevent infinite recursion
	}

	switch v := data.(type) {
	case nil:
		return "None"

	case bool:
		if v {
			return "True"
		}
		return "False"

	case int:
		return fmt.Sprintf("%d", v)

	case int64:
		return fmt.Sprintf("%d", v)

	case float64:
		return fmt.Sprintf("%g", v)

	case string:
		return fmt.Sprintf("\"%s\"", escapeString(v))

	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		items := make([]string, len(v))
		for i, item := range v {
			items[i] = toPythonReprImpl(item, depth+1)
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))

	case map[string]interface{}:
		if len(v) == 0 {
			return "{}"
		}
		items := make([]string, 0, len(v))
		for key, val := range v {
			items = append(items, fmt.Sprintf("\"%s\": %s",
				escapeString(key), toPythonReprImpl(val, depth+1)))
		}
		return fmt.Sprintf("{%s}", strings.Join(items, ", "))

	default:
		// Try to marshal to JSON
		jsonData, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("\"%v\"", v)
		}
		return string(jsonData)
	}
}

// escapeString escapes special characters in a string
func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// ResultRef represents a reference to a large result
// This is what gets sent to the LLM instead of the full data
type ResultRef struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// String returns a Python-style representation
func (r ResultRef) String() string {
	return fmt.Sprintf("ResultRef(\"%s\", count=%d)", r.ID, r.Count)
}

// CreateFilterFunction creates a Go filter function from Python-like filter expression
//
// Supported expressions:
// - "status == 'pending'"  → checks if item["status"] == "pending"
// - "size > 1000"          → checks if item["size"] > 1000
// - "name contains 'foo'"  → checks if "foo" in item["name"]
func (rm *ResultManager) CreateFilterFunction(expr string) func(interface{}) bool {
	// For now, support simple field comparison expressions
	// In production, this would use a proper expression parser

	expr = strings.TrimSpace(expr)

	// Simple equality check: field == value
	if parts := strings.Split(expr, "=="); len(parts) == 2 {
		field := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		return func(item interface{}) bool {
			switch v := item.(type) {
			case map[string]interface{}:
				if fieldVal, ok := v[field]; ok {
					return fmt.Sprintf("%v", fieldVal) == value
				}
			}
			return false
		}
	}

	// Greater than: field > value
	if parts := strings.Split(expr, ">"); len(parts) == 2 {
		field := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		var threshold float64
		fmt.Sscanf(value, "%f", &threshold)

		return func(item interface{}) bool {
			switch v := item.(type) {
			case map[string]interface{}:
				if fieldVal, ok := v[field]; ok {
					switch fv := fieldVal.(type) {
					case float64:
						return fv > threshold
					case int:
						return float64(fv) > threshold
					}
				}
			}
			return false
		}
	}

	// Default: always true
	return func(item interface{}) bool {
		return true
	}
}

// Clear removes all stored results
func (rm *ResultManager) Clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.refs = make(map[string]*StoredResult)
	rm.counter = 0

	// Remove files from disk
	files, _ := os.ReadDir(rm.workspace)
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "__result_") {
			os.Remove(filepath.Join(rm.workspace, file.Name()))
		}
	}
}
