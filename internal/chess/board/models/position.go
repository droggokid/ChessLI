package models

import "strconv"

// Rank is a board rank from 1 to 8.
type Rank int

const (
	// Rank1 is the first rank.
	Rank1 Rank = iota
	// Rank2 is the second rank.
	Rank2
	// Rank3 is the third rank.
	Rank3
	// Rank4 is the fourth rank.
	Rank4
	// Rank5 is the fifth rank.
	Rank5
	// Rank6 is the sixth rank.
	Rank6
	// Rank7 is the seventh rank.
	Rank7
	// Rank8 is the eighth rank.
	Rank8
)

// String returns the algebraic rank text.
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
	// FileA is the a-file.
	FileA File = 'a'
	// FileB is the b-file.
	FileB File = 'b'
	// FileC is the c-file.
	FileC File = 'c'
	// FileD is the d-file.
	FileD File = 'd'
	// FileE is the e-file.
	FileE File = 'e'
	// FileF is the f-file.
	FileF File = 'f'
	// FileG is the g-file.
	FileG File = 'g'
	// FileH is the h-file.
	FileH File = 'h'
)

// String returns the algebraic file text.
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
	// Rank is the position's board rank.
	Rank Rank `json:"rank"`
	// File is the position's board file.
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

// String returns algebraic square text such as e4.
func (p Position) String() string {
	return p.File.String() + p.Rank.String()
}
