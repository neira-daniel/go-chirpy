-- name: StoreRefreshToken :exec
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at)
VALUES (
    $1,
    now() AT TIME ZONE 'UTC',
    now() AT TIME ZONE 'UTC',
    $2,
    now() AT TIME ZONE 'UTC' + sqlc.arg(days)::integer * INTERVAL '1 day'
);

-- name: GetRefreshToken :one
SELECT *
FROM refresh_tokens
WHERE token = $1;
