package db

import (
	"database/sql"
	"fmt"
	"time"
)

type BenchmarkRun struct {
	ID               int
	ModelName        string
	ModelURL         string
	TotalTests       int
	PassedTests      int
	FailedTests      int
	SuccessRate      float64
	TotalTimeSeconds float64
	AvgIterations    float64
	StartedAt        time.Time
	CompletedAt      time.Time
}

type BenchmarkResult struct {
	ID            int
	RunID         int
	TestName      string
	Query         string
	Passed        bool
	Iterations    int
	TimeSeconds   float64
	Response      string
	FailureReason string
	CodeExecuted  string
}

type QuickTest struct {
	ID         int
	ModelName  string
	ModelPath  string
	Prompt     string
	Response   string
	DurationMs int64
	CreatedAt  time.Time
}

type AgenticBenchmarkRun struct {
	ID              int
	ModelName       string
	ModelPath       string
	TotalTests      int
	PassedTests     int
	FailedTests     int
	SuccessRate     float64
	TotalDurationMs int64
	StartedAt       time.Time
	CompletedAt     sql.NullTime
}

type AgenticBenchmarkResult struct {
	ID               int
	RunID            int
	TestName         string
	TaskDescription  string
	Prompt           string
	GeneratedCode    sql.NullString
	ExecutionOutput  sql.NullString
	ExpectedResult   string
	Passed           bool
	ValidationReason sql.NullString
	DurationMs       int64
	ErrorMessage     sql.NullString
	CreatedAt        time.Time
}

func (s *Store) SaveBenchmarkRun(run *BenchmarkRun) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO benchmark_runs (
			model_name, model_url, total_tests, passed_tests, failed_tests,
			success_rate, total_time_seconds, avg_iterations, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.ModelName, run.ModelURL, run.TotalTests, run.PassedTests, run.FailedTests,
		run.SuccessRate, run.TotalTimeSeconds, run.AvgIterations, run.StartedAt, run.CompletedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to save benchmark run: %w", err)
	}

	return result.LastInsertId()
}

