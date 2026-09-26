-- +goose Up
CREATE TABLE profiles (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX profiles_username_lower_idx
    ON profiles (LOWER(username));

CREATE UNIQUE INDEX profiles_email_lower_idx
    ON profiles (LOWER(email));

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
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (white_profile_id <> black_profile_id)
);

CREATE INDEX completed_games_white_profile_id_completed_at_idx
    ON completed_games (white_profile_id, completed_at DESC);

CREATE INDEX completed_games_black_profile_id_completed_at_idx
    ON completed_games (black_profile_id, completed_at DESC);

CREATE TABLE completed_game_moves (
    game_id UUID NOT NULL REFERENCES completed_games (id) ON DELETE CASCADE,
    ply INTEGER NOT NULL CHECK (ply > 0),
    uci TEXT NOT NULL CHECK (CHAR_LENGTH(uci) BETWEEN 4 AND 5),
    PRIMARY KEY (game_id, ply)
);

-- +goose Down
DROP TABLE completed_game_moves;
DROP TABLE completed_games;
DROP TABLE profiles;
