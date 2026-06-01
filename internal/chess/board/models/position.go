package models

import "strconv"

// Rank is a board rank from 1 to 8.
type Rank int

const (
	Rank1 Rank = iota
	Rank2
	Rank3
	Rank4
	Rank5
	Rank6
	Rank7
	Rank8
)

func (r Rank) String() string {
	return strconv.Itoa(int(r) + 1)
}

// IsValid reports whether r is within the board.
func (r Rank) IsValid() bool {
	return r >= Rank1 && r <= Rank8
}

// ToIndex converts r to a zero-based board array index.
func (r Rank) ToIndex() int {
	return int(r)
}

// File is a board file from a to h.
type File rune

const (
	FileA File = 'a'
	FileB File = 'b'
	FileC File = 'c'
	FileD File = 'd'
	FileE File = 'e'
	FileF File = 'f'
	FileG File = 'g'
	FileH File = 'h'
)

func (f File) String() string {
	return string(f)
}

// IsValid reports whether f is within the board.
func (f File) IsValid() bool {
	return f >= FileA && f <= FileH
}

// ToIndex converts f to a zero-based board array index.
func (f File) ToIndex() int {
	return int(f - 'a')
}

// ToFile converts a zero-based board array index to a file.
func ToFile(index int) File {
	return File('a' + index)
}

// Position identifies a board square by rank and file.
type Position struct {
	Rank Rank `json:"rank"`
	File File `json:"file"`
}

// NewPosition creates a position from rank and file.
func NewPosition(rank Rank, file File) Position {
	return Position{Rank: rank, File: file}
}

// IsValid reports whether p is inside the board.
func (p Position) IsValid() bool {
	return p.Rank.IsValid() && p.File.IsValid()
}

func (p Position) String() string {
	return p.File.String() + p.Rank.String()
}
