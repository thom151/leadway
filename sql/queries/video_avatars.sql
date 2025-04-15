-- name: CreateAvatar :one
INSERT INTO video_avatars(id, template_type, avatar_id, title, description, user_id)
VALUES(
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)RETURNING *;

-- name: GetAvatarById :one
SELECT * FROM video_avatars WHERE id = ?;


-- name: GetAvatarsByUser :one
SELECT * FROM video_avatars WHERE user_id = ?;

-- name: GetAvatarsByUserAndTemplateType :many
SELECT * FROM video_avatars WHERE user_id = ? AND template_type = "";

-- name: UpdateVideoAvatar :exec
UPDATE video_avatars
SET s3_url = ?
WHERE id = ?;
