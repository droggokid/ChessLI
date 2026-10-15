package play

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"ChessLI/internal/websocket/protocol"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const initialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
const afterE4FEN = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"

func TestLateCreateAcknowledgmentClearsTimeoutFeedbackAndPreservesDraft(t *testing.T) {
	model, request := submitText(t, &Model{}, "/create")
	model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
	model.input = "/join another-game"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameCreated, RequestID: request.RequestID,
		Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorWhite}}, true)
	view := model.View(74, true)
	if model.gameID != "game-1" || model.pending != (pendingAction{}) || model.action != actionIdle || model.input != "/join another-game" || model.message != "" || !strings.Contains(view, "Waiting for the other player to join.") || strings.Contains(view, "Restart") {
		t.Fatalf("late create did not resolve timeout: %+v\n%s", model, view)
	}
}

func TestMatchmakingAdmissionClearsOnlyUnchangedInput(t *testing.T) {
	for _, draft := range []string{"/match", "e4"} {
		t.Run(draft, func(t *testing.T) {
			model, request := submitText(t, &Model{}, "/match")
			if request.Type != protocol.ClientEnterMatchmaking || request.Payload != (protocol.EnterMatchmakingPayload{TimeControl: protocol.TimeControlRapid10Plus0}) {
				t.Fatalf("match request = %+v", request)
			}
			model.input = draft
			entered := protocol.ServerEvent{Type: protocol.ServerMatchmakingEntered, RequestID: "other-request",
				Matchmaking: &protocol.EnterMatchmakingPayload{TimeControl: protocol.TimeControlRapid10Plus0}}
			model, _ = model.Update(entered, true)
			if model.action != actionAwaiting || model.input != draft {
				t.Fatal("unrelated admission changed request")
			}
			model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
			if model.action != actionUnknown {
				t.Fatal("entry request had no response deadline")
			}
			entered.RequestID = request.RequestID
			model, _ = model.Update(entered, true)
			want := draft
			if draft == "/match" {
				want = ""
			}
			if model.action != actionQueued || !model.IsPending(request.RequestID) || model.input != want || model.message != "" {
				t.Fatalf("admission = %+v", model)
			}
			// Waiting has no deadline, even when the entry timer fires repeatedly.
			for range 3 {
				model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
			}
			if model.action != actionQueued || !strings.Contains(model.View(74, true), "Waiting for a matchmaking opponent") {
				t.Fatal("admitted ticket timed out")
			}
			for _, text := range []string{"/match", "/create", "/join game-1", "e4", "/resign"} {
				model.input = text
				if _, cmd := model.submit(true); cmd != nil {
					t.Fatalf("allowed %q while queued", text)
				}
			}
		})
	}
}

func TestMatchmakingAssignmentWaitsForInitialStateAndAcceptsPresence(t *testing.T) {
	for _, color := range []protocol.Color{protocol.ColorWhite, protocol.ColorBlack} {
		t.Run(string(color), func(t *testing.T) {
			model, request := submitText(t, &Model{}, "/match")
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerMatchmakingEntered, RequestID: request.RequestID,
				Matchmaking: &protocol.EnterMatchmakingPayload{TimeControl: protocol.TimeControlRapid10Plus0}}, true)
			// A new draft equal to the original command must survive later stages.
			model.input = "/match"
			found := protocol.ServerEvent{Type: protocol.ServerMatchFound, RequestID: "other-request",
				Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: color}}
			model, _ = model.Update(found, true)
			if model.gameID != "" || model.action != actionQueued {
				t.Fatal("unrelated assignment changed ticket")
			}
			found.RequestID = request.RequestID
			model, _ = model.Update(found, true)
			model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
			if model.gameID != "game-1" || model.color != color || model.action != actionInitializing || model.state != nil || model.input != "/match" || model.message != "" || !strings.Contains(model.View(74, true), "Waiting for the initial game state") {
				t.Fatalf("assignment = %+v", model)
			}
			// A state broadcast cannot replace the initial-state admission barrier.
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: initialState()}, true)
			for _, text := range []string{"e4", "/resign", "/create", "/match"} {
				model.input = text
				if _, cmd := model.submit(true); cmd != nil || model.state != nil {
					t.Fatalf("allowed %q before initial state", text)
				}
			}
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: initialState()}, true)
			if model.action != actionIdle || model.pending != (pendingAction{}) || model.input != "/match" || model.board() == "" {
				t.Fatalf("initial state = %+v", model)
			}
			presence := initialState()
			presence.White = &protocol.PlayerState{Color: protocol.ColorWhite, Connected: true}
			presence.Black = &protocol.PlayerState{Color: protocol.ColorBlack, Connected: true}
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: presence}, true)
			model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
			view := model.View(74, true)
			if model.state != presence || !strings.Contains(view, "White: online") || !strings.Contains(view, "Black: online") || model.action != actionIdle {
				t.Fatalf("presence or obsolete deadline changed state: %s", view)
			}
		})
	}
}

