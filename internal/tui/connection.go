package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"ChessLI/internal/identity"
	"ChessLI/internal/tui/screens/play"
	"ChessLI/internal/websocket/protocol"

	tea "charm.land/bubbletea/v2"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const operationTimeout = 5 * time.Second

type connectedMsg struct {
	generation uint64
	client     *websocket.Conn
	err        error
}

type receivedMsg struct {
	generation uint64
	event      protocol.ServerEvent
	err        error
}

type sentMsg struct {
	generation uint64
	result     play.SendResultMsg
}

// messageError means the frame was consumed and receiving may continue.
type messageError struct{ Err error }

func (e *messageError) Error() string { return "decode server message: " + e.Err.Error() }
func (e *messageError) Unwrap() error { return e.Err }

// connection owns the TUI's network lifetime, including commands that outlive Run.
type connection struct {
	ctx        context.Context
	cancel     context.CancelFunc
	address    string
	mu         sync.Mutex
	client     *websocket.Conn
	operations sync.WaitGroup
	closeOnce  sync.Once
	closeErr   error
}

func newConnection(address string) *connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &connection{ctx: ctx, cancel: cancel, address: address}
}

func (c *connection) begin() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx.Err() != nil {
		return false
	}
	c.operations.Add(1)
	return true
}

func (c *connection) connect(profileID identity.ProfileID, generation uint64) tea.Cmd {
	return func() tea.Msg {
		if !c.begin() {
			return connectedMsg{generation: generation, err: context.Canceled}
		}
		defer c.operations.Done()

		c.mu.Lock()
		old := c.client
		c.client = nil
		c.mu.Unlock()
		if old != nil {
			if err := closeSocket(old); err != nil {
				return connectedMsg{generation: generation, err: err}
			}
		}

		address, err := url.Parse(c.address)
		if err != nil {
			return connectedMsg{generation: generation, err: err}
		}
		if profileID != "" {
			query := address.Query()
			query.Set("profileId", string(profileID))
			address.RawQuery = query.Encode()
		}
		ctx, cancel := context.WithTimeout(c.ctx, operationTimeout)
		defer cancel()
		client, _, err := websocket.Dial(ctx, address.String(), nil)
		if err != nil {
			return connectedMsg{generation: generation, err: fmt.Errorf("connect: %w", err)}
		}

		c.mu.Lock()
		closed := c.ctx.Err() != nil
		if !closed {
			c.client = client
		}
		c.mu.Unlock()
		if closed {
			_ = closeSocket(client)
			return connectedMsg{generation: generation, err: context.Canceled}
		}
		return connectedMsg{generation: generation, client: client}
	}
}

// receive is chained by Update so only one command reads a socket at a time.
func (c *connection) receive(client *websocket.Conn, generation uint64) tea.Cmd {
	return func() tea.Msg {
		if !c.begin() {
			return receivedMsg{generation: generation, err: context.Canceled}
		}
		defer c.operations.Done()
		kind, data, err := client.Read(c.ctx)
		if err != nil {
			return receivedMsg{generation: generation, err: fmt.Errorf("receive: %w", err)}
		}
		if kind != websocket.MessageText {
			return receivedMsg{generation: generation, err: &messageError{Err: errors.New("expected a text message")}}
		}
		event, err := protocol.DecodeServerEvent(data)
		if err != nil {
			err = &messageError{Err: err}
		}
		return receivedMsg{generation: generation, event: event, err: err}
	}
}

// send reports socket write success, not server acceptance.
func (c *connection) send(client *websocket.Conn, generation uint64, request play.RequestMsg) tea.Cmd {
	return func() tea.Msg {
		if !c.begin() {
			return sentMsg{generation: generation, result: play.SendResultMsg{RequestID: request.RequestID, Err: context.Canceled}}
		}
		defer c.operations.Done()
		ctx, cancel := context.WithTimeout(c.ctx, operationTimeout)
		defer cancel()
		raw, err := json.Marshal(request.Payload)
		if err != nil {
			err = fmt.Errorf("encode payload: %w", err)
		} else if err = wsjson.Write(ctx, client, protocol.ClientEnvelope{
			Type: request.Type, RequestID: request.RequestID, Payload: raw,
		}); err != nil {
			err = fmt.Errorf("send: %w", err)
		}
		return sentMsg{generation: generation, result: play.SendResultMsg{RequestID: request.RequestID, Err: err}}
	}
}

func (c *connection) responseDeadline(requestID string) tea.Cmd {
	return func() tea.Msg {
		if !c.begin() {
			return nil
		}
		defer c.operations.Done()
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			return play.TimeoutMsg{RequestID: requestID}
		case <-c.ctx.Done():
			return nil
		}
	}
}

func (c *connection) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.cancel()
		client := c.client
		c.client = nil
		c.mu.Unlock()
		if client != nil {
			c.closeErr = closeSocket(client)
		}
		c.operations.Wait()
	})
	return c.closeErr
}

// Skip the closing handshake so shutdown cannot wait on a peer.
func closeSocket(client *websocket.Conn) error {
	err := client.CloseNow()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
