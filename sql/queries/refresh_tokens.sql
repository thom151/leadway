-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at) 
VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)
RETURNING *;


-- name: GetToken :one
SELECT * FROM refresh_tokens WHERE token = ?;

-- name: RevokeToken :exec
UPDATE refresh_tokens SET revoked_at = ?, updated_at = ? where token = ?;
