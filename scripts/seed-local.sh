#!/bin/sh
set -eu

if [ ! -f .env ]; then
    echo '.env is required; copy .env.example to .env first' >&2
    exit 1
fi

set -a
. ./.env
set +a

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;

INSERT INTO profiles (id, username, email, password_hash)
VALUES
    ('00000000-0000-4000-8000-000000000001', 'countess_ada', 'ada@example.test', '$argon2id$v=19$m=19456,t=2,p=1$Y3VyaWUtbG9jYWwtc2VlZA$2ZDY/1TWVGNkdXkYrPQ4RdvI92G0ez7EiwmOThTx1gE'),
    ('00000000-0000-4000-8000-000000000002', 'graceful_panic', 'grace@example.test', '$argon2id$v=19$m=19456,t=2,p=1$bGFtYXJyLWxvY2FsLXNlZWQ$S5OijM+AZ0PLpz+27eqX3UqHUC/8u/q5kWEedJ16rzs'),
    ('00000000-0000-4000-8000-000000000003', 'linus_torvalds_the_floor', 'linus@example.test', '$argon2id$v=19$m=19456,t=2,p=1$aG9wcGVyLWxvY2FsLXNlZWQ$5sRDEBcTJ4hkJUU5TwhVRd7eZty9xptQAIgQjPKVMd4')
ON CONFLICT (id) DO NOTHING;

INSERT INTO completed_games (
    id, white_profile_id, black_profile_id, initial_time_ms, increment_ms,
    outcome, termination, final_fen, white_remaining_ms, black_remaining_ms, moves
)
VALUES
    (
        '10000000-0000-4000-8000-000000000001',
        '00000000-0000-4000-8000-000000000001',
        '00000000-0000-4000-8000-000000000002',
        300000, 2000, 'black_win', 'checkmate',
        'rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3',
        286868, 274834,
        '[
            {"ply":1,"uci":"f2f3","white_remaining_ms":298790,"black_remaining_ms":300000},
            {"ply":2,"uci":"e7e5","white_remaining_ms":298790,"black_remaining_ms":294234},
            {"ply":3,"uci":"g2g4","white_remaining_ms":286868,"black_remaining_ms":294234},
            {"ply":4,"uci":"d8h4","white_remaining_ms":286868,"black_remaining_ms":274834}
        ]'::jsonb
    ),
    (
        '10000000-0000-4000-8000-000000000002',
        '00000000-0000-4000-8000-000000000002',
        '00000000-0000-4000-8000-000000000003',
        600000, 0, 'draw', 'draw_agreement',
        'rnbqkbnr/ppp1pppp/8/3p4/2PP4/8/PP2PPPP/RNBQKBNR b KQkq - 0 2',
        583333, 590222,
        '[
            {"ply":1,"uci":"d2d4","white_remaining_ms":592111,"black_remaining_ms":600000},
            {"ply":2,"uci":"d7d5","white_remaining_ms":592111,"black_remaining_ms":590222},
            {"ply":3,"uci":"c2c4","white_remaining_ms":583333,"black_remaining_ms":590222}
        ]'::jsonb
    )
ON CONFLICT (id) DO NOTHING;

COMMIT;
SQL
