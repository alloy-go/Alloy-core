CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,

    password_hash TEXT NOT NULL,
    token TEXT NOT NULL,
    config_path TEXT NOT NULL,

    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now()
);