func TestMatchmakingRecoveryRequiresAnAssignedGameID(t *testing.T) {
	for _, stage := range []string{"entry", "queued", "assigned", "initialized"} {
		t.Run(stage, func(t *testing.T) {
			model := activeModel(t)
			finished := initialState()
			finished.Status = protocol.GameStatusFinished
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: finished}, true)
			model, request := submitText(t, model, "/match")
			if stage != "entry" {
				model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerMatchmakingEntered, RequestID: request.RequestID,
					Matchmaking: &protocol.EnterMatchmakingPayload{TimeControl: protocol.TimeControlRapid10Plus0}}, true)
			}
			// Late snapshots for the finished game cannot clear the queue correlation.
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: finished}, true)
			if !model.IsPending(request.RequestID) {
				t.Fatal("previous game cleared ticket")
			}
			if stage == "assigned" || stage == "initialized" {
				model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerMatchFound, RequestID: request.RequestID,
					Assignment: &protocol.GameCreatedPayload{GameID: "game-2", Color: protocol.ColorBlack}}, true)
			}
			state := initialState()
			state.GameID = "game-2"
			if stage == "initialized" {
				model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: state}, true)
			}
			model.input = "e5"
			model, _ = model.Update(DisconnectedMsg{}, false)
			model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, false)
			model, cmd := model.Recover()
			if stage == "entry" || stage == "queued" {
				if cmd != nil || model.action != actionUnknown || !strings.Contains(model.View(74, true), "Restart the TUI") {
					t.Fatal("unknown match ID requeued or resumed previous game")
				}
				if _, cmd := model.submit(true); cmd != nil {
					t.Fatal("unknown match ID allowed another action")
				}
				return
			}
			if cmd == nil {
				t.Fatal("assigned game was not resumed")
			}
			resume := cmd().(RequestMsg)
			if resume.Type != protocol.ClientResumeGame || resume.Payload != (protocol.ResumeGamePayload{GameID: "game-2"}) {
				t.Fatalf("recovery = %+v", resume)
			}
			model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
			if model.action != actionAwaiting || !model.IsPending(resume.RequestID) {
				t.Fatal("entry deadline affected resume")
			}
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameResumed, RequestID: resume.RequestID,
				Assignment: &protocol.GameCreatedPayload{GameID: "game-2", Color: protocol.ColorBlack}}, true)
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: state}, true)
			if model.action != actionIdle || model.input != "e5" || model.state != state {
				t.Fatalf("recovered match = %+v", model)
			}
		})
	}
}

func TestMatchmakingUsesExistingAdmissionRestrictions(t *testing.T) {
	for _, stage := range []string{"disconnected", "private waiting", "active", "pending"} {
		t.Run(stage, func(t *testing.T) {
			model := &Model{}
			ready := true
			switch stage {
			case "disconnected":
				ready = false
			case "private waiting":
				model.gameID = "game-1"
			case "active":
				model = activeModel(t)
			case "pending":
				model, _ = submitText(t, model, "/create")
			}
			model.input = "/match"
			if _, cmd := model.submit(ready); cmd != nil {
				t.Fatalf("allowed matchmaking while %s", stage)
			}
		})
	}
}

