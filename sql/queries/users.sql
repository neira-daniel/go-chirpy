-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES (
    gen_random_uuid(),
    now() AT TIME ZONE 'UTC',
    now() AT TIME ZONE 'UTC',
    $1
)
RETURNING *;

-- name: ResetDatabase :exec
DELETE FROM users;
