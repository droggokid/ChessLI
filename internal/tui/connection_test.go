package tui

import (
	"strings"
	"testing"

	"ChessLI/internal/tui/screens/play"
	"ChessLI/internal/websocket/protocol"
)

func TestConnectionReportsPayloadEncodingFailure(t *testing.T) {
	network := newConnection("")
	t.Cleanup(func() { _ = network.Close() })
	result := network.send(nil, play.Request{
		Type: protocol.ClientMakeMove, RequestID: "move-1", Payload: make(chan int),
	})().(play.SendResultMsg)
	if result.RequestID != "move-1" || result.Err == nil ||
		!strings.Contains(result.Err.Error(), "encode payload") {
		t.Fatalf("send result = %+v", result)
	}
}
