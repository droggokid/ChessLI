package play

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"ChessLI/internal/identity"
	"ChessLI/internal/websocket/protocol"

	tea "charm.land/bubbletea/v2"
	"github.com/corentings/chess/v2"
)

type RequestMsg struct {
	Type      protocol.ClientMessageType
	RequestID string
	Payload   any
}

type SendResultMsg struct {
	RequestID string
	Err       error
}

type TimeoutMsg struct{ RequestID string }
type DisconnectedMsg struct{}

type actionStatus uint8

const (
	actionIdle actionStatus = iota
	actionAwaiting
	actionQueued
	actionInitializing
	actionUnknown
	actionRecoveryRequired
)

type pendingAction struct {
	id     string
	kind   protocol.ClientMessageType
	gameID identity.GameID
	input  string
}

type Model struct {
	gameID         identity.GameID
	color          protocol.Color
	state          *protocol.GameStatePayload
	position       *chess.Position
	moves          []string
	input          string
	pending        pendingAction
	sequence       uint64
	action         actionStatus
	recoveryTarget identity.GameID // Retained only after a rejected resume.
	message        string
}

func (m *Model) HasInput() bool                  { return m.input != "" }
func (m *Model) IsPending(requestID string) bool { return m.pending.id == requestID && requestID != "" }

// Recover uses a fresh connection; game.resume cannot use an occupied session.
func (m *Model) Recover() (*Model, tea.Cmd) {
	if m.pending.kind == protocol.ClientCreateGame {
		m.message = "Create outcome unknown; no game ID received. Restart the TUI before creating another game."
		return m, nil
	}
	if m.pending.kind == protocol.ClientEnterMatchmaking && m.pending.gameID == "" {
		m.message = "Matchmaking outcome unknown; no game ID received. Restart the TUI before entering matchmaking again."
		return m, nil
	}
	gameID := m.recoveryTarget
	if gameID == "" {
		gameID = m.gameID
	}
	if m.pending.kind == protocol.ClientJoinGame || m.pending.kind == protocol.ClientResumeGame {
		gameID = m.pending.gameID
	}
	if gameID == "" {
		if m.action == actionIdle {
			m.message = ""
		}
		return m, nil
	}
	return m.request(protocol.ClientResumeGame, gameID, protocol.ResumeGamePayload{GameID: gameID})
}

func (m *Model) Update(msg tea.Msg, ready bool) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case DisconnectedMsg:
		if m.pending.id != "" {
			m.action = actionUnknown
		} else if m.gameID != "" {
			m.action = actionRecoveryRequired
		}
	case SendResultMsg:
		if m.IsPending(msg.RequestID) && msg.Err != nil {
			m.action = actionUnknown
			m.message = "Write failed; outcome unknown. Restart with the same profile ID to recover."
		}
	case TimeoutMsg:
		if m.IsPending(msg.RequestID) && (m.pending.kind != protocol.ClientEnterMatchmaking || m.action == actionAwaiting) {
			m.action = actionUnknown
			m.message = "No response; outcome unknown. Restart with the same profile ID to recover."
			if m.pending.kind == protocol.ClientCreateGame {
				m.message = "No response; create outcome unknown. Restart with the same profile ID to recover."
			}
		}
	case protocol.ServerEvent:
		return m.event(msg)
	case tea.PasteMsg:
		m.input += cleanInput(msg.Content)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.input = ""
		case "backspace":
			_, size := utf8.DecodeLastRuneInString(m.input)
			if size > 0 {
				m.input = m.input[:len(m.input)-size]
			}
		case "enter":
			return m.submit(ready)
		default:
			m.input += cleanInput(msg.Text)
		}
	}
	if m.action == actionIdle {
		switch msg.(type) {
		case tea.PasteMsg, tea.KeyPressMsg:
			m.message = ""
		}
	}
	return m, nil
}

