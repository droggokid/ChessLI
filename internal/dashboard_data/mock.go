package dashboard_data

import (
	"context"
	"time"

	domain "ChessLI/internal/game"
)

type Statistics struct {
	Wins   int
	Draws  int
	Losses int
}

type RecentGame struct {
	Opponent    string
	Result      string
	CompletedAt time.Time
	Termination domain.Termination
}

type Overview struct {
	Username   string
	Statistics Statistics
	Recent     []RecentGame
}

type Mock struct{}

func (Mock) Load(ctx context.Context) (Overview, error) {
	if err := ctx.Err(); err != nil {
		return Overview{}, err
	}

	return Overview{
		Username: "Local Player (sample data)",
		Statistics: Statistics{
			Wins:   2,
			Losses: 1,
		},
		Recent: []RecentGame{
			{
				Opponent:    "Alex",
				Result:      "Win",
				CompletedAt: time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC),
				Termination: domain.TerminationCheckmate,
			},
			{
				Opponent:    "Sam",
				Result:      "Loss",
				CompletedAt: time.Date(2026, 10, 8, 19, 0, 0, 0, time.UTC),
				Termination: domain.TerminationResignation,
			},
			{
				Opponent:    "Jordan",
				Result:      "Win",
				CompletedAt: time.Date(2026, 10, 7, 17, 0, 0, 0, time.UTC),
				Termination: domain.TerminationResignation,
			},
		},
	}, nil
}
