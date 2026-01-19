CREATE TABLE IF NOT EXISTS deployments (
    deployment_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    project_id UUID NOT NULL,

    commit_sha TEXT NOT NULL,
    image_tag TEXT NOT NULL,
    status TEXT NOT NULL,

    secret_status TEXT,
    service_status TEXT,
    deployment_status TEXT,

    namespace TEXT,
    deployment_name TEXT,

    error_message TEXT,

    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now(),
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now(),

    failure_type TEXT,
    needs_rollback BOOLEAN DEFAULT false,

    rollback_from UUID,

    deployment_type VARCHAR(50),

    canary_stage INTEGER,
    canary_track TEXT,
    canary_target_replicas INTEGER,
    canary_analysis_window INTEGER,

    CONSTRAINT fk_deployments_project
        FOREIGN KEY (project_id)
        REFERENCES projects(project_id)
        ON DELETE CASCADE
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_deployments_rollback'
          AND table_name = 'deployments'
          AND constraint_type = 'FOREIGN KEY'
    ) THEN
        ALTER TABLE deployments
        ADD CONSTRAINT fk_deployments_rollback
        FOREIGN KEY (rollback_from)
        REFERENCES deployments(deployment_id);
    END IF;
END;
$$;
