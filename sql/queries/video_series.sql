-- name: CreateVideoSeriesMeta :one
INSERT INTO video_series (id, user_id, client_id, title, description )
VALUES(
    ?,
    ?,
    ?,
    ?,
    ?
)RETURNING *;

-- name: GetVideoSeriesById :one
SELECT * FROM video_series WHERE id = ?;


-- name: UpdateVideoSeries :exec
UPDATE video_series 
SET
    title = ?,
    description = ?,
    user_id = ?,
    client_id = ?,
    s3_url = ?
WHERE id = ?;


-- name: GetVideosSeriesByUserAndClient :many
SELECT * FROM video_templates WHERE user_id = ?;
