package log

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// LogEntry is the structured representation of a log line
type LogEntry struct {
	Ts         time.Time              `json:"ts"`
	Level      string                 `json:"level"`
	Logger     string                 `json:"logger,omitempty"`
	File       string                 `json:"file,omitempty"`
	Line       int                    `json:"line,omitempty"`
	Msg        string                 `json:"msg"`
	Body       string                 `json:"body,omitempty"`
	BodyFormat string                 `json:"body_format,omitempty"`
	Ctx        map[string]interface{} `json:"ctx,omitempty"`
}

var (
	// quick fallback regex: timestamp level msg
	fallbackRegex = regexp.MustCompile(`^(?P<ts>\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)?Z?)\\s+(?P<level>[A-Z]+)\\s+(?P<msg>.*)$`)
)

// ParseLogLine tries to parse a single log line into LogEntry. It prefers JSON, then falls back to a best-effort regex parse.
func ParseLogLine(line string) (LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return LogEntry{}, errors.New("empty line")
	}

	// Try JSON
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err == nil {
		le := LogEntry{}
		// timestamp
		if v, ok := raw["ts"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				le.Ts = t
			}
		}
		// level
		if v, ok := raw["level"].(string); ok {
			le.Level = strings.ToLower(v)
		}
		// logger
		if v, ok := raw["logger"].(string); ok {
			le.Logger = v
		}
		// file
		if v, ok := raw["file"].(string); ok {
			le.File = v
		}
		// line
		if v, ok := raw["line"].(float64); ok {
			le.Line = int(v)
		} else if v, ok := raw["line"].(string); ok {
			if n, err := strconv.Atoi(v); err == nil {
				le.Line = n
			}
		}
		// msg
		if v, ok := raw["msg"].(string); ok {
			le.Msg = v
		}
		// body
		if v, ok := raw["body"].(string); ok {
			le.Body = v
		}
		// body_format
		if v, ok := raw["body_format"].(string); ok {
			le.BodyFormat = v
		}
		// ctx
		if v, ok := raw["ctx"].(map[string]interface{}); ok {
			le.Ctx = v
		}

		return le, nil
	}

	// Fallback: regex
	if matches := fallbackRegex.FindStringSubmatch(line); matches != nil {
		idx := make(map[string]string)
		for i, name := range fallbackRegex.SubexpNames() {
			if i != 0 && name != "" && i < len(matches) {
				idx[name] = matches[i]
			}
		}
		le := LogEntry{Level: strings.ToLower(idx["level"]), Msg: idx["msg"]}
		if ts, ok := idx["ts"]; ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				le.Ts = t
			}
		}
		return le, nil
	}

	// As last resort: put entire line into Msg
	return LogEntry{Msg: line, Level: "info", Ts: time.Now()}, nil
}

// ReadLogLines reads logs from an io.Reader and emits parsed entries to a channel.
func ReadLogLines(r io.Reader, out chan<- LogEntry) error {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		if le, err := ParseLogLine(line); err == nil {
			out <- le
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	return nil
}

// ---- Emitter ----

// EmitStructured emits a structured log line to the provided writer.
// When w is nil, uses os.Stdout.
func EmitStructured(w io.Writer, entry LogEntry) error {
	if w == nil {
		w = os.Stdout
	}
	// Ensure timestamp
	if entry.Ts.IsZero() {
		entry.Ts = time.Now().UTC()
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// EmitWithZerolog helper to emit via zerolog when desired
func EmitWithZerolog(level string, msg string, body string, bodyFormat string, ctx map[string]interface{}) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	e := logger.WithLevel(zerolog.InfoLevel)
	switch strings.ToLower(level) {
	case "debug":
		e = logger.WithLevel(zerolog.DebugLevel)
	case "info":
		e = logger.WithLevel(zerolog.InfoLevel)
	case "warn", "warning":
		e = logger.WithLevel(zerolog.WarnLevel)
	case "error":
		e = logger.WithLevel(zerolog.ErrorLevel)
	case "fatal":
		e = logger.WithLevel(zerolog.FatalLevel)
	}

	event := e.Str("msg", msg)
	if body != "" {
		event = event.Str("body", body)
	}
	if bodyFormat != "" {
		event = event.Str("body_format", bodyFormat)
	}
	if ctx != nil {
		event = event.Dict("ctx", zerolog.Dict().Fields(ctx))
	}
	event.Send()
}
