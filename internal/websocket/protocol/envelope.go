package protocol

import "encoding/json"

// ServerEnvelope is an event or response sent by the server.
type ServerEnvelope struct {
	Type      ServerMessageType `json:"type"`
	RequestID string            `json:"requestId,omitempty"`
	Payload   any               `json:"payload,omitempty"`
}

// ClientEnvelope is a command sent by the client.
type ClientEnvelope struct {
	Type      ClientMessageType `json:"type"`
	RequestID string            `json:"requestId,omitempty"`
	Payload   json.RawMessage   `json:"payload,omitempty"`
}
