# Code Review Findings

## Verification

Command run:

```sh
GOCACHE=/tmp/chessli-go-cache go test ./...
```

Result: passing build. All packages compile, but there are currently no test files.

## Findings

### High: `MoveTo` does not update the piece position

`internal/chess/board/models/piece.go:35` defines:

```go
func (p BasePiece) MoveTo(position Position)
```

Because this method has a value receiver, it mutates a copy of `BasePiece`, not the piece stored on the board. `Game.Move` calls `movingPiece.MoveTo(toSpot.Position)` at `internal/chess/game/game.go:109`, but the piece's stored position remains unchanged.

This breaks the next turn calculation because `CalculateAllLegalMoves` uses `piece.Position()` as the cache key at `internal/chess/game/game.go:45` and `internal/chess/game/game.go:49`.

Recommended fix: make `MoveTo` use a pointer receiver on `BasePiece`.

### High: `King`, `Knight`, and `Pawn` cannot move yet

These pieces now satisfy the `models.Piece` interface, but their move generators still return nil:

- `internal/chess/board/pieces/king.go:24`
- `internal/chess/board/pieces/knight.go:24`
- `internal/chess/board/pieces/pawn.go:24`

Since `Game.Move` validates against cached legal moves, these pieces currently have no legal moves and cannot be moved.

### Medium: `LegalMovesFor` can cache moves outside the current turn

`LegalMovesFor` is exported and accepts any board position. It does not check whether the piece at that position belongs to `g.Turn` before calculating and storing moves in `g.legalMoves`.

Current `Move` still rejects opponent pieces later, but the cache can contain entries that do not belong to the active player. Consider making `LegalMovesFor` enforce turn ownership, or splitting it into a public turn-aware method and a private calculation helper.

### Medium: `Move` reports invalid sources as illegal moves first

`Game.Move` checks `containsMove` before reading the board:

```go
if !g.containsMove(from, to)
```

This means an invalid position or empty source returns `"illegal move"` instead of `"position outside board"` or `"no piece at source position"`. That is not a correctness failure, but it makes debugging and UI feedback less precise.

### Medium: Current move generation is pseudo-legal only

Bishop, rook, and queen movement account for blockers and captures through `walkDirection`, but game-level legality is not complete yet. The engine does not filter moves that leave the king in check, detect checkmate/stalemate, or handle castling, en passant, and promotion.

Until those rules exist, cached moves should be treated as pseudo-legal moves rather than full chess-legal moves.

### Low: Starter piece helpers are setup-only

`WhiteStarterPieces` and `BlackStarterPieces` read fixed starting ranks in `internal/chess/board/board.go:106` and `internal/chess/board/board.go:119`. That is fine for initializing `Game`, but these methods should not be reused later as "all current pieces" queries after moves have happened.

## Resolved Since Last Review

- The project now compiles with `go test ./...`.
- `King`, `Knight`, and `Pawn` now match the `LegalMoves(from, board)` interface.
- `walkDirection` now converts file indexes through `models.ToFile`.
- `Game.Move` now assigns the flipped turn through `prepareNextTurn`.
- `containsMove` now checks whether `to` appears in `g.legalMoves[from]`.
- Black pawns are now created with `models.Black`.

## Suggested Next Steps

1. Fix `BasePiece.MoveTo` so moved pieces actually update their stored position.
2. Add tests around `Move` to prove turn preparation uses the new piece position.
3. Implement `Knight` legal moves first, then `King`, then `Pawn`.
4. Decide whether `LegalMovesFor` should reject non-current-turn pieces.
5. Add tests for captures, turn switching, sliding-piece blockers, and invalid move errors.
