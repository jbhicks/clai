package tools

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"
)

type ExecutionLogger interface {
	LogExecution(conversationID int, language, code string, exitCode int, duration int64, outputSize int, execError string) error
}

var globalLogger ExecutionLogger
var globalConversationID int

func SetExecutionLogger(logger ExecutionLogger, conversationID int) {
	globalLogger = logger
	globalConversationID = conversationID
}

const (
	DefaultTimeout = 30 * time.Second
	MaxTimeout     = 5 * time.Minute
	MaxOutputSize  = 1 * 1024 * 1024
	MaxHistorySize = 50 * 1024
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+(-[a-z]*r[a-z]*\s+)?/`),
	regexp.MustCompile(`(?i)\bsudo\b`),
	regexp.MustCompile(`(?i)\bdd\s+if=`),
	regexp.MustCompile(`(?i)\bmkfs\b`),
	regexp.MustCompile(`:\(\)\{\s*:\|:\&\s*\};:`),
	regexp.MustCompile(`(?i)>>\s*/dev/s[dr]`),
	regexp.MustCompile(`(?i)\bchmod\s+(-[a-z]*R[a-z]*\s+)?777`),
	regexp.MustCompile(`(?i)\bformat\s+[a-z]:`),
}

func ExecuteCode(language, code string) (string, error) {
	return ExecuteCodeWithTimeout(language, code, DefaultTimeout)
}

func IsDangerousCode(code string) (bool, string) {
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(code) {
			return true, fmt.Sprintf("Code contains dangerous pattern: %s", pattern.String())
		}
	}
	return false, ""
}

func ExecuteCodeWithTimeout(language, code string, timeout time.Duration) (string, error) {
	start := time.Now()

	if dangerous, reason := IsDangerousCode(code); dangerous {
		logExecution(language, code, -1, time.Since(start).Milliseconds(), 0, reason)
		return "", fmt.Errorf("code execution blocked: %s", reason)
	}

	if timeout > MaxTimeout {
		timeout = MaxTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch language {
	case "bash", "sh":
		cmd = exec.CommandContext(ctx, "bash", "-c", code)
	case "python", "python3":
		cmd = exec.CommandContext(ctx, "python3", "-c", code)
	case "javascript", "js", "node":
		cmd = exec.CommandContext(ctx, "node", "-e", code)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	output, err := cmd.CombinedOutput()
	duration := time.Since(start).Milliseconds()

	if len(output) > MaxOutputSize {
		output = output[:MaxOutputSize]
		output = append(output, []byte("\n... [output truncated]")...)
	}

	result := string(output)
	exitCode := 0
	execError := ""

	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		execError = err.Error()

		if ctx.Err() == context.DeadlineExceeded {
			execError = fmt.Sprintf("execution timeout after %v", timeout)
			result = result + "\n[Execution timeout]"
		}
	}

	logExecution(language, code, exitCode, duration, len(output), execError)

	return result, err
}

func TruncateForHistory(output string) string {
	outputBytes := []byte(output)
	if len(outputBytes) <= MaxHistorySize {
		return output
	}

	headSize := MaxHistorySize / 2
	tailSize := MaxHistorySize - headSize - 100

	summary := make([]byte, 0, MaxHistorySize)
	summary = append(summary, outputBytes[:headSize]...)
	summary = append(summary, []byte(fmt.Sprintf("\n\n... [%d bytes omitted] ...\n\n", len(outputBytes)-headSize-tailSize))...)
	summary = append(summary, outputBytes[len(outputBytes)-tailSize:]...)

	return string(summary)
}

func logExecution(language, code string, exitCode int, duration int64, outputSize int, execError string) {
	if globalLogger != nil {
		err := globalLogger.LogExecution(globalConversationID, language, code, exitCode, duration, outputSize, execError)
		if err != nil {
			// Silently fail - don't log to stderr
		}
	}
}
