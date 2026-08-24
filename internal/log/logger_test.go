package log

import "testing"

func TestNewLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  LogFormat
		wantNil bool
	}{
		{name: "JSON", format: LogFormatJSON},
		{name: "text", format: LogFormatText},
		{name: "unsupported", format: LogFormat("xml"), wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := NewLogger(tt.format)
			if (logger == nil) != tt.wantNil {
				t.Fatalf("NewLogger(%q) nil = %v, want %v", tt.format, logger == nil, tt.wantNil)
			}
		})
	}
}
