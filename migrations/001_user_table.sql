CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY,
    token TEXT NOT NULL,
    config_path TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
