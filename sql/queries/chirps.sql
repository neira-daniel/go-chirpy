-- name: SaveChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(),
    now() AT TIME ZONE 'UTC',
    now() AT TIME ZONE 'UTC',
    $1,
    $2
)
RETURNING *;

-- name: GetChirps :many
SELECT *
FROM chirps
ORDER BY created_at ASC;

-- name: GetChirpByID :one
SELECT *
FROM chirps
WHERE id = $1;

-- name: GetChirpsByAuthor :many
SELECT *
FROM chirps
WHERE user_id = $1;

-- name: DeleteChirpByID :exec
DELETE FROM chirps
WHERE id = $1;
