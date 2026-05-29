package models

// Direction is a rank/file offset used to describe piece movement.
type Direction struct {
	// RankDelta is the signed rank offset.
	RankDelta int
	// FileDelta is the signed file offset.
	FileDelta int
}

var (
	// North moves one rank toward Black's side of the board.
	North = Direction{RankDelta: 1, FileDelta: 0}
	// South moves one rank toward White's side of the board.
	South = Direction{RankDelta: -1, FileDelta: 0}
	// East moves one file toward h.
	East = Direction{RankDelta: 0, FileDelta: 1}
	// West moves one file toward a.
	West = Direction{RankDelta: 0, FileDelta: -1}

	// NorthEast moves one rank north and one file east.
	NorthEast = Direction{RankDelta: 1, FileDelta: 1}
	// NorthWest moves one rank north and one file west.
	NorthWest = Direction{RankDelta: 1, FileDelta: -1}
	// SouthEast moves one rank south and one file east.
	SouthEast = Direction{RankDelta: -1, FileDelta: 1}
	// SouthWest moves one rank south and one file west.
	SouthWest = Direction{RankDelta: -1, FileDelta: -1}
)