func TestCreateWaitsForAuthoritativeInitialState(t *testing.T) {
	model := &Model{}
	model, request := submitText(t, model, "/create")
	if request.Type != protocol.ClientCreateGame {
		t.Fatalf("request = %#v", request)
	}
	payload := request.Payload.(protocol.CreateGamePayload)
	if payload.TimeControl != protocol.TimeControlRapid10Plus0 || payload.Color != protocol.ColorPreferenceWhite {
		t.Fatalf("payload = %#v", payload)
	}
	model, _ = model.Update(protocol.ServerEvent{
		Type: protocol.ServerGameCreated, RequestID: request.RequestID,
		Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorWhite},
	}, true)
	if model.state != nil || model.board() != "" || !strings.Contains(model.View(74, true), "Waiting for the other player") {
		t.Fatal("create rendered an unconfirmed board")
	}
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: initialState()}, true)
	if model.state == nil || model.board() == "" || model.turn() != protocol.ColorWhite {
		t.Fatal("initial state did not initialize board")
	}
}

func TestJoinWaitsForInitialState(t *testing.T) {
	model := &Model{}
	model, request := submitText(t, model, "/join game-1")
	model, _ = model.Update(protocol.ServerEvent{
		Type: protocol.ServerGameJoined, RequestID: request.RequestID,
		Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorBlack},
	}, true)
	if !model.IsPending(request.RequestID) {
		t.Fatal("join completed before initial state")
	}
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: initialState()}, true)
	if model.pending.id != "" || model.board() == "" || model.color != protocol.ColorBlack {
		t.Fatal("initial state did not complete join")
	}
	if !strings.HasPrefix(model.board(), " H G F E D C B A") {
		t.Fatalf("black board has wrong orientation: %s", model.board())
	}
}

func TestMoveUsesLatestVersionAndWaitsForCorrelatedResponse(t *testing.T) {
	for _, notation := range []struct {
		text string
		want protocol.MoveNotation
	}{
		{"e4", protocol.MoveNotationSAN}, {"e2e4", protocol.MoveNotationUCI}, {"a7a8q", protocol.MoveNotationUCI},
	} {
		t.Run(notation.text, func(t *testing.T) {
			model := activeModel(t)
			originalBoard := model.board()
			model, request := submitText(t, model, notation.text)
			payload := request.Payload.(protocol.MovePayload)
			if payload.ExpectedVersion == nil || *payload.ExpectedVersion != 0 || payload.Notation != notation.want {
				t.Fatalf("move = %#v", payload)
			}
			model, _ = model.Update(SendResultMsg{RequestID: request.RequestID}, true)
			if !model.IsPending(request.RequestID) || model.board() != originalBoard {
				t.Fatal("write success changed authoritative state or cleared pending")
			}
			model.input = "d4"
			pending := model.pending
			next, cmd := model.submit(true)
			if cmd != nil || next.pending != pending {
				t.Fatal("allowed a second pending move")
			}
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerError, RequestID: "other-request", Error: &protocol.ErrorPayload{Code: protocol.ErrorIllegalMove, Message: "unrelated"}}, true)
			if !model.IsPending(request.RequestID) {
				t.Fatal("unrelated error cleared pending move")
			}
			state := initialState()
			state.Version, state.FEN, state.LastMove = 1, afterE4FEN, "e4"
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: state}, true)
			if !model.IsPending(request.RequestID) {
				t.Fatal("uncorrelated broadcast cleared pending move")
			}
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, RequestID: request.RequestID, State: state}, true)
			if model.pending.id != "" || model.board() == originalBoard || !reflect.DeepEqual(model.moves, []string{"v1 e4"}) {
				t.Fatalf("confirmed move not applied: %#v", model)
			}
		})
	}
}

