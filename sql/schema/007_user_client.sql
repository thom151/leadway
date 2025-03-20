-- +goose Up
ALTER TABLE clients ADD COLUMN agent_id TEXT NOT NULL REFERENCES users(id);

-- +goose Down
ALTER TABLE clients DROP COLUMN agent_id;
