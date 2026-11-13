-- +goose Up
CREATE TABLE profiles (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX profiles_username_lower_idx
    ON profiles (LOWER(username));

CREATE UNIQUE INDEX profiles_email_lower_idx
    ON profiles (LOWER(email));

-- Keep move validation with the immutable completed-game record. Guard shapes
-- before casting so malformed histories fail the CHECK constraint.
-- +goose StatementBegin
CREATE FUNCTION completed_game_moves_valid(moves JSONB)
RETURNS BOOLEAN
LANGUAGE SQL
IMMUTABLE
AS $$
    SELECT CASE
        WHEN jsonb_typeof(moves) IS DISTINCT FROM 'array' THEN FALSE
        ELSE NOT EXISTS (
            SELECT 1
            FROM jsonb_array_elements(moves) WITH ORDINALITY AS history(move, ply)
            WHERE CASE
                WHEN jsonb_typeof(move) IS DISTINCT FROM 'object'
                    OR jsonb_typeof(move -> 'ply') IS DISTINCT FROM 'number'
                    OR jsonb_typeof(move -> 'uci') IS DISTINCT FROM 'string'
                    OR jsonb_typeof(move -> 'white_remaining_ms') IS DISTINCT FROM 'number'
                    OR jsonb_typeof(move -> 'black_remaining_ms') IS DISTINCT FROM 'number'
                THEN TRUE
                ELSE (move ->> 'ply')::NUMERIC <> ply
                    OR CHAR_LENGTH(move ->> 'uci') NOT BETWEEN 4 AND 5
                    OR (move ->> 'white_remaining_ms')::NUMERIC NOT BETWEEN 0 AND 9223372036854775807
                    OR (move ->> 'black_remaining_ms')::NUMERIC NOT BETWEEN 0 AND 9223372036854775807
                    OR TRUNC((move ->> 'white_remaining_ms')::NUMERIC) <> (move ->> 'white_remaining_ms')::NUMERIC
                    OR TRUNC((move ->> 'black_remaining_ms')::NUMERIC) <> (move ->> 'black_remaining_ms')::NUMERIC
            END
        )
    END;
$$;
-- +goose StatementEnd

CREATE TABLE completed_games (
    id UUID PRIMARY KEY,
    white_profile_id UUID NOT NULL REFERENCES profiles (id),
    black_profile_id UUID NOT NULL REFERENCES profiles (id),
    initial_time_ms BIGINT NOT NULL CHECK (initial_time_ms > 0),
    increment_ms BIGINT NOT NULL CHECK (increment_ms >= 0),
    outcome TEXT NOT NULL CHECK (outcome IN ('white_win', 'black_win', 'draw')),
    termination TEXT NOT NULL CHECK (termination IN (
        'checkmate',
        'stalemate',
        'resignation',
        'timeout',
        'draw_agreement',
        'threefold_repetition',
        'fivefold_repetition',
        'fifty_move_rule',
        'seventy_five_move_rule',
        'insufficient_material'
    )),
    final_fen TEXT NOT NULL,
    white_remaining_ms BIGINT NOT NULL CHECK (white_remaining_ms BETWEEN 0 AND 9223372036854),
    black_remaining_ms BIGINT NOT NULL CHECK (black_remaining_ms BETWEEN 0 AND 9223372036854),
    moves JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (completed_game_moves_valid(moves)),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (white_profile_id <> black_profile_id)
);

CREATE INDEX completed_games_white_profile_id_completed_at_idx
    ON completed_games (white_profile_id, completed_at DESC);

CREATE INDEX completed_games_black_profile_id_completed_at_idx
    ON completed_games (black_profile_id, completed_at DESC);

-- +goose Down
DROP TABLE completed_games;
DROP FUNCTION completed_game_moves_valid(JSONB);
DROP TABLE profiles;
