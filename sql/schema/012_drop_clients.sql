-- +goose Up
PRAGMA foreign_keys=off;

CREATE TABLE video_series_new (
    id TEXT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    user_id TEXT NOT NULL,
    s3_url TEXT,
    audio_s3 TEXT NOT NULL DEFAULT 'unset',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

INSERT INTO video_series_new (
    id, title, description, user_id, s3_url, audio_s3, created_at, updated_at
)
SELECT
    id, title, description, user_id, s3_url, 'unset' AS audio_s3, created_at, updated_at
FROM video_series;

DROP TABLE video_series;

ALTER TABLE video_series_new RENAME TO video_series;

PRAGMA foreign_keys=on;



-- +goose Down
PRAGMA foreign_keys=off;

-- Create temp table with a placeholder client_id column
CREATE TABLE video_series_old (
    id TEXT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    user_id TEXT NOT NULL,
    client_id TEXT NOT NULL DEFAULT '',  -- add this back with a default
    s3_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO video_series_old (
    id, title, description, user_id, client_id, s3_url, created_at, updated_at
)
SELECT
    id, title, description, user_id, '' AS client_id, s3_url, created_at, updated_at
FROM video_series;

DROP TABLE video_series;

CREATE TABLE video_series (
    id TEXT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    user_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    s3_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (client_id) REFERENCES clients(id)
);

INSERT INTO video_series (
    id, title, description, user_id, client_id, s3_url, created_at, updated_at
)
SELECT
    id, title, description, user_id, client_id, s3_url, created_at, updated_at
FROM video_series_old;

DROP TABLE video_series_old;

PRAGMA foreign_keys=on;
