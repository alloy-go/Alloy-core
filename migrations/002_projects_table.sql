CREATE TABLE IF NOT EXISTS projects (
    project_id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    project_name TEXT NOT NULL,
    deployment_type TEXT NOT NULL,
    context_name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, project_name)
);
