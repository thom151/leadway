-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,        
    email TEXT UNIQUE NOT NULL,           
    password_hash TEXT NOT NULL,          
    created_at TEXT NOT NULL,  
    updated_at TEXT NOT NULL, 
    avatar_url TEXT,                        
    is_admin BOOLEAN DEFAULT FALSE      
);

-- +goose Down
DROP TABLE users;
