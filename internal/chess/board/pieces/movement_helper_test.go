package pieces

import (
	"testing"

	"chessli/internal/chess/board/models"
)

func TestCanOccupy(t *testing.T) {
	movingPiece := NewRook(models.White, models.NewPosition(models.Rank1, models.FileA))

	tests := []struct {
		name string
		spot *models.Spot
		want bool
	}{
		{name: "nil spot", want: false},
		{
			name: "empty spot",
			spot: models.NewSpot(nil, models.NewPosition(models.Rank1, models.FileB), models.White),
			want: true,
		},
		{
			name: "enemy occupied spot",
			spot: models.NewSpot(NewPawn(models.Black, models.NewPosition(models.Rank1, models.FileB)), models.NewPosition(models.Rank1, models.FileB), models.White),
			want: true,
		},
		{
			name: "friendly occupied spot",
			spot: models.NewSpot(NewPawn(models.White, models.NewPosition(models.Rank1, models.FileB)), models.NewPosition(models.Rank1, models.FileB), models.White),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canOccupy(tt.spot, movingPiece); got != tt.want {
				t.Fatalf("canOccupy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPossibleMovesFiltersOutOfBounds(t *testing.T) {
	from := models.NewPosition(models.Rank1, models.FileA)

	assertMoves(t,
		possibleMoves(from, []models.Direction{
			{RankDelta: 1, FileDelta: 2},
			{RankDelta: -1, FileDelta: 2},
			{RankDelta: 2, FileDelta: -1},
		}),
		models.NewPosition(models.Rank2, models.FileC),
	)
}

func TestWalkDirectionRequiresPieceAtStart(t *testing.T) {
	board := newTestBoard()

	moves := walkDirection(
		models.NewPosition(models.Rank1, models.FileA),
		board,
		models.North,
		canOccupy,
	)

	assertMoves(t, moves)
}

func TestCanAttack(t *testing.T) {
	if canAttack(nil, NewRook(models.White, models.NewPosition(models.Rank1, models.FileA))) {
		t.Fatal("canAttack(nil) = true, want false")
	}
}
