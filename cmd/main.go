package main

import (
	"chessli/internal/chess/board"
	"chessli/internal/chess/board/models"
	"fmt"
)

func main() {
	gameBoard := board.NewBoard()
	fmt.Println(gameBoard.Spots[models.Rank1.ToIndex()][models.FileC.ToIndex()])
	//fmt.Println(board.StringBlackView())
}
