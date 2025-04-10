-- name: CreateVideoSeriesMeta :one
INSERT INTO video_series (id, user_id, title, description )
VALUES(
    ?,
    ?,
    ?,
    ?
)RETURNING *;

-- name: GetVideoSeriesById :one
SELECT * FROM video_series WHERE id = ?;

-- name: SetAudioUrl :one
UPDATE video_series SET audio_s3 = ?
WHERE id = ?
RETURNING *;

-- name: SetFIFUrl :one
UPDATE video_series SET s3_url= ?
WHERE id = ?
RETURNING *;

-- name: UpdateVideoSeries :exec
UPDATE video_series 
SET
    title = ?,
    description = ?,
    user_id = ?,
    s3_url = ?
WHERE id = ?;


-- name: GetVideosSeriesByUserAndClient :many
SELECT * FROM video_templates WHERE user_id = ?;
