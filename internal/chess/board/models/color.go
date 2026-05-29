package models

// Color identifies a chess side or square color.
type Color string

const (
	// White is the white side.
	White Color = "white"
	// Black is the black side.
	Black Color = "black"
)

// String returns the color text.
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
