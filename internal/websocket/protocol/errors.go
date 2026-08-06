package protocol

type ErrorCode string

const (
	ErrorInvalidMessage ErrorCode = "invalid_message"
	ErrorUnknownType    ErrorCode = "unknown_message_type"
	ErrorGameNotFound   ErrorCode = "game_not_found"
	ErrorGameFull       ErrorCode = "game_full"
	ErrorNotPlayer      ErrorCode = "not_a_player"
	ErrorNotYourTurn    ErrorCode = "not_your_turn"
	ErrorIllegalMove    ErrorCode = "illegal_move"
	ErrorGameFinished   ErrorCode = "game_finished"
	ErrorNotImplemented ErrorCode = "not_implemented"
	ErrorInternal       ErrorCode = "internal_error"
)
