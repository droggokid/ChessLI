package main

import (
	"chessli/internal/chess/model"
	"fmt"
)

func main() {
	board := model.NewBoard()
	fmt.Println(board)
	fmt.Println(board.StringBlackView())
}