func TestInvalidMoveFeedbackPreservesPositionAndInput(t *testing.T) {
	model := activeModel(t)
	board := model.board()
	model, request := submitText(t, model, "Qa5")
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerError, RequestID: request.RequestID, Error: &protocol.ErrorPayload{Code: protocol.ErrorIllegalMove, Message: "illegal move"}}, true)
	if model.pending.id != "" || model.input != "Qa5" || model.board() != board || !strings.Contains(model.View(74, true), "illegal_move") {
		t.Fatal("invalid move feedback lost input or changed board")
	}
}

func TestSnapshotsAcceptEqualVersionsAndRejectOlderOrOtherGames(t *testing.T) {
	model := activeModel(t)
	state := initialState()
	state.Version = 2
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: state}, true)
	equal := *state
	equal.White = &protocol.PlayerState{ProfileID: "profile-1", Color: protocol.ColorWhite, Connected: false}
	equal.PendingDrawOffer = &protocol.DrawOfferPayload{OfferID: "offer-1", OfferedBy: protocol.ColorBlack}
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: &equal}, true)
	if model.state != &equal {
		t.Fatal("equal-version presence/draw update was ignored")
	}
	older := equal
	older.Version = 1
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: &older}, true)
	other := equal
	other.GameID = "other-game"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: &other}, true)
	if model.state != &equal {
		t.Fatal("older or other-game state replaced current state")
	}
}

func TestUncertainOrStaleMoveRequiresRecovery(t *testing.T) {
	for _, cause := range []string{"timeout", "write failure", "stale version"} {
		t.Run(cause, func(t *testing.T) {
			model := activeModel(t)
			model, request := submitText(t, model, "e4")
			switch cause {
			case "timeout":
				model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
			case "write failure":
				model, _ = model.Update(SendResultMsg{RequestID: request.RequestID, Err: errors.New("closed")}, true)
			case "stale version":
				model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerError, RequestID: request.RequestID, Error: &protocol.ErrorPayload{Code: protocol.ErrorStaleGameVersion, Message: "stale game version"}}, true)
			}
			model.input = "d4"
			if _, cmd := model.submit(true); cmd != nil {
				t.Fatal("move allowed before recovery")
			}
			model, _ = model.Update(DisconnectedMsg{}, true)
			if model.state == nil || model.board() == "" {
				t.Fatal("disconnect discarded board")
			}
			model, cmd := model.Recover()
			resumed := cmd().(RequestMsg)
			if resumed.Type != protocol.ClientResumeGame {
				t.Fatal("recovery resent an action")
			}
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameResumed, RequestID: resumed.RequestID, Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorWhite}}, true)
			if _, cmd := model.submit(true); cmd != nil {
				t.Fatal("assignment cleared recovery before initial snapshot")
			}
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: initialState()}, true)
			if model.IsPending(resumed.RequestID) || !strings.Contains(model.View(74, true), "Your turn") {
				t.Fatal("initial snapshot did not finish recovery")
			}
		})
	}
}

func TestMissingCreateResponseIsNeverRetried(t *testing.T) {
	model := &Model{}
	model, request := submitText(t, model, "/create")
	model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
	model, cmd := model.Recover()
	if cmd != nil || !strings.Contains(model.View(74, true), "no game ID") {
		t.Fatal("unknown create outcome must require explicit recovery")
	}
}

func TestFinishedGameAllowsAnotherGameAndResetsVersionTracking(t *testing.T) {
	model := activeModel(t)
	model, request := submitText(t, model, "/resign")
	finished := initialState()
	finished.Status, finished.Version = protocol.GameStatusFinished, 9
	finished.Outcome = &protocol.GameOutcome{Result: protocol.ResultBlackWin, Reason: protocol.GameOverResignation}
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, RequestID: request.RequestID, State: finished}, true)
	model, create := submitText(t, model, "/create")
	// A late terminal snapshot for the previous game must not acknowledge the new request.
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: finished}, true)
	if !model.IsPending(create.RequestID) {
		t.Fatal("old terminal snapshot cleared new create request")
	}
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameCreated, RequestID: create.RequestID, Assignment: &protocol.GameCreatedPayload{GameID: "game-2", Color: protocol.ColorWhite}}, true)
	next := initialState()
	next.GameID = "game-2"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: next}, true)
	if model.state != next || model.state.Version != 0 || len(model.moves) != 0 {
		t.Fatal("new game inherited old state tracking")
	}
}