func (m *Model) submit(ready bool) (*Model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	if text == "" {
		return m, nil
	}
	if !ready {
		return m, nil
	}
	if m.action != actionIdle {
		return m, nil
	}

	switch {
	case text == "/match":
		if m.state == nil && m.gameID != "" || m.state != nil && m.state.Status != protocol.GameStatusFinished {
			m.message = "Finish your current game before entering matchmaking."
			return m, nil
		}
		return m.request(protocol.ClientEnterMatchmaking, "", protocol.EnterMatchmakingPayload{TimeControl: protocol.TimeControlRapid10Plus0})
	case text == "/create":
		if m.state == nil && m.gameID != "" || m.state != nil && m.state.Status != protocol.GameStatusFinished {
			m.message = "Finish your current game before creating another."
			return m, nil
		}
		return m.request(protocol.ClientCreateGame, "", protocol.CreateGamePayload{
			TimeControl: protocol.TimeControlRapid10Plus0, Color: protocol.ColorPreferenceWhite,
		})
	case text == "/join" || strings.HasPrefix(text, "/join "):
		if m.state == nil && m.gameID != "" || m.state != nil && m.state.Status != protocol.GameStatusFinished {
			m.message = "Finish your current game before joining another."
			return m, nil
		}
		gameID := identity.GameID(strings.TrimSpace(strings.TrimPrefix(text, "/join")))
		if gameID == "" {
			m.message = "Use /join GAME_ID."
			return m, nil
		}
		return m.request(protocol.ClientJoinGame, gameID, protocol.JoinGamePayload{GameID: gameID})
	}

	if m.state == nil {
		m.message = "Use /match, /create or /join GAME_ID; moves require both players to join."
		return m, nil
	}
	if m.state.Status == protocol.GameStatusFinished {
		m.message = "Game finished. Use /match, /create or /join GAME_ID."
		return m, nil
	}
	if text == "/resign" {
		return m.request(protocol.ClientResign, m.gameID, protocol.ResignPayload{GameID: m.gameID})
	}
	if strings.HasPrefix(text, "/") {
		m.message = "Use /resign, or enter a SAN or UCI move."
		return m, nil
	}
	if m.turn() != m.color {
		m.message = "Wait for your turn."
		return m, nil
	}
	notation := protocol.MoveNotationSAN
	if isUCI(text) {
		notation = protocol.MoveNotationUCI
	}
	return m.request(protocol.ClientMakeMove, m.gameID, protocol.MovePayload{
		GameID: m.gameID, Move: text, Notation: notation, ExpectedVersion: new(m.state.Version),
	})
}

func (m *Model) request(kind protocol.ClientMessageType, gameID identity.GameID, payload any) (*Model, tea.Cmd) {
	m.sequence++
	id := fmt.Sprintf("tui-%d", m.sequence)
	m.pending = pendingAction{id: id, kind: kind, gameID: gameID, input: m.input}
	m.recoveryTarget = ""
	m.action = actionAwaiting
	m.message = ""
	request := RequestMsg{Type: kind, RequestID: id, Payload: payload}
	return m, func() tea.Msg { return request }
}

