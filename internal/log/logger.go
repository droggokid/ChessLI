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
// Config.Load validates format; JSON is the default unless text is requested.
func NewLogger(format LogFormat) *slog.Logger {
	if format == LogFormatText {
		return slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