func (s *Store) SaveBenchmarkResult(result *BenchmarkResult) error {
	_, err := s.db.Exec(`
		INSERT INTO benchmark_results (
			run_id, test_name, query, passed, iterations, time_seconds,
			response, failure_reason, code_executed
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, result.RunID, result.TestName, result.Query, result.Passed, result.Iterations,
		result.TimeSeconds, result.Response, result.FailureReason, result.CodeExecuted)

	if err != nil {
		return fmt.Errorf("failed to save benchmark result: %w", err)
	}

	return nil
}

func (s *Store) GetBenchmarkRuns(limit int) ([]BenchmarkRun, error) {
	rows, err := s.db.Query(`
		SELECT id, model_name, model_url, total_tests, passed_tests, failed_tests,
			success_rate, total_time_seconds, avg_iterations, started_at, completed_at
		FROM benchmark_runs
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query benchmark runs: %w", err)
	}
	defer rows.Close()

	var runs []BenchmarkRun
	for rows.Next() {
		var run BenchmarkRun
		err := rows.Scan(&run.ID, &run.ModelName, &run.ModelURL, &run.TotalTests,
			&run.PassedTests, &run.FailedTests, &run.SuccessRate, &run.TotalTimeSeconds,
			&run.AvgIterations, &run.StartedAt, &run.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan benchmark run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

func (s *Store) GetBenchmarkRun(id int) (*BenchmarkRun, error) {
	var run BenchmarkRun
	err := s.db.QueryRow(`
		SELECT id, model_name, model_url, total_tests, passed_tests, failed_tests,
			success_rate, total_time_seconds, avg_iterations, started_at, completed_at
		FROM benchmark_runs
		WHERE id = ?
	`, id).Scan(&run.ID, &run.ModelName, &run.ModelURL, &run.TotalTests,
		&run.PassedTests, &run.FailedTests, &run.SuccessRate, &run.TotalTimeSeconds,
		&run.AvgIterations, &run.StartedAt, &run.CompletedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmark run: %w", err)
	}

	return &run, nil
}

func (s *Store) GetBenchmarkResults(runID int) ([]BenchmarkResult, error) {
	rows, err := s.db.Query(`
		SELECT id, run_id, test_name, query, passed, iterations, time_seconds,
			response, failure_reason, code_executed
		FROM benchmark_results
		WHERE run_id = ?
		ORDER BY id
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query benchmark results: %w", err)
	}
	defer rows.Close()

	var results []BenchmarkResult
	for rows.Next() {
		var result BenchmarkResult
		var failureReason, codeExecuted sql.NullString
		err := rows.Scan(&result.ID, &result.RunID, &result.TestName, &result.Query,
			&result.Passed, &result.Iterations, &result.TimeSeconds, &result.Response,
			&failureReason, &codeExecuted)
		if err != nil {
			return nil, fmt.Errorf("failed to scan benchmark result: %w", err)
		}
		if failureReason.Valid {
			result.FailureReason = failureReason.String
		}
		if codeExecuted.Valid {
			result.CodeExecuted = codeExecuted.String
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) SaveQuickTest(test *QuickTest) error {
	_, err := s.db.Exec(`
		INSERT INTO quick_tests (model_name, model_path, prompt, response, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, test.ModelName, test.ModelPath, test.Prompt, test.Response, test.DurationMs, test.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save quick test: %w", err)
	}

	return nil
}

func (s *Store) GetRecentQuickTests(limit int) ([]QuickTest, error) {
	rows, err := s.db.Query(`
		SELECT id, model_name, model_path, prompt, response, duration_ms, created_at
		FROM quick_tests
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query quick tests: %w", err)
	}
	defer rows.Close()

	var tests []QuickTest
	for rows.Next() {
		var test QuickTest
		err := rows.Scan(&test.ID, &test.ModelName, &test.ModelPath, &test.Prompt,
			&test.Response, &test.DurationMs, &test.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan quick test: %w", err)
		}
		tests = append(tests, test)
	}

	return tests, nil
}

// SaveAgenticBenchmarkRun saves a new agentic benchmark run and returns the run ID
func (s *Store) SaveAgenticBenchmarkRun(run *AgenticBenchmarkRun) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO agentic_benchmark_runs (
			model_name, model_path, total_tests, passed_tests, failed_tests,
			success_rate, total_duration_ms, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.ModelName, run.ModelPath, run.TotalTests, run.PassedTests, run.FailedTests,
		run.SuccessRate, run.TotalDurationMs, run.StartedAt, run.CompletedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to save agentic benchmark run: %w", err)
	}

	return result.LastInsertId()
}

// UpdateAgenticBenchmarkRun updates an existing benchmark run
func (s *Store) UpdateAgenticBenchmarkRun(run *AgenticBenchmarkRun) error {
	var completedAt interface{}
	if run.CompletedAt.Valid {
		completedAt = run.CompletedAt.Time
	} else {
		completedAt = nil
	}

	_, err := s.db.Exec(`
		UPDATE agentic_benchmark_runs
		SET passed_tests = ?, failed_tests = ?, success_rate = ?,
			total_duration_ms = ?, completed_at = ?
		WHERE id = ?
	`, run.PassedTests, run.FailedTests, run.SuccessRate,
		run.TotalDurationMs, completedAt, run.ID)

	if err != nil {
		return fmt.Errorf("failed to update agentic benchmark run: %w", err)
	}

	return nil
}

// SaveAgenticBenchmarkResult saves a single test result
func (s *Store) SaveAgenticBenchmarkResult(result *AgenticBenchmarkResult) error {
	_, err := s.db.Exec(`
		INSERT INTO agentic_benchmark_results (
			run_id, test_name, task_description, prompt, generated_code,
			execution_output, expected_result, passed, validation_reason,
			duration_ms, error_message, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, result.RunID, result.TestName, result.TaskDescription, result.Prompt,
		result.GeneratedCode, result.ExecutionOutput, result.ExpectedResult,
		result.Passed, result.ValidationReason, result.DurationMs,
		result.ErrorMessage, result.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save agentic benchmark result: %w", err)
	}

	return nil
}

// GetRecentAgenticBenchmarkRuns returns the most recent agentic benchmark runs
func (s *Store) GetRecentAgenticBenchmarkRuns(limit int) ([]AgenticBenchmarkRun, error) {
	rows, err := s.db.Query(`
		SELECT id, model_name, model_path, total_tests, passed_tests, failed_tests,
			success_rate, total_duration_ms, started_at, completed_at
		FROM agentic_benchmark_runs
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query agentic benchmark runs: %w", err)
	}
	defer rows.Close()

	var runs []AgenticBenchmarkRun
	for rows.Next() {
		var run AgenticBenchmarkRun
		err := rows.Scan(&run.ID, &run.ModelName, &run.ModelPath, &run.TotalTests,
			&run.PassedTests, &run.FailedTests, &run.SuccessRate, &run.TotalDurationMs,
			&run.StartedAt, &run.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agentic benchmark run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// GetAgenticBenchmarkResults returns all results for a specific run
func (s *Store) GetAgenticBenchmarkResults(runID int) ([]AgenticBenchmarkResult, error) {
	rows, err := s.db.Query(`
		SELECT id, run_id, test_name, task_description, prompt, generated_code,
			execution_output, expected_result, passed, validation_reason,
			duration_ms, error_message, created_at
		FROM agentic_benchmark_results
		WHERE run_id = ?
		ORDER BY id
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query agentic benchmark results: %w", err)
	}
	defer rows.Close()

	var results []AgenticBenchmarkResult
	for rows.Next() {
		var result AgenticBenchmarkResult
		err := rows.Scan(&result.ID, &result.RunID, &result.TestName, &result.TaskDescription,
			&result.Prompt, &result.GeneratedCode, &result.ExecutionOutput, &result.ExpectedResult,
			&result.Passed, &result.ValidationReason, &result.DurationMs, &result.ErrorMessage,
			&result.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agentic benchmark result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// DeleteAllAgenticBenchmarkRuns deletes all agentic benchmark runs and their associated results
func (s *Store) DeleteAllAgenticBenchmarkRuns() error {
	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete all results first (foreign key constraint)
	_, err = tx.Exec("DELETE FROM agentic_benchmark_results")
	if err != nil {
		return fmt.Errorf("failed to delete agentic benchmark results: %w", err)
	}

	// Delete all runs
	_, err = tx.Exec("DELETE FROM agentic_benchmark_runs")
	if err != nil {
		return fmt.Errorf("failed to delete agentic benchmark runs: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
