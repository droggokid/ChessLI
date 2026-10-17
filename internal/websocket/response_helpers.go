package websocket

import (
	"context"
	"errors"
	"log/slog"

	"ChessLI/internal/websocket/protocol"
)

func sendApplicationError(ctx context.Context, client *Session, requestID string, cause error) error {
	code, publicMessage := mapApplicationError(cause)
	if code == protocol.ErrorInternal && ctx.Err() == nil && !errors.Is(cause, protocol.ErrSessionNotRunning) {
		slog.Error("game request failed", "event", "request.failed", "profile_id", client.profileID,
			"request_id", requestID, "error_code", code, "error", cause)
	}
	return sendError(ctx, client, requestID, code, publicMessage)
}

func sendError(ctx context.Context, client *Session, requestID string, code protocol.ErrorCode, message string) error {
	return client.Send(ctx, protocol.ServerEnvelope{Type: protocol.ServerError, RequestID: requestID, Payload: protocol.ErrorPayload{Code: code, Message: message}})
}
