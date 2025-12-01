package tools

import (
	"strings"
	"testing"
	"time"
)

func TestExecuteCode(t *testing.T) {
	tests := []struct {
		name     string
		language string
		code     string
		wantErr  bool
		contains string
	}{
		{
			name:     "bash echo",
			language: "bash",
			code:     "echo 'hello world'",
			wantErr:  false,
			contains: "hello world",
		},
		{
			name:     "python print",
			language: "python",
			code:     "print('hello from python')",
			wantErr:  false,
			contains: "hello from python",
		},
		{
			name:     "bash ls",
			language: "bash",
			code:     "ls -l sample.txt 2>&1 | head -1",
			wantErr:  false,
			contains: "sample.txt",
		},
		{
			name:     "unsupported language",
			language: "ruby",
			code:     "puts 'hello'",
			wantErr:  true,
			contains: "",
		},
		{
			name:     "dangerous command blocked",
			language: "bash",
			code:     "sudo rm -rf /",
			wantErr:  true,
			contains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := ExecuteCode(tt.language, tt.code)

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.contains != "" && !strings.Contains(output, tt.contains) {
				t.Errorf("output %q does not contain %q", output, tt.contains)
			}
		})
	}
}

func TestExecuteCodeLanguageVariations(t *testing.T) {
	tests := []struct {
		name     string
		language string
		code     string
		wantErr  bool
	}{
		{
			name:     "bash variant",
			language: "bash",
			code:     "echo test",
			wantErr:  false,
		},
		{
			name:     "sh variant",
			language: "sh",
			code:     "echo test",
			wantErr:  false,
		},
		{
			name:     "python variant",
			language: "python",
			code:     "print('test')",
			wantErr:  false,
		},
		{
			name:     "python3 variant",
			language: "python3",
			code:     "print('test')",
			wantErr:  false,
		},
		{
			name:     "javascript variant",
			language: "javascript",
			code:     "console.log('test')",
			wantErr:  false,
		},
		{
			name:     "js variant",
			language: "js",
			code:     "console.log('test')",
			wantErr:  false,
		},
		{
			name:     "node variant",
			language: "node",
			code:     "console.log('test')",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExecuteCode(tt.language, tt.code)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestExecuteCodeWithTimeout(t *testing.T) {
	code := "sleep 10"
	_, err := ExecuteCodeWithTimeout("bash", code, 1*time.Second)

	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestExecuteCodeTimeoutEnforcement(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "quick command within timeout",
			code:    "echo hello",
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name:    "slow command exceeds timeout",
			code:    "sleep 5",
			timeout: 1 * time.Second,
			wantErr: true,
		},
		{
			name:    "max timeout enforced",
			code:    "echo hello",
			timeout: 10 * time.Minute,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			_, err := ExecuteCodeWithTimeout("bash", tt.code, tt.timeout)
			duration := time.Since(start)

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantErr && duration > tt.timeout+2*time.Second {
				t.Errorf("timeout not enforced properly: took %v, timeout was %v", duration, tt.timeout)
			}
		})
	}
}

func TestIsDangerousCode(t *testing.T) {
	tests := []struct {
		code      string
		dangerous bool
	}{
		{"echo hello", false},
		{"rm -rf /", true},
		{"sudo apt install", true},
		{"dd if=/dev/zero", true},
		{"cat file.txt", false},
		{"chmod 777 /tmp/test", true},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			dangerous, _ := IsDangerousCode(tt.code)
			if dangerous != tt.dangerous {
				t.Errorf("IsDangerousCode(%q) = %v, want %v", tt.code, dangerous, tt.dangerous)
			}
		})
	}
}

