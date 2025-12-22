package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"clai/internal/logger"
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
	CREATE INDEX IF NOT EXISTS idx_updated_at ON conversations(updated_at DESC);
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
