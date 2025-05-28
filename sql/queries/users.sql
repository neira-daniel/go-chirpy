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
