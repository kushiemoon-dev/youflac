package backend

import (
	"log/slog"
	"os"
	"strings"
)

// Logger is the package-level structured logger.
// All backend code should use this instead of fmt.Printf.
var Logger = slog.Default()

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

	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	Logger = logger
}
