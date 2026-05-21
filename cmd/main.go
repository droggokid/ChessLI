package main

import (
	"chessli/internal/chess/board"
	"fmt"
)

func main() {
	gameBoard := board.NewBoard()
	//fmt.Println(gameBoard.Spots[models.Rank1.ToIndex()][models.FileC.ToIndex()])
	fmt.Println(gameBoard.String())
}
