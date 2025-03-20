-- name: CreateVideoMeta :one
INSERT INTO video_templates (id, user_id, title, description )
VALUES(
    ?,
    ?,
    ?,
    ?
)RETURNING *;

-- name: GetVideoById :one
SELECT * FROM video_templates WHERE id = ?;


-- name: UpdateVideo :exec
UPDATE video_templates 
SET
    title = ?,
    description = ?,
    user_id = ?,
    s3_url = ?
WHERE id = ?;


-- name: GetVideosByUser :many
SELECT * FROM video_templates WHERE user_id = ?;
