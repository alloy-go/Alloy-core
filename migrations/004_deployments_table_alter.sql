-- Add rollback_from column only if it doesn't exist
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'deployments' AND column_name = 'rollback_from'
    ) THEN
        ALTER TABLE deployments 
        ADD COLUMN rollback_from UUID REFERENCES deployments(deployment_id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'deployments' AND column_name = 'deployment_type'
    ) THEN
        ALTER TABLE deployments
        ADD COLUMN deployment_type VARCHAR(20) DEFAULT 'deploy';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_deployments_rollback ON deployments(rollback_from);

CREATE INDEX IF NOT EXISTS idx_deployments_project_status ON deployments(project_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_deployments_type ON deployments(deployment_type);

DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.constraint_column_usage 
        WHERE constraint_name = 'check_deployment_type'
    ) THEN
        ALTER TABLE deployments
        ADD CONSTRAINT check_deployment_type CHECK (deployment_type IN ('deploy', 'rollback'));
    END IF;
END $$;