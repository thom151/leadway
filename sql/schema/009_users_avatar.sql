-- +goose Up
ALTER TABLE users
ADD COLUMN avatar_id TEXT NOT NULL
DEFAULT 'unset';

-- +goose Down
ALTER TABLE users
DROP COLUMN avatar_id;
