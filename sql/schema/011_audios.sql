-- +goose Up
ALTER TABLE video_series
ADD COLUMN audio_s3 TEXT NOT NULL
DEFAULT 'unset';

-- +goose Down
ALTER TABLE video_series
DROP COLUMN audio_s3;
