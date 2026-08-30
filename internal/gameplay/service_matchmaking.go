package gameplay

import "context"

// EnterMatchmaking queues a profile or matches it with a compatible opponent.
func (s *GameService) EnterMatchmaking(ctx context.Context, command EnterMatchmakingCommand) (MatchTicket, error) {
	if err := ctx.Err(); err != nil {
		return MatchTicket{}, err
	}

	timeControl := command.timeControl
	if err := validateTimeControl(timeControl.initial, timeControl.increment); err != nil {
		return MatchTicket{}, err
	}

	player := newWaitingPlayer(ctx, command)
	s.mu.Lock()
	waiting, queued, err := s.findOpponentLocked(player)
	if err != nil {
		s.mu.Unlock()
		close(player.result)
		return MatchTicket{}, err
	}

	if queued {
		s.mu.Unlock()
		go s.removeWaitingPlayerOnCancel(player)
		return MatchTicket{Result: player.result}, nil
	}

	waitingResult, currentResult, game, err := s.createMatchLocked(waiting, player)
	if err != nil {
		s.mu.Unlock()
		close(waiting.result)
		close(player.result)
		return MatchTicket{}, err
	}
	s.mu.Unlock()

	game.scheduleExpiration(s.notifyGameExpired)
	deliverMatchResult(waiting.result, waitingResult)
	deliverMatchResult(player.result, currentResult)
	return MatchTicket{Result: player.result}, nil
}
