package game

import (
	"chessli/internal/chess/board/models"
	"fmt"
)

// Player represents a named player assigned to a color.
type Player struct {
	// Name is the player's display name.
	Name string
	// Color is the side assigned to the player.
	Color models.Color
}

// NewPlayer creates a player with a name and color.
func NewPlayer(name string, color models.Color) Player {
	return Player{Name: name, Color: color}
}

// String returns a human-readable player description.
func (p *Player) String() string {
	return fmt.Sprintf("%s player %s", p.Color, p.Name)
}
