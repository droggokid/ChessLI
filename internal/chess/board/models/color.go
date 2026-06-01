package models

// Color identifies a chess side or square color.
type Color string

const (
	White Color = "white"
	Black Color = "black"
)

func (c Color) String() string {
	return string(c)
}

// Flip returns the opposite color.
func (c Color) Flip() Color {
	if c == White {
		return Black
	}
	return White
}
