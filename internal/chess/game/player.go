package game

import (
	"chessli/internal/chess/board/models"
)

type Player struct {
	Color models.Color
}

func NewPlayer(t int) *Player {
	return &Player{}
}

func (p *Player) String() string {
	return ""
}
