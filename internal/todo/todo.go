package todo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the current state of a todo item
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusCancelled  Status = "cancelled"
)

// Priority represents the importance level of a todo item
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Todo represents a single todo item
type Todo struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Status    Status    `json:"status"`
	Priority  Priority  `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager handles todo operations and persistence
type Manager struct {
	todos    map[string]*Todo
	filePath string
}

// NewManager creates a new todo manager using the .clai directory
func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	claiDir := filepath.Join(homeDir, ".clai")
	return &Manager{
		todos:    make(map[string]*Todo),
		filePath: filepath.Join(claiDir, "todos.json"),
	}
}

// Load reads todos from disk
func (m *Manager) Load() error {
	if _, err := os.Stat(m.filePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return fmt.Errorf("failed to read todos file: %w", err)
	}

	var todos []*Todo
	if err := json.Unmarshal(data, &todos); err != nil {
		return fmt.Errorf("failed to parse todos JSON: %w", err)
	}

	m.todos = make(map[string]*Todo)
	for _, todo := range todos {
		m.todos[todo.ID] = todo
	}

	return nil
}

// Save writes todos to disk
func (m *Manager) Save() error {
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create todos directory: %w", err)
	}

	todos := make([]*Todo, 0, len(m.todos))
	for _, todo := range m.todos {
		todos = append(todos, todo)
	}

	data, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal todos: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write todos file: %w", err)
	}

	return nil
}

// Add creates a new todo item
func (m *Manager) Add(content string, priority Priority) (*Todo, error) {
	id := fmt.Sprintf("%d", time.Now().UnixNano())

	todo := &Todo{
		ID:        id,
		Content:   content,
		Status:    StatusPending,
		Priority:  priority,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.todos[id] = todo
	return todo, nil
}

// Get retrieves a todo by ID
func (m *Manager) Get(id string) (*Todo, error) {
	todo, exists := m.todos[id]
	if !exists {
		return nil, fmt.Errorf("todo with ID %s not found", id)
	}
	return todo, nil
}

// Update modifies an existing todo
func (m *Manager) Update(id string, updates map[string]interface{}) error {
	todo, exists := m.todos[id]
	if !exists {
		return fmt.Errorf("todo with ID %s not found", id)
	}

	for key, value := range updates {
		switch key {
		case "content":
			if content, ok := value.(string); ok {
				todo.Content = content
			}
		case "status":
			if status, ok := value.(Status); ok {
				todo.Status = status
			}
		case "priority":
			if priority, ok := value.(Priority); ok {
				todo.Priority = priority
			}
		}
	}

	todo.UpdatedAt = time.Now()
	return nil
}

// Delete removes a todo
func (m *Manager) Delete(id string) error {
	if _, exists := m.todos[id]; !exists {
		return fmt.Errorf("todo with ID %s not found", id)
	}
	delete(m.todos, id)
	return nil
}

// List returns all todos, optionally filtered by status
func (m *Manager) List(statusFilter Status) []*Todo {
	var result []*Todo

	for _, todo := range m.todos {
		if statusFilter == "" || todo.Status == statusFilter {
			result = append(result, todo)
		}
	}

	return result
}

// Count returns the number of todos by status
func (m *Manager) Count() map[Status]int {
	counts := make(map[Status]int)
	for _, todo := range m.todos {
		counts[todo.Status]++
	}
	return counts
}
