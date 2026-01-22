package main

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

// LogFormat represents the available log output formats
type LogFormat string

const (
	LogFormatPretty LogFormat = "pretty" // Colorized, human-readable (tint)
	LogFormatJSON   LogFormat = "json"   // JSON lines
	LogFormatText   LogFormat = "text"   // key=value pairs
)

// InitLogger initializes the global slog logger with the specified format and level
func InitLogger(format LogFormat, level slog.Level) {
	var handler slog.Handler

	switch format {
	case LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	case LogFormatText:
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	case LogFormatPretty:
		fallthrough
	default:
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level:      level,
			TimeFormat: time.DateTime,
		})
	}

	slog.SetDefault(slog.New(handler))
}

// ParseLogFormat converts a string to LogFormat, defaulting to pretty
func ParseLogFormat(s string) LogFormat {
	switch strings.ToLower(s) {
	case "json":
		return LogFormatJSON
	case "text":
		return LogFormatText
	default:
		return LogFormatPretty
	}
}

// ParseLogLevel converts a string to slog.Level, defaulting to Info
func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
