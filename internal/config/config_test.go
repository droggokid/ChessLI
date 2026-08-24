package config

import (
	"ChessLI/internal/log"
	"testing"
)

// TestLoad verifies configuration defaults, overrides, and validation.
func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      log.LogFormat
		wantError bool
	}{
		{name: "defaults to JSON", want: log.LogFormatJSON},
		{name: "text", value: "text", want: log.LogFormatText},
		{name: "normalizes whitespace and case", value: " TEXT ", want: log.LogFormatText},
		{name: "rejects unsupported format", value: "pretty", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(logFormatEnvironment, test.value)

			config, err := Load()
			if test.wantError {
				if err == nil {
					t.Fatal("Load() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if config.LogFormat != test.want {
				t.Fatalf("Load().LogFormat = %q, want %q", config.LogFormat, test.want)
			}
		})
	}
}
