package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrUnsupportedEvent = errors.New("unsupported server event")

type ServerEvent struct {
	Type        ServerMessageType
	RequestID   string
	Ready       *ConnectionReadyPayload
	Matchmaking *EnterMatchmakingPayload
	Assignment  *GameCreatedPayload
	State       *GameStatePayload
	Error       *ErrorPayload
}

// DecodeServerEvent validates a server response while accepting additive fields.
func DecodeServerEvent(data []byte) (ServerEvent, error) {
	var envelope RawServerEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ServerEvent{}, err
	}
	if envelope.Type == "" {
		return ServerEvent{}, errors.New("missing message type")
	}
	event := ServerEvent{Type: envelope.Type, RequestID: envelope.RequestID}
	var destination any
	var required []string
	switch envelope.Type {
	case ServerConnectionReady:
		event.Ready = &ConnectionReadyPayload{}
		destination = event.Ready
		required = []string{"profileId"}
	case ServerMatchmakingEntered:
		event.Matchmaking = &EnterMatchmakingPayload{}
		destination = event.Matchmaking
		required = []string{"timeControl"}
	case ServerGameCreated, ServerGameJoined, ServerGameResumed, ServerMatchFound:
		event.Assignment = &GameCreatedPayload{}
		destination = event.Assignment
		required = []string{"gameId", "color"}
	case ServerGameInitial, ServerGameState:
		event.State = &GameStatePayload{}
		destination = event.State
		required = []string{"gameId", "fen", "status", "version"}
	case ServerError:
		event.Error = &ErrorPayload{}
		destination = event.Error
		required = []string{"code", "message"}
	default:
		return event, fmt.Errorf("%w: %q", ErrUnsupportedEvent, envelope.Type)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Payload, &fields); err != nil {
		return ServerEvent{}, fmt.Errorf("%s payload: %w", envelope.Type, err)
	}
	if fields == nil {
		return ServerEvent{}, fmt.Errorf("%s payload must be an object", envelope.Type)
	}
	for _, name := range required {
		raw := bytes.TrimSpace(fields[name])
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			return ServerEvent{}, fmt.Errorf("%s payload missing %q", envelope.Type, name)
		}
		if name != "version" {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
				return ServerEvent{}, fmt.Errorf("%s payload requires nonempty %q", envelope.Type, name)
			}
		}
	}
	if err := json.Unmarshal(envelope.Payload, destination); err != nil {
		return ServerEvent{}, fmt.Errorf("%s payload: %w", envelope.Type, err)
	}
	if event.Assignment != nil && event.Assignment.Color != ColorWhite && event.Assignment.Color != ColorBlack {
		return ServerEvent{}, fmt.Errorf("%s payload has invalid color %q", envelope.Type, event.Assignment.Color)
	}
	if event.State != nil && event.State.Status != GameStatusActive && event.State.Status != GameStatusFinished {
		return ServerEvent{}, fmt.Errorf("%s payload has invalid status %q", envelope.Type, event.State.Status)
	}
	return event, nil
}