func TestSessionMovesMarkGapsAndDoNotDuplicateOnResignation(t *testing.T) {
	model := activeModel(t)
	state := initialState()
	state.Version, state.FEN, state.LastMove = 3, afterE4FEN, "e4"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: state}, true)
	finished := *state
	finished.Version, finished.Status = 4, protocol.GameStatusFinished
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: &finished}, true)
	if !reflect.DeepEqual(model.moves, []string{"… missing updates …", "v3 e4"}) {
		t.Fatalf("moves = %#v", model.moves)
	}
}

func TestInvalidFENPreservesLastBoardAndBlocksSubmission(t *testing.T) {
	model := activeModel(t)
	board := model.board()
	invalid := initialState()
	invalid.Version, invalid.FEN = 1, "invalid"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: invalid}, true)
	if model.board() != board || model.state == invalid {
		t.Fatal("invalid FEN replaced authoritative board or permitted submission")
	}
	model.input = "e4"
	if _, cmd := model.submit(true); cmd != nil {
		t.Fatal("invalid FEN permitted submission")
	}
}

func initialState() *protocol.GameStatePayload {
	return &protocol.GameStatePayload{GameID: "game-1", FEN: initialFEN, Status: protocol.GameStatusActive}
}

func activeModel(t *testing.T) *Model {
	t.Helper()
	model := &Model{gameID: "game-1", color: protocol.ColorWhite}
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: initialState()}, true)
	if model.state == nil {
		t.Fatal("fixture did not initialize")
	}
	return model
}

func submitText(t *testing.T, model *Model, text string) (*Model, RequestMsg) {
	t.Helper()
	model.input = text
	model, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), true)
	if cmd == nil {
		t.Fatalf("no request for %q: %s", text, model.View(74, true))
	}
	request, ok := cmd().(RequestMsg)
	if !ok {
		t.Fatalf("unexpected command for %q", text)
	}
	return model, request
}

func TestMoveIncludesNonzeroAuthoritativeVersion(t *testing.T) {
	model := activeModel(t)
	state := initialState()
	state.Version = 7
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: state}, true)
	_, request := submitText(t, model, "e4")
	move := request.Payload.(protocol.MovePayload)
	if move.ExpectedVersion == nil || *move.ExpectedVersion != 7 {
		t.Fatalf("version = %v, want 7", move.ExpectedVersion)
	}
}

func TestResumeRejectionKeepsRecoveryTargetForRetry(t *testing.T) {
	model := &Model{}
	model, join := submitText(t, model, "/join game-1")
	model, _ = model.Update(TimeoutMsg{RequestID: join.RequestID}, true)
	model, cmd := model.Recover()
	resume := cmd().(RequestMsg)
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerError, RequestID: resume.RequestID,
		Error: &protocol.ErrorPayload{Code: protocol.ErrorInvalidMessage, Message: "session already registered"}}, true)
	if _, cmd := model.submit(true); cmd != nil {
		t.Fatal("resume rejection enabled new actions")
	}
	model, cmd = model.Recover()
	if cmd == nil {
		t.Fatal("lost recovery target")
	}
	retry := cmd().(RequestMsg)
	if retry.Type != protocol.ClientResumeGame || retry.Payload.(protocol.ResumeGamePayload).GameID != "game-1" {
		t.Fatalf("retry = %#v", retry)
	}
}

