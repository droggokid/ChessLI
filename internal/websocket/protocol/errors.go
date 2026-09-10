package protocol

import "errors"

type ErrorCode string

const (
	ErrorInvalidMessage   ErrorCode = "invalid_message"
	ErrorUnknownType      ErrorCode = "unknown_message_type"
	ErrorGameNotFound     ErrorCode = "game_not_found"
	ErrorGameFull         ErrorCode = "game_full"
	ErrorNotPlayer        ErrorCode = "not_a_player"
	ErrorNotYourTurn      ErrorCode = "not_your_turn"
	ErrorIllegalMove      ErrorCode = "illegal_move"
	ErrorGameFinished     ErrorCode = "game_finished"
	ErrorInternal         ErrorCode = "internal_error"
	ErrorStaleGameVersion ErrorCode = "stale_game_version"
)

var (
	ErrSessionNotRunning    = errors.New("session is not running")
	ErrSessionAlreadyRun    = errors.New("session has already been run")
	ErrSessionAlreadyInGame = errors.New("session is already registered with a game")
	ErrSessionQueueFull     = errors.New("session outgoing queue is full")
)
