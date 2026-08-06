package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrMissingPayload = errors.New("message payload is missing")

// DecodePayload decodes a client message payload into T and rejects unknown fields.
func DecodePayload[T any](message ClientEnvelope) (T, error) {
	var payload T

	raw := bytes.TrimSpace(message.Payload)

	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return payload, ErrMissingPayload
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode message payload: %w", err)
	}

	return payload, nil
}