func TestAuthoritativeStateCanFinishRecoveryAfterResumeRejection(t *testing.T) {
	for _, eventType := range []protocol.ServerMessageType{protocol.ServerGameInitial, protocol.ServerGameState} {
		t.Run(string(eventType), func(t *testing.T) {
			model := activeModel(t)
			state := initialState()
			if eventType == protocol.ServerGameState {
				state.Status = protocol.GameStatusFinished
				model, _ = model.Update(protocol.ServerEvent{Type: eventType, State: state}, true)
			}
			model, cmd := model.Recover()
			resume := cmd().(RequestMsg)
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerError, RequestID: resume.RequestID,
				Error: &protocol.ErrorPayload{Code: protocol.ErrorInvalidMessage, Message: "session already registered"}}, true)
			model, _ = model.Update(protocol.ServerEvent{Type: eventType, State: state}, true)
			text := "e4"
			if state.Status == protocol.GameStatusFinished {
				text = "/create"
			}
			model, request := submitText(t, model, text)
			if !model.IsPending(request.RequestID) {
				t.Fatal("authoritative state did not release recovery")
			}
		})
	}
}

func TestRepeatedFinishedSnapshotsPreserveNewInput(t *testing.T) {
	model := activeModel(t)
	finished := initialState()
	finished.Status, finished.Version = protocol.GameStatusFinished, 1
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: finished}, true)
	model, _ = model.Update(tea.PasteMsg{Content: "/create"}, true)
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: finished}, true)
	if model.input != "/create" {
		t.Fatal("repeated terminal snapshot erased new input")
	}
}

func TestAcknowledgmentPreservesEditedInput(t *testing.T) {
	model, request := submitText(t, activeModel(t), "e4")
	model.input = "Nf3"
	state := initialState()
	state.Version, state.FEN, state.LastMove = 1, afterE4FEN, "e4"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, RequestID: request.RequestID, State: state}, true)
	if model.pending.id != "" || model.input != "Nf3" {
		t.Fatalf("acknowledgment lost edited input: pending=%+v input=%q", model.pending, model.input)
	}
}

func TestAssignmentAndRecoveryPreserveDraftInput(t *testing.T) {
	model, request := submitText(t, &Model{}, "/join game-1")
	model.input = "e5"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameJoined, RequestID: request.RequestID,
		Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorBlack}}, true)
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: initialState()}, true)
	if model.input != "e5" {
		t.Fatal("join response erased draft")
	}
	model, cmd := model.Recover()
	resume := cmd().(RequestMsg)
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameResumed, RequestID: resume.RequestID,
		Assignment: &protocol.GameCreatedPayload{GameID: "game-1", Color: protocol.ColorBlack}}, true)
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameInitial, State: initialState()}, true)
	if model.input != "e5" {
		t.Fatal("recovery erased draft")
	}
}

func TestOpponentMoveClearsStaleTurnFeedback(t *testing.T) {
	model := activeModel(t)
	model.color = protocol.ColorBlack
	model.input = "e5"
	model, _ = model.submit(true)
	state := initialState()
	state.Version, state.FEN, state.LastMove = 1, afterE4FEN, "e4"
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: state}, true)
	if model.message != "" || !strings.Contains(model.View(74, true), "Your turn") {
		t.Fatalf("stale turn feedback: %s", model.View(74, true))
	}
}

func TestReconnectWithoutGameClearsDisconnectedFeedback(t *testing.T) {
	model := &Model{input: "/create"}
	model, cmd := model.submit(false)
	if cmd != nil || !strings.Contains(model.View(76, false), "Disconnected") {
		t.Fatal("disconnected submission did not show feedback")
	}
	model, cmd = model.Recover()
	if cmd != nil || model.message != "" || strings.Contains(model.View(76, true), "Disconnected") {
		t.Fatal("reconnection retained stale disconnected feedback")
	}
}

func TestJoinWithoutIDShowsUsage(t *testing.T) {
	for _, input := range []string{"/join", "/join   "} {
		model := &Model{input: input}
		model, cmd := model.submit(true)
		if cmd != nil || model.message != "Use /join GAME_ID." {
			t.Fatalf("input %q: cmd=%v message=%q", input, cmd != nil, model.message)
		}
	}
}

