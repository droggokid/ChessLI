package models

type Direction struct {
	RankDelta int
	FileDelta int
}

var (
	North = Direction{RankDelta: 1, FileDelta: 0}
	South = Direction{RankDelta: -1, FileDelta: 0}
	East  = Direction{RankDelta: 0, FileDelta: 1}
	West  = Direction{RankDelta: 0, FileDelta: -1}

	NorthEast = Direction{RankDelta: 1, FileDelta: 1}
	NorthWest = Direction{RankDelta: 1, FileDelta: -1}
	SouthEast = Direction{RankDelta: -1, FileDelta: 1}
	SouthWest = Direction{RankDelta: -1, FileDelta: -1}
)
