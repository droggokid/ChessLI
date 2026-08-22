package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

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
type Session struct {
	profileID identity.ProfileID
	conn      *coderws.Conn
	started   atomic.Bool

	outgoing chan protocol.ServerEnvelope
	done     chan struct{}
}

// NewSession creates a client for conn.
func NewSession(conn *coderws.Conn) *Session {
	return &Session{
		profileID: identity.NewProfileID(),
		conn:      conn,
		outgoing:  make(chan protocol.ServerEnvelope, outgoingBufferSize),
		done:      make(chan struct{}),
	}
}

// Run exchanges messages until the context is canceled or an I/O loop stops.
func (c *Session) Run(ctx context.Context, handleMessage HandlerFunc) error {
	if handleMessage == nil {
		return errors.New("websocket message handler is required")
	}
	if !c.started.CompareAndSwap(false, true) {
		return protocol.ErrSessionAlreadyRun
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)

	go func() {
		errCh <- c.readLoop(ctx, handleMessage)
	}()

	go func() {
		errCh <- c.writeLoop(ctx)
	}()

	firstError := <-errCh
	close(c.done)
	cancel()

	_ = c.conn.CloseNow()
	<-errCh

	return firstError
}

func (c *Session) readLoop(ctx context.Context, handleMessage HandlerFunc) error {
	for {
		var message json.RawMessage

		if err := wsjson.Read(ctx, c.conn, &message); err != nil {
			return fmt.Errorf("read websocket message: %w", err)
		}

		if err := handleMessage(ctx, c, message); err != nil {
			return fmt.Errorf("handle websocket message: %w", err)
		}
	}
}

func (c *Session) writeLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case message, ok := <-c.outgoing:
			if !ok {
				return nil
			}

			if err := wsjson.Write(ctx, c.conn, message); err != nil {
				return fmt.Errorf(
					"write websocket message: %w",
					err,
				)
			}
		}
	}
}

// Send queues a message for writing or returns if the context or client closes.
func (c *Session) Send(ctx context.Context, message protocol.ServerEnvelope) error {
	if !c.started.Load() {
		return protocol.ErrSessionNotRunning
	}

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-c.done:
		return protocol.ErrSessionNotRunning

	case c.outgoing <- message:
		return nil
	}
}
