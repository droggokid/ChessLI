package log

import (
	"log/slog"
	"os"
)

// LogFormat specifies the output format for gameplay logs.
type LogFormat string

const (
	// LogFormatJSON writes structured JSON logs for deployed environments.
	LogFormatJSON LogFormat = "json"
	// LogFormatText writes human-readable logs for local development.
	LogFormatText LogFormat = "text"
)

// NewLogger returns an info-level logger that writes to standard output.
// It returns nil when format is unsupported.
func NewLogger(format LogFormat) *slog.Logger {
	var logger *slog.Logger

	if format == LogFormatJSON {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	} else if format == LogFormatText {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
	return logger
}
