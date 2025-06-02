-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    now() AT TIME ZONE 'UTC',
    now() AT TIME ZONE 'UTC',
    $1,
    $2
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: ResetDatabase :exec
DELETE FROM users;

-- name: UpdateCredentials :one
UPDATE users
SET updated_at = now() AT TIME ZONE 'UTC',
    email = $1,
    hashed_password = $2
WHERE id = $3
RETURNING *;

-- name: UpgradeUser :one
UPDATE users
SET is_chirpy_red = true
WHERE id = $1
RETURNING *;
