package log

import (
	"testing"
	"time"
)

func TestParseJSONLogLine(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	jsonLine := `{"ts":"` + now + `","level":"info","msg":"hello","body":"details","body_format":"text","ctx":{"request_id":"abc"}}`
	le, err := ParseLogLine(jsonLine)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if le.Msg != "hello" {
		t.Fatalf("expected msg hello, got %s", le.Msg)
	}
	if le.Body != "details" {
		t.Fatalf("expected body details, got %s", le.Body)
	}
	if le.Ctx == nil || le.Ctx["request_id"] != "abc" {
		t.Fatalf("expected ctx.request_id abc, got %v", le.Ctx)
	}
}

func TestParsePlainFallback(t *testing.T) {
	line := "2025-01-01T12:00:00Z INFO everything ok"
	le, err := ParseLogLine(line)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if le.Msg == "" {
		t.Fatalf("expected non-empty msg for fallback, got empty")
	}
}
