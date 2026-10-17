package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const outgoingBufferSize = 16

// HandlerFunc processes a JSON message received from a client.
type HandlerFunc func(
	ctx context.Context,
	client *Session,
	message json.RawMessage,
) error

// Session exchanges JSON messages over a WebSocket connection.
// Send is safe for concurrent use; Run may be called only once.
type Session struct {
	profileID identity.ProfileID
	conn      *coderws.Conn
	running   atomic.Bool

	outgoing chan protocol.ServerEnvelope

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

// NewSession creates a Session for profileID and conn.
func NewSession(profileID identity.ProfileID, conn *coderws.Conn) *Session {
	return &Session{
		profileID:         profileID,
		conn:              conn,
		outgoing:          make(chan protocol.ServerEnvelope, outgoingBufferSize),
		heartbeatInterval: heartbeatInterval,
		heartbeatTimeout:  heartbeatTimeout,
	}
}

// Run exchanges messages until the context is canceled or an I/O loop stops.
// It sends connection.ready first and joins all I/O loops before returning; call only once.
// The server supplies a valid connection and message handler for each new session.
func (c *Session) Run(ctx context.Context, handleMessage HandlerFunc) error {
	c.running.Store(true)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer c.running.Store(false)
	defer c.conn.CloseNow()

	// Write before starting the loops so no response can overtake connection.ready.
	readyCtx, readyCancel := context.WithTimeout(ctx, heartbeatTimeout)
	err := wsjson.Write(readyCtx, c.conn, protocol.ServerEnvelope{
		Type:    protocol.ServerConnectionReady,
		Payload: protocol.ConnectionReadyPayload{ProfileID: c.profileID},
	})
	readyCancel()
	if err != nil {
		return fmt.Errorf("send connection ack: %w", err)
	}

	errCh := make(chan error, 3)

	go func() {
		errCh <- c.readLoop(ctx, handleMessage)
	}()

	go func() {
		errCh <- c.writeLoop(ctx)
	}()

	go func() {
		errCh <- c.heartbeatLoop(ctx)
	}()

	firstError := <-errCh
	// Closing the socket releases the other loops before we join them.
	c.running.Store(false)
	cancel()

	_ = c.conn.CloseNow()
	<-errCh
	<-errCh

	return firstError
}

func (c *Session) heartbeatLoop(ctx context.Context) error {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, c.heartbeatTimeout)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return fmt.Errorf("ping websocket: %w", err)
			}
		}
	}
}

func (c *Session) readLoop(ctx context.Context, handleMessage HandlerFunc) error {
	for {
		_, message, err := c.conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read websocket message: %w", err)
		}

		if err := handleMessage(ctx, c, json.RawMessage(message)); err != nil {
			return fmt.Errorf("handle websocket message: %w", err)
		}
	}
}

func (c *Session) writeLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case message := <-c.outgoing:
			if err := wsjson.Write(ctx, c.conn, message); err != nil {
				return fmt.Errorf("write websocket message: %w", err)
			}
		}
	}
}

// Send queues a message without allowing a slow client to block its caller.
// It is safe for concurrent use; success means queued, not delivered.
func (c *Session) Send(ctx context.Context, message protocol.ServerEnvelope) error {
	if !c.running.Load() {
		return protocol.ErrSessionNotRunning
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case c.outgoing <- message:
		return nil
	default:
		slog.Warn("outgoing queue full", "event", "message.dropped", "reason", "queue_full",
			"profile_id", c.profileID, "message_type", message.Type, "request_id", message.RequestID)
		return protocol.ErrSessionQueueFull
	}
}
