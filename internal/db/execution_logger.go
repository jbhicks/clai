package db

// ExecutionLogger implements the tools.ExecutionLogger interface
type ExecutionLogger struct {
	store *Store
}

// NewExecutionLogger creates a new execution logger bound to a store
func NewExecutionLogger(store *Store) *ExecutionLogger {
	return &ExecutionLogger{store: store}
}

// LogExecution implements the ExecutionLogger interface
func (el *ExecutionLogger) LogExecution(conversationID int, language, code string, exitCode int, duration int64, outputSize int, execError string) error {
	return el.store.SaveExecutionLog(conversationID, language, code, exitCode, duration, outputSize, execError)
}
