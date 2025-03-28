-- +goose Up
CREATE TABLE video_avatars(
    id TEXT PRIMARY KEY,
    template_type TEXT NOT NULL,
    avatar_id TEXT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    user_id TEXT NOT NULL,
    s3_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- +goose Down 
DROP TABLE video_avatars;
