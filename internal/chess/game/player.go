package game

import (
	"chessli/internal/chess/board/models"
	"fmt"
)

// Player represents a named player assigned to a color.
type Player struct {
	Name  string
	Color models.Color
}

// NewPlayer creates a player with a name and color.
func NewPlayer(name string, color models.Color) Player {
	return Player{Name: name, Color: color}
}

func (p *Player) String() string {
	return fmt.Sprintf("%s player %s", p.Color, p.Name)
}
