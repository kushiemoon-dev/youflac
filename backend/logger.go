package backend

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Logger is the package-level structured logger.
// All backend code should use this instead of fmt.Printf.
var Logger = slog.Default()

// LogEntry is a single buffered log line.
type LogEntry struct {
	ID      int64  `json:"id"`
	Time    string `json:"time"`    // HH:MM:SS
	Level   string `json:"level"`   // DEBUG, INFO, WARN, ERROR
	Message string `json:"message"`
}

// logBuffer is the in-process ring buffer of recent log entries.
var logBuffer = &ringBuffer{capacity: 500}

// ringBuffer holds up to capacity LogEntry items in a circular fashion.
type ringBuffer struct {
	mu       sync.RWMutex
	entries  []LogEntry
	capacity int
	seq      int64 // monotonic ID counter
}

func (r *ringBuffer) add(level, msg string) {
	r.mu.Lock()
	r.seq++
	entry := LogEntry{
		ID:      r.seq,
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: msg,
	}
	if len(r.entries) >= r.capacity {
		// Overwrite oldest
		copy(r.entries, r.entries[1:])
		r.entries[len(r.entries)-1] = entry
	} else {
		r.entries = append(r.entries, entry)
	}
	r.mu.Unlock()
}

// GetLogs returns all buffered entries with ID > sinceID.
func GetLogs(sinceID int64) []LogEntry {
	logBuffer.mu.RLock()
	defer logBuffer.mu.RUnlock()

	var result []LogEntry
	for _, e := range logBuffer.entries {
		if e.ID > sinceID {
			result = append(result, e)
		}
	}
	return result
}

// bufferingWriter wraps an io.Writer and also writes parsed lines to the ring buffer.
type bufferingWriter struct {
	underlying io.Writer
}

func (bw *bufferingWriter) Write(p []byte) (n int, err error) {
	n, err = bw.underlying.Write(p)
	if n > 0 {
		line := strings.TrimRight(string(p[:n]), "\n")
		// Parse level from slog text format: "level=INFO" or "level=DEBUG" etc.
		level := "INFO"
		if idx := strings.Index(line, "level="); idx != -1 {
			rest := line[idx+6:]
			end := strings.IndexAny(rest, " \t")
			if end == -1 {
				end = len(rest)
			}
			level = strings.ToUpper(rest[:end])
		}
		// Strip the slog preamble to get just the message
		msg := line
		if idx := strings.Index(line, "msg="); idx != -1 {
			rest := line[idx+4:]
			// msg may be quoted
			if len(rest) > 0 && rest[0] == '"' {
				end := strings.Index(rest[1:], "\"")
				if end != -1 {
					msg = rest[1 : end+1]
				}
			} else {
				end := strings.IndexAny(rest, " \t")
				if end == -1 {
					end = len(rest)
				}
				msg = rest[:end]
			}
		}
		logBuffer.add(level, msg)
	}
	return
}

// InitLogger initialises the slog default logger.
// logLevel should be one of: "debug", "info", "warn", "error".
// The LOG_LEVEL environment variable overrides the config value.
func InitLogger(logLevel string) {
	if env := os.Getenv("LOG_LEVEL"); env != "" {
		logLevel = env
	}

	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	writer := &bufferingWriter{underlying: os.Stdout}

	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	Logger = logger
}
