package db

import (
	"clai/internal/logger"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"clai/internal/llm"
)

type Store struct {
	db *sql.DB
}

type Conversation struct {
	ID        int
	Title     string
	Messages  []llm.Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

func New() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbDir := filepath.Join(homeDir, ".clai")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "conversations.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.init(); err != nil {
		db.Close()
		return nil, err
	}

	logger.Info("[DB] Opened database at %s", dbPath)
	return store, nil
}

func (s *Store) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		messages TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS execution_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id INTEGER NOT NULL,
		language TEXT NOT NULL,
		code TEXT NOT NULL,
		exit_code INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL,
		output_size INTEGER NOT NULL,
		error_message TEXT,
		executed_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS benchmark_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model_name TEXT NOT NULL,
		model_url TEXT NOT NULL,
		total_tests INTEGER NOT NULL,
		passed_tests INTEGER NOT NULL,
		failed_tests INTEGER NOT NULL,
		success_rate REAL NOT NULL,
		total_time_seconds REAL NOT NULL,
		avg_iterations REAL NOT NULL,
		started_at DATETIME NOT NULL,
		completed_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS benchmark_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER NOT NULL,
		test_name TEXT NOT NULL,
		query TEXT NOT NULL,
		passed BOOLEAN NOT NULL,
		iterations INTEGER NOT NULL,
		time_seconds REAL NOT NULL,
		response TEXT NOT NULL,
		failure_reason TEXT,
		code_executed TEXT,
		FOREIGN KEY (run_id) REFERENCES benchmark_runs(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_updated_at ON conversations(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_benchmark_runs_started ON benchmark_runs(started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_benchmark_results_run ON benchmark_results(run_id);
	CREATE TABLE IF NOT EXISTS quick_tests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model_name TEXT NOT NULL,
		model_path TEXT NOT NULL,
		prompt TEXT NOT NULL,
		response TEXT NOT NULL,
		duration_ms INTEGER NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_quick_tests_created ON quick_tests(created_at DESC);
	CREATE TABLE IF NOT EXISTS agentic_benchmark_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model_name TEXT NOT NULL,
		model_path TEXT NOT NULL,
		total_tests INTEGER NOT NULL,
		passed_tests INTEGER NOT NULL,
		failed_tests INTEGER NOT NULL,
		success_rate REAL NOT NULL,
		total_duration_ms INTEGER NOT NULL,
		started_at DATETIME NOT NULL,
		completed_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS agentic_benchmark_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER NOT NULL,
		test_name TEXT NOT NULL,
		task_description TEXT NOT NULL,
		prompt TEXT NOT NULL,
		generated_code TEXT,
		execution_output TEXT,
		expected_result TEXT NOT NULL,
		passed BOOLEAN NOT NULL,
		validation_reason TEXT,
		duration_ms INTEGER NOT NULL,
		error_message TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (run_id) REFERENCES agentic_benchmark_runs(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_agentic_runs_started ON agentic_benchmark_runs(started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_agentic_results_run ON agentic_benchmark_results(run_id);
	CREATE TABLE IF NOT EXISTS downloads (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		filename TEXT NOT NULL,
		status TEXT NOT NULL,
		progress REAL NOT NULL,
		bytes_downloaded INTEGER NOT NULL,
		total_bytes INTEGER NOT NULL,
		speed INTEGER NOT NULL,
		error TEXT,
		started_at DATETIME NOT NULL,
		completed_at DATETIME,
		retry_count INTEGER NOT NULL DEFAULT 0,
		supports_resume BOOLEAN NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
	CREATE INDEX IF NOT EXISTS idx_downloads_started ON downloads(started_at DESC);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveConversation(conv *Conversation) error {
	messagesJSON, err := json.Marshal(conv.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	now := time.Now()
	if conv.ID == 0 {
		conv.CreatedAt = now
		conv.UpdatedAt = now
		result, err := s.db.Exec(
			"INSERT INTO conversations (title, messages, created_at, updated_at) VALUES (?, ?, ?, ?)",
			conv.Title, string(messagesJSON), conv.CreatedAt, conv.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert conversation: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert id: %w", err)
		}
		conv.ID = int(id)
		logger.Info("[DB] Created conversation %d: %s", conv.ID, conv.Title)
	} else {
		conv.UpdatedAt = now
		_, err := s.db.Exec(
			"UPDATE conversations SET title = ?, messages = ?, updated_at = ? WHERE id = ?",
			conv.Title, string(messagesJSON), conv.UpdatedAt, conv.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to update conversation: %w", err)
		}
		logger.Info("[DB] Updated conversation %d", conv.ID)
	}

	return nil
}

func (s *Store) GetLatestConversation() (*Conversation, error) {
	var conv Conversation
	var messagesJSON string

	err := s.db.QueryRow(
		"SELECT id, title, messages, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT 1",
	).Scan(&conv.ID, &conv.Title, &messagesJSON, &conv.CreatedAt, &conv.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest conversation: %w", err)
	}

	if err := json.Unmarshal([]byte(messagesJSON), &conv.Messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	logger.Info("[DB] Loaded conversation %d with %d messages", conv.ID, len(conv.Messages))
	return &conv, nil
}

func (s *Store) ListConversations(limit int) ([]Conversation, error) {
	rows, err := s.db.Query(
		"SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

func (s *Store) GetConversation(id int) (*Conversation, error) {
	var conv Conversation
	var messagesJSON string

	err := s.db.QueryRow(
		"SELECT id, title, messages, created_at, updated_at FROM conversations WHERE id = ?",
		id,
	).Scan(&conv.ID, &conv.Title, &messagesJSON, &conv.CreatedAt, &conv.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation: %w", err)
	}

	if err := json.Unmarshal([]byte(messagesJSON), &conv.Messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	return &conv, nil
}

func (s *Store) DeleteConversation(id int) error {
	_, err := s.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	logger.Info("[DB] Deleted conversation %d", id)
	return nil
}

func GenerateConversationTitle(messages []llm.Message) string {
	for _, msg := range messages {
		if msg.Role == "user" && msg.Content != "" {
			title := msg.Content
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			return title
		}
	}
	return "New Conversation"
}

func (s *Store) SaveExecutionLog(conversationID int, language, code string, exitCode int, duration int64, outputSize int, execError string) error {
	executedAt := time.Now()

	_, err := s.db.Exec(
		"INSERT INTO execution_logs (conversation_id, language, code, exit_code, duration_ms, output_size, error_message, executed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		conversationID, language, code, exitCode, duration, outputSize, execError, executedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert execution log: %w", err)
	}

	logger.Info("[DB] Saved execution log for conversation %d", conversationID)
	return nil
}
