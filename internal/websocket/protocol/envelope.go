package protocol

import "encoding/json"

type serverEnvelope[T any] struct {
	Type      ServerMessageType `json:"type"`
	RequestID string            `json:"requestId"`
	Payload   T                 `json:"payload"`
}

// ServerEnvelope is an event or response sent by the server.
type ServerEnvelope = serverEnvelope[any]

// RawServerEnvelope preserves the JSON payload when receiving a server message.
type RawServerEnvelope = serverEnvelope[json.RawMessage]

// ClientEnvelope is a command sent by the client.
type ClientEnvelope struct {
	Type      ClientMessageType `json:"type"`
	RequestID string            `json:"requestId"`
	Payload   json.RawMessage   `json:"payload"`
}
