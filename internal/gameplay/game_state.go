package gameplay

import (
	"fmt"
	"log/slog"
	"time"

	domain "ChessLI/internal/game"
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

// gameState holds synchronous game rules. Its owning game must hold mu for access.
type gameState struct {
	id             identity.GameID
	version        uint64
	whiteProfileID identity.ProfileID
	blackProfileID identity.ProfileID
	engine         *chess.Game
	lastMoveSAN    string
	clock          gameClock
	outcome        domain.Outcome
	termination    domain.Termination
	drawOffers     drawOfferState
}

// newGameState returns state with the standard starting position and the supplied
// players and time control.
func newGameState(id identity.GameID, white identity.ProfileID, black identity.ProfileID, initialTime time.Duration, increment time.Duration, now time.Time) gameState {
	g := gameState{
		id:             id,
		whiteProfileID: white,
		blackProfileID: black,
		engine:         chess.NewGame(),
		clock:          newGameClock(initialTime, increment),
		drawOffers: drawOfferState{
			lastOfferedAt: make(map[identity.ProfileID]time.Time, 2),
		},
	}

	if white != "" && black != "" {
		g.clock.start(now)
	}

	return g
}

// move decodes, validates, and applies a move for the profile whose turn it is.
func (g *gameState) move(command MoveCommand, now time.Time) (GameSnapshot, error) {
	if g.expire(now) {
		return g.snapshot(now), nil
	}

	if command.ExpectedVersion != g.version {
		return GameSnapshot{}, ErrStaleGameVersion
	}

	if err := g.validateMove(command.ProfileID); err != nil {
		return GameSnapshot{}, err
	}

	position := g.engine.Position()

	notation, err := engineNotation(command.Notation)
	if err != nil {
		return GameSnapshot{}, err
	}

	move, err := notation.Decode(position, command.Move)
	if err != nil {
		return GameSnapshot{}, fmt.Errorf("%w: %v", ErrIllegalMove, err)
	}

	san := chess.AlgebraicNotation{}.Encode(position, move)
	mover := position.Turn()

	if err = g.engine.Move(move, nil); err != nil {
		return GameSnapshot{}, ErrIllegalMove
	}

	g.clock.completeMove(now, mover)
	g.syncOutcomeFromEngine()

	g.version++
	g.lastMoveSAN = san
	g.drawOffers.pending = nil
	if g.outcome != domain.OutcomeNone {
		g.clock.stop(now, g.engine.Position().Turn())
		g.logFinished()
	}

	return g.snapshot(now), nil
}

func (g *gameState) validateMove(source identity.ProfileID) error {
	if err := g.validateReady(); err != nil {
		return err
	}

	if g.outcome != domain.OutcomeNone {
		return ErrGameFinished
	}
	color, err := colorFromProfileID(source, g.whiteProfileID, g.blackProfileID)
	if err != nil {
		return err
	}
	if color != g.engine.Position().Turn() {
		return ErrNotYourTurn
	}

	return nil
}

func (g *gameState) validateReady() error {
	if g.whiteProfileID == "" || g.blackProfileID == "" {
		return ErrGameNotReady
	}

	return nil
}

// joinPrivate assigns a profile to the unoccupied color in a private game.
func (g *gameState) joinPrivate(command JoinPrivateCommand, now time.Time) (chess.Color, error) {
	switch {
	case g.whiteProfileID == command.ProfileID || g.blackProfileID == command.ProfileID:
		return chess.NoColor, ErrAlreadyParticipant

	case g.whiteProfileID == "":
		g.whiteProfileID = command.ProfileID
		g.startClockIfReady(now)
		return chess.White, nil

	case g.blackProfileID == "":
		g.blackProfileID = command.ProfileID
		g.startClockIfReady(now)
		return chess.Black, nil

	default:
		return chess.NoColor, ErrGameFull
	}
}

func (g *gameState) startClockIfReady(now time.Time) {
	if g.whiteProfileID != "" && g.blackProfileID != "" {
		g.clock.start(now)
		slog.Info("game started", "event", "game.started", "game_id", g.id, "mode", "private",
			"white_profile_id", g.whiteProfileID, "black_profile_id", g.blackProfileID,
			"initial_ms", g.clock.whiteRemaining.Milliseconds(), "increment_ms", g.clock.increment.Milliseconds())
	}
}

// resign resigns a game participant.
func (g *gameState) resign(command ResignCommand, now time.Time) (GameSnapshot, error) {
	if err := g.validateReady(); err != nil {
		return GameSnapshot{}, err
	}
	if g.expire(now) || g.outcome != domain.OutcomeNone {
		return GameSnapshot{}, ErrGameFinished
	}

	color, err := colorFromProfileID(command.ProfileID, g.whiteProfileID, g.blackProfileID)
	if err != nil {
		return GameSnapshot{}, err
	}

	g.engine.Resign(color)
	g.syncOutcomeFromEngine()
	g.version++
	g.clock.stop(now, g.engine.Position().Turn())
	g.drawOffers.pending = nil
	g.logFinished()

	return g.snapshot(now), nil
}

func (g *gameState) resume(profileID identity.ProfileID, now time.Time) (GameSnapshot, error) {
	g.expire(now)

	if _, err := colorFromProfileID(profileID, g.whiteProfileID, g.blackProfileID); err != nil {
		return GameSnapshot{}, err
	}

	return g.snapshot(now), nil
}

// offerDraw makes a draw offer for a game participant.
func (g *gameState) offerDraw(command OfferDrawCommand, now time.Time) (GameSnapshot, error) {
	if err := g.validateDrawParticipant(command.ProfileID, now); err != nil {
		return GameSnapshot{}, err
	}
	if err := g.drawOffers.offer(command.ProfileID, now); err != nil {
		return GameSnapshot{}, err
	}

	return g.snapshot(now), nil
}

// acceptDraw accepts a pending draw offer.
func (g *gameState) acceptDraw(command DrawOfferResponseCommand, now time.Time) (GameSnapshot, error) {
	if err := g.validateDrawParticipant(command.ProfileID, now); err != nil {
		return GameSnapshot{}, err
	}
	if err := g.drawOffers.validateResponse(command); err != nil {
		return GameSnapshot{}, err
	}
	if err := g.engine.Draw(chess.DrawOffer); err != nil {
		return GameSnapshot{}, err
	}

	g.syncOutcomeFromEngine()
	g.clock.stop(now, g.engine.Position().Turn())
	g.version++
	g.drawOffers.pending = nil
	g.logFinished()

	return g.snapshot(now), nil
}

// declineDraw declines a pending draw offer.
func (g *gameState) declineDraw(command DrawOfferResponseCommand, now time.Time) (GameSnapshot, error) {
	if err := g.validateDrawParticipant(command.ProfileID, now); err != nil {
		return GameSnapshot{}, err
	}
	if err := g.drawOffers.validateResponse(command); err != nil {
		return GameSnapshot{}, err
	}

	g.drawOffers.pending = nil
	return g.snapshot(now), nil
}

func (g *gameState) validateDrawParticipant(profileID identity.ProfileID, now time.Time) error {
	if err := g.validateReady(); err != nil {
		return err
	}
	if g.expire(now) || g.outcome != domain.OutcomeNone {
		return ErrGameFinished
	}
	if profileID != g.whiteProfileID && profileID != g.blackProfileID {
		return ErrNotParticipant
	}
	return nil
}

func (g *gameState) syncOutcomeFromEngine() {
	g.outcome = outcomeFromEngine(g.engine.Outcome())
	g.termination = terminationReason(g.engine.Method())
}

func (g *gameState) expire(now time.Time) bool {
	active := g.engine.Position().Turn()

	loser, expired := g.clock.expire(now, active)
	if !expired {
		return false
	}

	winner := loser.Other()
	if !g.hasMatingMaterial(winner) {
		g.outcome = domain.OutcomeDraw
	} else if loser == chess.White {
		g.outcome = domain.OutcomeBlackWin
	} else {
		g.outcome = domain.OutcomeWhiteWin
	}

	g.termination = domain.TerminationTimeout
	g.version++
	g.drawOffers.pending = nil
	g.logFinished()

	return true
}

// Log the transition itself so repeated snapshots and broadcasts cannot count it twice.
func (g *gameState) logFinished() {
	slog.Info("game finished", "event", "game.finished", "game_id", g.id,
		"outcome", g.outcome, "termination", g.termination, "version", g.version)
}

// hasMatingMaterial reports the minimum FIDE requirement for a timeout
// win: the player who still has time must have some piece capable of
// participating in checkmate. A bare king can never give checkmate.
func (g *gameState) hasMatingMaterial(color chess.Color) bool {
	for _, piece := range g.engine.Position().Board().SquareMap() {
		if piece.Color() == color && piece.Type() != chess.King {
			return true
		}
	}

	return false
}

func (g *gameState) snapshot(now time.Time) GameSnapshot {
	whiteRemaining, blackRemaining := g.clock.remaining(now, g.engine.Position().Turn())
	var pendingDrawOffer *DrawOffer
	if g.drawOffers.pending != nil {
		// Keep callers from mutating the game's pending offer through the snapshot.
		pendingDrawOffer = new(*g.drawOffers.pending)
	}

	return GameSnapshot{
		GameID:           g.id,
		FEN:              g.engine.FEN(),
		Version:          g.version,
		WhiteProfileID:   g.whiteProfileID,
		BlackProfileID:   g.blackProfileID,
		WhiteRemaining:   whiteRemaining,
		BlackRemaining:   blackRemaining,
		LastMoveSAN:      g.lastMoveSAN,
		Outcome:          g.outcome,
		Termination:      g.termination,
		PendingDrawOffer: pendingDrawOffer,
	}
}
