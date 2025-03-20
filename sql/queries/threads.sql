-- name: CreateThread :one
INSERT INTO threads (user_id, contact_id, thread_id)
VALUES (
    ?,
    ?,
    ?
) RETURNING *;

-- name: GetThread :one
SELECT * FROM threads WHERE user_id = ? AND contact_id = ?;
