-- name: CreateUser :one
INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserAvatarId :one
SELECT * FROM users WHERE avatar_id = ?;

-- name: UpdateUserAvatarId :one
UPDATE users SET avatar_id = ? WHERE id = ?
RETURNING *;
