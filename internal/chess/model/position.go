package model

import "strconv"

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

func (r Rank) ToIndex() int {
	return int(r)
}

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

func (f File) ToIndex() int {
	return int(f - 'a')
}

func ToFile(index int) File {
	return File('a' + index)
}

type Position struct {
	Rank Rank `json:"rank"`
	File File `json:"file"`
}

func NewPosition(rank Rank, file File) Position {
	return Position{Rank: rank, File: file}
}

func (p Position) String() string {
	return p.File.String() + p.Rank.String()
}
