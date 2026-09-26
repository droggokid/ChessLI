-- name: CreateProfile :one
INSERT INTO profiles (
    id, username, email, password_hash
)
VALUES (
    $1, $2, $3, $4
)
RETURNING id, username, email, created_at;

-- name: GetProfileByID :one
SELECT id, username, email, created_at
FROM profiles
WHERE id = $1;

-- name: GetProfilesByIDs :many
SELECT id, username, email, created_at
FROM profiles
WHERE id = ANY(sqlc.arg(ids)::UUID[])
ORDER BY id;

-- name: GetProfileByEmailForLogin :one
SELECT id, password_hash
FROM profiles
WHERE LOWER(email) = LOWER(sqlc.arg(email));
