package game

import (
	"chessli/internal/chess/board/models"
	"fmt"
)

type Player struct {
	Name  string
	Color models.Color
}

func NewPlayer(name string, color models.Color) Player {
	return Player{Name: name, Color: color}
}

func (p *Player) String() string {
	return fmt.Sprintf("%s player %s", p.Color, p.Name)
}
