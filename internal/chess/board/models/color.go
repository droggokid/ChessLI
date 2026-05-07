package models

type Color string

const (
	White Color = "white"
	Black Color = "black"
)

func (c Color) String() string {
	return string(c)
}

func (c Color) Flip() Color {
	if c == White {
		return Black
	}
	return White
}
