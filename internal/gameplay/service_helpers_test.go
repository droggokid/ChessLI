package gameplay

import (
	"testing"
	"time"
)

func TestValidateTimeControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		initial   time.Duration
		increment time.Duration
		wantErr   bool
	}{
		{name: "valid", initial: time.Minute, increment: time.Second},
		{name: "zero initial", initial: 0, wantErr: true},
		{name: "negative initial", initial: -time.Second, wantErr: true},
		{name: "negative increment", initial: time.Minute, increment: -time.Second, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateTimeControl(tt.initial, tt.increment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateTimeControl() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
