package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"ChessLI/internal/websocket/protocol"
)

func TestFullQueueLogsMetadataWithoutPayload(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	client := newQueuedSession("player")
	for len(client.outgoing) < cap(client.outgoing) {
		client.outgoing <- protocol.ServerEnvelope{}
	}
	err := client.Send(context.Background(), protocol.ServerEnvelope{
		Type: protocol.ServerGameState, RequestID: "request", Payload: "private-payload",
	})
	if !errors.Is(err, protocol.ErrSessionQueueFull) {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("decode queue warning: %v", err)
	}
	if event["event"] != "message.dropped" || event["level"] != "WARN" || event["profile_id"] != "player" ||
		event["request_id"] != "request" || event["message_type"] != string(protocol.ServerGameState) || event["reason"] != "queue_full" {
		t.Fatalf("queue warning = %+v", event)
	}
	if strings.Contains(output.String(), "private-payload") {
		t.Fatal("queue warning included the message payload")
	}
}
