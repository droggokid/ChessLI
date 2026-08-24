package protocol

import (
	"encoding/json"
	"errors"
	"testing"

	"ChessLI/internal/identity"
)

func TestDecodePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		payload    json.RawMessage
		want       JoinGamePayload
		wantErr    bool
		missingErr bool
	}{
		{
			name:    "valid payload",
			payload: json.RawMessage(`{"gameId":"game"}`),
			want:    JoinGamePayload{GameID: identity.GameID("game")},
		},
		{name: "missing payload", wantErr: true, missingErr: true},
		{name: "null payload", payload: json.RawMessage(`null`), wantErr: true, missingErr: true},
		{name: "malformed payload", payload: json.RawMessage(`{"gameId":`), wantErr: true},
		{name: "unknown field", payload: json.RawMessage(`{"gameId":"game","extra":true}`), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodePayload[JoinGamePayload](ClientEnvelope{Payload: tt.payload})
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodePayload() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.missingErr && !errors.Is(err, ErrMissingPayload) {
				t.Fatalf("DecodePayload() error = %v, want %v", err, ErrMissingPayload)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("DecodePayload() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
