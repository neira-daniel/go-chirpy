// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: refresh_tokens.sql

package database

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

const getRefreshToken = `-- name: GetRefreshToken :one
SELECT token, created_at, updated_at, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token = $1
`

func (q *Queries) GetRefreshToken(ctx context.Context, token string) (RefreshToken, error) {
	row := q.db.QueryRowContext(ctx, getRefreshToken, token)
	var i RefreshToken
	err := row.Scan(
		&i.Token,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.UserID,
		&i.ExpiresAt,
		&i.RevokedAt,
	)
	return i, err
}

const revokeAccess = `-- name: RevokeAccess :execresult
UPDATE refresh_tokens
SET updated_at = now() AT TIME ZONE 'UTC',
    revoked_at = now() AT TIME ZONE 'UTC'
WHERE token = $1
`

func (q *Queries) RevokeAccess(ctx context.Context, token string) (sql.Result, error) {
	return q.db.ExecContext(ctx, revokeAccess, token)
}

const storeRefreshToken = `-- name: StoreRefreshToken :exec
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at)
VALUES (
    $1,
    now() AT TIME ZONE 'UTC',
    now() AT TIME ZONE 'UTC',
    $2,
    now() AT TIME ZONE 'UTC' + $3::integer * INTERVAL '1 day'
)
`

type StoreRefreshTokenParams struct {
	Token  string
	UserID uuid.UUID
	Days   int32
}

func (q *Queries) StoreRefreshToken(ctx context.Context, arg StoreRefreshTokenParams) error {
	_, err := q.db.ExecContext(ctx, storeRefreshToken, arg.Token, arg.UserID, arg.Days)
	return err
}
