package testing

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(s string) string {
	stripped := ansiRegex.ReplaceAllString(s, "")
	return strings.TrimSpace(stripped)
}

func NormalizeWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")

	var normalized []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			normalized = append(normalized, line)
		}
	}

	return strings.Join(normalized, "\n")
}

func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected to contain %q\nGot:\n%s", needle, haystack)
	}
}

func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected NOT to contain %q\nGot:\n%s", needle, haystack)
	}
}

func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for condition")
		}

		<-ticker.C
	}
}
