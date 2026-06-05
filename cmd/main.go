package main

import (
	"chessli/internal/chess/board/models"
	"chessli/internal/chess/game"
	"fmt"
)

func main() {
	chessGame, err := game.NewGame("Alice", "Bob")
	if err != nil {
		panic(err)
	}

	fmt.Printf("new game: turn=%s state=%s\n", chessGame.Turn, chessGame.State)
	printLegalMoves(chessGame, models.NewPosition(models.Rank2, models.FileE))

	play(&chessGame, "Alice", models.NewPosition(models.Rank2, models.FileE), models.NewPosition(models.Rank4, models.FileE))
	play(&chessGame, "Bob", models.NewPosition(models.Rank7, models.FileE), models.NewPosition(models.Rank5, models.FileE))
	play(&chessGame, "Alice", models.NewPosition(models.Rank1, models.FileF), models.NewPosition(models.Rank4, models.FileC))
	play(&chessGame, "Bob", models.NewPosition(models.Rank8, models.FileB), models.NewPosition(models.Rank6, models.FileC))
	play(&chessGame, "Alice", models.NewPosition(models.Rank1, models.FileD), models.NewPosition(models.Rank5, models.FileH))
	play(&chessGame, "Bob", models.NewPosition(models.Rank8, models.FileG), models.NewPosition(models.Rank6, models.FileF))
	play(&chessGame, "Alice", models.NewPosition(models.Rank5, models.FileH), models.NewPosition(models.Rank7, models.FileF))

	fmt.Printf("\nfinal: turn=%s state=%s inCheck=%v draw=%v\n",
		chessGame.Turn,
		chessGame.State,
		chessGame.CurrentPlayerIsInCheck(),
		chessGame.Result.Draw,
	)
	if chessGame.Result.Winner != nil {
		fmt.Printf("winner: %s (%s)\n", chessGame.Result.Winner.Name, chessGame.Result.Winner.Color)
	}

	printLegalMoves(chessGame, models.NewPosition(models.Rank8, models.FileE))
	printCaptures(chessGame)
	printMoveHistory(chessGame)
}

func play(chessGame *game.Game, player string, from models.Position, to models.Position) {
	move := models.NewMove(from, to)
	if err := chessGame.Move(move); err != nil {
		fmt.Printf("%s %s failed: %v\n", player, move, err)
		return
	}

	fmt.Printf("%s %s -> turn=%s state=%s\n", player, move, chessGame.Turn, chessGame.State)
}

func printLegalMoves(chessGame game.Game, pos models.Position) {
	moves, err := chessGame.LegalMovesFor(pos)
	if err != nil {
		fmt.Printf("legal moves for %s failed: %v\n", pos, err)
		return
	}

	fmt.Printf("legal moves for %s: %v\n", pos, moves)
}

func printCaptures(chessGame game.Game) {
	fmt.Printf("\ncaptures by white: %d\n", len(chessGame.CapturedByWhite))
	for _, piece := range chessGame.CapturedByWhite {
		fmt.Printf("- %s %s\n", piece.Color(), piece.Type())
	}

	fmt.Printf("captures by black: %d\n", len(chessGame.CapturedByBlack))
	for _, piece := range chessGame.CapturedByBlack {
		fmt.Printf("- %s %s\n", piece.Color(), piece.Type())
	}
}

func printMoveHistory(chessGame game.Game) {
	fmt.Printf("\nmove history (%d moves):\n", len(chessGame.MoveHistory))
	for i, record := range chessGame.MoveHistory {
		capture := ""
		if record.WasCapture {
			capture = fmt.Sprintf(" captures %s", record.CapturedPiece)
		}

		fmt.Printf("%d. %s %s %s%s\n",
			i+1,
			record.MovingColor,
			record.MovingPiece,
			record.Move,
			capture,
		)
	}
}
