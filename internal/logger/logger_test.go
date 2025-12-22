package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	tests := []struct {
		level         string
		expectDebug   bool
		expectInfo    bool
		expectWarn    bool
		expectError   bool
	}{
		{"DEBUG", true, true, true, true},
		{"INFO", false, true, true, true},
		{"WARN", false, false, true, true},
		{"ERROR", false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			buf := &bytes.Buffer{}
			os.Setenv("LOG_LEVEL", tt.level)
			Init(buf)

			Debug("debug message")
			Info("info message")
			Warn("warn message")
			Error("error message")

			output := buf.String()

			if tt.expectDebug != strings.Contains(output, "[DEBUG]") {
				t.Errorf("Level %s: expected DEBUG=%v, got %v", tt.level, tt.expectDebug, strings.Contains(output, "[DEBUG]"))
			}
			if tt.expectInfo != strings.Contains(output, "[INFO]") {
				t.Errorf("Level %s: expected INFO=%v, got %v", tt.level, tt.expectInfo, strings.Contains(output, "[INFO]"))
			}
			if tt.expectWarn != strings.Contains(output, "[WARN]") {
				t.Errorf("Level %s: expected WARN=%v, got %v", tt.level, tt.expectWarn, strings.Contains(output, "[WARN]"))
			}
			if tt.expectError != strings.Contains(output, "[ERROR]") {
				t.Errorf("Level %s: expected ERROR=%v, got %v", tt.level, tt.expectError, strings.Contains(output, "[ERROR]"))
			}
		})
	}
}
