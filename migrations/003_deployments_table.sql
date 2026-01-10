-- New table for deployment tracking
CREATE TABLE IF NOT EXISTS deployments (
    deployment_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    commit_sha TEXT NOT NULL,
    image_tag TEXT NOT NULL,
    status TEXT NOT NULL, -- pending, deploying, ready, failed
    secret_status TEXT,
    service_status TEXT,
    deployment_status TEXT,
    namespace TEXT,
    deployment_name TEXT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