func TestIsDangerousCodeComprehensive(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		dangerous bool
		reason    string
	}{
		{
			name:      "safe echo",
			code:      "echo hello world",
			dangerous: false,
		},
		{
			name:      "rm with recursive flag",
			code:      "rm -rf /var/log",
			dangerous: true,
			reason:    "rm -rf pattern",
		},
		{
			name:      "sudo command",
			code:      "sudo apt-get update",
			dangerous: true,
			reason:    "sudo usage",
		},
		{
			name:      "dd disk write",
			code:      "dd if=/dev/zero of=/dev/sda",
			dangerous: true,
			reason:    "dd pattern",
		},
		{
			name:      "mkfs format",
			code:      "mkfs.ext4 /dev/sda1",
			dangerous: true,
			reason:    "mkfs usage",
		},
		{
			name:      "fork bomb",
			code:      ":(){ :|:& };:",
			dangerous: true,
			reason:    "fork bomb pattern",
		},
		{
			name:      "redirect to device",
			code:      "echo test >> /dev/sda",
			dangerous: true,
			reason:    "redirect to device",
		},
		{
			name:      "chmod 777 recursive",
			code:      "chmod -R 777 /",
			dangerous: true,
			reason:    "chmod 777",
		},
		{
			name:      "format drive windows",
			code:      "format c:",
			dangerous: true,
			reason:    "format command",
		},
		{
			name:      "safe cat",
			code:      "cat /etc/hosts",
			dangerous: false,
		},
		{
			name:      "safe ls",
			code:      "ls -la /home",
			dangerous: false,
		},
		{
			name:      "safe grep",
			code:      "grep -r pattern /var/log",
			dangerous: false,
		},
		{
			name:      "safe python",
			code:      "python -c 'print(2+2)'",
			dangerous: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dangerous, reason := IsDangerousCode(tt.code)
			if dangerous != tt.dangerous {
				t.Errorf("IsDangerousCode(%q) = %v (reason: %s), want %v", tt.code, dangerous, reason, tt.dangerous)
			}
		})
	}
}

func TestExecuteCodeErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		language string
		code     string
		wantErr  bool
	}{
		{
			name:     "syntax error bash",
			language: "bash",
			code:     "if [ true",
			wantErr:  true,
		},
		{
			name:     "syntax error python",
			language: "python",
			code:     "print('unclosed",
			wantErr:  true,
		},
		{
			name:     "command not found",
			language: "bash",
			code:     "nonexistentcommand123",
			wantErr:  true,
		},
		{
			name:     "exit 1",
			language: "bash",
			code:     "exit 1",
			wantErr:  true,
		},
		{
			name:     "python exception",
			language: "python",
			code:     "raise Exception('test error')",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExecuteCode(tt.language, tt.code)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTruncateForHistory(t *testing.T) {
	small := "small output"
	result := TruncateForHistory(small)
	if result != small {
		t.Errorf("small output was truncated")
	}

	large := strings.Repeat("a", MaxHistorySize+1000)
	result = TruncateForHistory(large)
	if len(result) > MaxHistorySize+200 {
		t.Errorf("output not truncated properly: got %d bytes", len(result))
	}
	if !strings.Contains(result, "bytes omitted") {
		t.Error("truncation marker not found")
	}
}

func TestTruncateForHistoryEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		shouldTruncate bool
	}{
		{
			name:           "empty string",
			input:          "",
			shouldTruncate: false,
		},
		{
			name:           "exactly at limit",
			input:          strings.Repeat("a", MaxHistorySize),
			shouldTruncate: false,
		},
		{
			name:           "one byte over limit",
			input:          strings.Repeat("a", MaxHistorySize+1),
			shouldTruncate: true,
		},
		{
			name:           "very large output",
			input:          strings.Repeat("x", MaxHistorySize*10),
			shouldTruncate: true,
		},
		{
			name:           "multiline small",
			input:          "line1\nline2\nline3",
			shouldTruncate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateForHistory(tt.input)

			if tt.shouldTruncate {
				if len(result) >= len(tt.input) {
					t.Errorf("expected truncation but got same or larger size")
				}
				if !strings.Contains(result, "bytes omitted") {
					t.Error("truncation marker not found")
				}
			} else {
				if result != tt.input {
					t.Errorf("unexpected truncation for small input")
				}
			}
		})
	}
}

func TestExecuteCodeOutputSizeLimit(t *testing.T) {
	code := "head -c 2000000 /dev/zero | base64"
	output, err := ExecuteCode("bash", code)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output) > MaxOutputSize+1000 {
		t.Errorf("output size exceeds limit: got %d bytes, max is %d", len(output), MaxOutputSize)
	}

	if !strings.Contains(output, "output truncated") {
		t.Error("large output should contain truncation marker")
	}
}

func TestExecuteCodeSpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		language string
		code     string
		wantErr  bool
	}{
		{
			name:     "unicode in python",
			language: "python",
			code:     "print('你好世界')",
			wantErr:  false,
		},
		{
			name:     "special chars in bash",
			language: "bash",
			code:     "echo '$@#%^&*'",
			wantErr:  false,
		},
		{
			name:     "quotes and escapes",
			language: "bash",
			code:     "echo \"test\\\"escaped\\\"\"",
			wantErr:  false,
		},
		{
			name:     "backticks in python",
			language: "python",
			code:     "print(f'template {2+2}')",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExecuteCode(tt.language, tt.code)
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
