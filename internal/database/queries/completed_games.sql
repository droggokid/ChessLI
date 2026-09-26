-- name: CreateCompletedGame :exec
INSERT INTO completed_games (
    id,
    white_profile_id,
    black_profile_id,
    initial_time_ms,
    increment_ms,
    outcome,
    termination,
    final_fen,
    completed_at
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(white_profile_id),
    sqlc.arg(black_profile_id),
    sqlc.arg(initial_time_ms),
    sqlc.arg(increment_ms),
    sqlc.arg(outcome),
    sqlc.arg(termination),
    sqlc.arg(final_fen),
    sqlc.arg(completed_at)
);

-- name: CreateCompletedGameMoves :exec
INSERT INTO completed_game_moves (game_id, ply, uci)
SELECT
    sqlc.arg(game_id),
    moves.ply::INTEGER,
    moves.uci
FROM unnest(sqlc.arg(uci_moves)::TEXT[]) WITH ORDINALITY AS moves(uci, ply);

-- name: ListCompletedGamesByProfileID :many
SELECT
    id,
    white_profile_id,
    black_profile_id,
    initial_time_ms,
    increment_ms,
    outcome,
    termination,
    completed_at
FROM completed_games
WHERE white_profile_id = sqlc.arg(profile_id)
   OR black_profile_id = sqlc.arg(profile_id)
ORDER BY completed_at DESC, id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: GetCompletedGameByID :one
SELECT
    id,
    white_profile_id,
    black_profile_id,
    initial_time_ms,
    increment_ms,
    outcome,
    termination,
    final_fen,
    completed_at
FROM completed_games
WHERE id = $1;

-- name: ListCompletedGameMovesByGameID :many
SELECT ply, uci
FROM completed_game_moves
WHERE game_id = $1
ORDER BY ply;
