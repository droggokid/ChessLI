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

var errMessageHandlerRequired = errors.New("websocket message handler is required")

type sessionRunState uint32

const (
	sessionNotStarted sessionRunState = iota
	sessionRunning
	sessionStopped
)

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
	runState  atomic.Uint32

	outgoing chan protocol.ServerEnvelope
}

// NewSession creates a Session for conn.
func NewSession(conn *coderws.Conn) *Session {
	return &Session{
		profileID: identity.NewProfileID(),
		conn:      conn,
		outgoing:  make(chan protocol.ServerEnvelope, outgoingBufferSize),
	}
}

// Run exchanges messages until the context is canceled or an I/O loop stops.
// It may be called only once.
func (c *Session) Run(ctx context.Context, handleMessage HandlerFunc) error {
	if handleMessage == nil {
		return errMessageHandlerRequired
	}
	if !c.runState.CompareAndSwap(uint32(sessionNotStarted), uint32(sessionRunning)) {
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
	c.runState.Store(uint32(sessionStopped))
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

		case message := <-c.outgoing:
			if err := wsjson.Write(ctx, c.conn, message); err != nil {
				return fmt.Errorf(
					"write websocket message: %w",
					err,
				)
			}
		}
	}
}

// Send queues a message without allowing a slow client to block its caller.
// It is safe for concurrent use.
func (c *Session) Send(ctx context.Context, message protocol.ServerEnvelope) error {
	if sessionRunState(c.runState.Load()) != sessionRunning {
		return protocol.ErrSessionNotRunning
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case c.outgoing <- message:
		return nil
	default:
		return protocol.ErrSessionQueueFull
	}
}