func (m *Model) event(event protocol.ServerEvent) (*Model, tea.Cmd) {
	if event.Ready != nil {
		// The server knows the game even when its creation response was lost.
		m.clearPending()
		m.recoveryTarget = event.Ready.ActiveGameID
		if m.recoveryTarget != "" {
			return m.Recover()
		}
		m.message = ""
		return m, nil
	}
	if event.Error != nil {
		m.message = fmt.Sprintf("%s: %s", event.Error.Code, event.Error.Message)
		if m.IsPending(event.RequestID) {
			recovery := m.pending.kind == protocol.ClientResumeGame
			target := m.pending.gameID
			m.clearPending()
			if recovery || event.Error.Code == protocol.ErrorStaleGameVersion {
				m.action = actionRecoveryRequired
			}
			if recovery {
				if event.Error.Code == protocol.ErrorGameNotFound || event.Error.Code == protocol.ErrorNotPlayer {
					m.gameID = ""
					m.state = nil
					m.position = nil
					m.moves = nil
					m.action = actionIdle
				} else {
					m.recoveryTarget = target
				}
			}
			if m.action == actionRecoveryRequired {
				m.message += " — Restart with the same profile ID to recover."
			}
		}
		return m, nil
	}
	if event.Matchmaking != nil {
		if event.Type == protocol.ServerMatchmakingEntered && m.IsPending(event.RequestID) && m.pending.kind == protocol.ClientEnterMatchmaking && m.pending.gameID == "" && (m.action == actionAwaiting || m.action == actionUnknown) {
			m.clearSubmittedInput()
			m.action = actionQueued
			m.message = ""
		}
		return m, nil
	}
	if event.Assignment != nil {
		if !m.IsPending(event.RequestID) {
			return m, nil
		}
		expected := map[protocol.ClientMessageType]protocol.ServerMessageType{
			protocol.ClientCreateGame:       protocol.ServerGameCreated,
			protocol.ClientJoinGame:         protocol.ServerGameJoined,
			protocol.ClientResumeGame:       protocol.ServerGameResumed,
			protocol.ClientEnterMatchmaking: protocol.ServerMatchFound,
		}
		if expected[m.pending.kind] != event.Type {
			return m, nil
		}
		assignment := event.Assignment
		if m.pending.gameID != "" && m.pending.gameID != assignment.GameID {
			return m, nil
		}
		if m.gameID != assignment.GameID {
			m.state = nil
			m.position = nil
			m.moves = nil
		}
		m.gameID, m.color = assignment.GameID, assignment.Color
		if event.Type == protocol.ServerGameCreated {
			m.acknowledge()
			m.message = ""
		} else if event.Type == protocol.ServerMatchFound {
			m.pending.gameID = assignment.GameID
			m.action = actionInitializing
			m.message = ""
		} else {
			m.clearSubmittedInput()
		}
		return m, nil
	}
	if event.State == nil || event.State.GameID != m.gameID {
		return m, nil
	}
	if m.pending.kind == protocol.ClientEnterMatchmaking && (m.pending.gameID == "" || m.state == nil && event.Type != protocol.ServerGameInitial) {
		return m, nil
	}
	state := event.State
	if m.state != nil && state.Version < m.state.Version {
		return m, nil
	}
	fen, err := chess.FEN(state.FEN)
	if err != nil {
		m.message = "Invalid server position: " + err.Error() + ". Restart with the same profile ID to recover."
		m.action = actionRecoveryRequired
		return m, nil
	}
	m.position = chess.NewGame(fen).Position()

	// Only received moves are known; reconnects and skipped versions may leave gaps.
	if m.state != nil && state.Version > m.state.Version && state.FEN != m.state.FEN && state.LastMove != "" {
		if state.Version-m.state.Version > 1 {
			m.moves = append(m.moves, "… missing updates …")
		}
		m.moves = append(m.moves, fmt.Sprintf("v%d %s", state.Version, state.LastMove))
		m.moves = m.moves[max(0, len(m.moves)-4):]
	}
	if m.action == actionIdle && (m.state == nil || state.Version > m.state.Version) {
		m.message = ""
	}
	wasFinished := m.state != nil && m.state.Status == protocol.GameStatusFinished
	m.state = state
	acknowledged := m.IsPending(event.RequestID) && (m.pending.kind == protocol.ClientMakeMove || m.pending.kind == protocol.ClientResign)
	initialTarget := m.pending.gameID == m.gameID && (m.pending.kind == protocol.ClientJoinGame || m.pending.kind == protocol.ClientResumeGame || m.pending.kind == protocol.ClientEnterMatchmaking)
	initialized := event.Type == protocol.ServerGameInitial && (initialTarget || m.recoveryTarget == m.gameID)
	finished := state.Status == protocol.GameStatusFinished && (m.pending.gameID == m.gameID || m.recoveryTarget == m.gameID || m.pending.id == "" && !wasFinished)
	if acknowledged || initialized || finished {
		m.acknowledge()
		m.message = ""
	}
	return m, nil
}

// Recovery has no submitted input and must preserve the draft.
func (m *Model) clearSubmittedInput() {
	if m.pending.id != "" && m.pending.kind != protocol.ClientResumeGame && m.input == m.pending.input {
		m.input = ""
	}
}

func (m *Model) acknowledge() {
	if m.pending.kind != protocol.ClientEnterMatchmaking {
		m.clearSubmittedInput()
	}
	m.clearPending()
}

func (m *Model) clearPending() {
	m.pending = pendingAction{}
	m.action = actionIdle
	m.recoveryTarget = ""
}