func TestLateAcknowledgmentResolvesUnknownOutcome(t *testing.T) {
	model, request := submitText(t, activeModel(t), "e4")
	if !strings.Contains(model.View(74, true), "Awaiting server response") {
		t.Fatal("request was not shown as awaiting acknowledgment")
	}
	model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
	if !strings.Contains(model.View(74, true), "outcome unknown") {
		t.Fatal("timeout was not shown as an unknown outcome")
	}
	state := initialState()
	state.Version, state.FEN = 1, afterE4FEN
	model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, RequestID: request.RequestID, State: state}, true)
	if model.IsPending(request.RequestID) || model.HasInput() || !strings.Contains(model.View(74, true), "Opponent’s turn") {
		t.Fatal("late acknowledgment did not resolve the unknown outcome")
	}
	model, _ = model.Update(TimeoutMsg{RequestID: request.RequestID}, true)
	if strings.Contains(model.View(74, true), "unknown") {
		t.Fatal("completed request became uncertain again")
	}
}

func TestDisconnectPreservesBoardAndDraftUntilRecovery(t *testing.T) {
	model := activeModel(t)
	model.input = "e4"
	board := model.board()
	model, _ = model.Update(DisconnectedMsg{}, false)
	if model.board() != board || model.input != "e4" || !strings.Contains(model.View(74, false), "Disconnected") {
		t.Fatal("disconnect discarded board or draft")
	}
	if _, cmd := model.submit(true); cmd != nil {
		t.Fatal("readiness alone permitted submission without recovery")
	}
	if !strings.Contains(model.View(74, true), "reconnect and recover") {
		t.Fatal("known game was not shown as requiring recovery")
	}
}

func TestResumeTerminalRejectionPermitsNewGame(t *testing.T) {
	for _, code := range []protocol.ErrorCode{protocol.ErrorGameNotFound, protocol.ErrorNotPlayer} {
		t.Run(string(code), func(t *testing.T) {
			model, join := submitText(t, &Model{}, "/join game-1")
			model, _ = model.Update(TimeoutMsg{RequestID: join.RequestID}, true)
			model, cmd := model.Recover()
			resume := cmd().(RequestMsg)
			model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerError, RequestID: resume.RequestID,
				Error: &protocol.ErrorPayload{Code: code, Message: "cannot resume"}}, true)
			if _, cmd := model.Recover(); cmd != nil {
				t.Fatal("terminal rejection retained a recovery target")
			}
			_, create := submitText(t, model, "/create")
			if create.Type != protocol.ClientCreateGame {
				t.Fatal("terminal rejection blocked a new game")
			}
		})
	}
}

func TestRecentMovesRetainOnlyDisplayedEntries(t *testing.T) {
	model := activeModel(t)
	for version := uint64(1); version <= 6; version++ {
		state := initialState()
		state.Version, state.LastMove = version, "move"
		if version%2 == 1 {
			state.FEN = afterE4FEN
		}
		model, _ = model.Update(protocol.ServerEvent{Type: protocol.ServerGameState, State: state}, true)
	}
	if !reflect.DeepEqual(model.moves, []string{"v3 move", "v4 move", "v5 move", "v6 move"}) {
		t.Fatalf("recent moves = %v", model.moves)
	}
}

func TestViewKeepsLongInputCursorVisibleAndAlignsBoard(t *testing.T) {
	model := activeModel(t)
	model.input = strings.Repeat("界", 60) + "e2e4"
	view := model.View(74, true)
	input := strings.Split(view, "\n")[1]
	if lipgloss.Width(input) > 74 || !strings.HasSuffix(input, "e2e4▏") {
		t.Fatalf("input overflowed or hid cursor: %q", input)
	}
	if !strings.HasPrefix(model.board(), " A B C D E F G H\n8") {
		t.Fatalf("board labels do not align: %s", model.board())
	}
}
