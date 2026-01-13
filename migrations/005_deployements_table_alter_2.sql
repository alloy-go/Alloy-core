DO $$
BEGIN
    -- failure_type
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'deployments' AND column_name = 'failure_type'
    ) THEN
        ALTER TABLE deployments
        ADD COLUMN failure_type TEXT;
    END IF;

    -- needs_rollback
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'deployments' AND column_name = 'needs_rollback'
    ) THEN
        ALTER TABLE deployments
        ADD COLUMN needs_rollback BOOLEAN NOT NULL DEFAULT FALSE;
    END IF;

    -- canary_stage
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'deployments' AND column_name = 'canary_stage'
    ) THEN
        ALTER TABLE deployments
        ADD COLUMN canary_stage INTEGER NOT NULL DEFAULT 0;
    END IF;

    -- canary_track
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'deployments' AND column_name = 'canary_track'
    ) THEN
        ALTER TABLE deployments
        ADD COLUMN canary_track TEXT NOT NULL DEFAULT 'stable';
    END IF;

    -- canary_target_replicas
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'deployments' AND column_name = 'canary_target_replicas'
    ) THEN
        ALTER TABLE deployments
        ADD COLUMN canary_target_replicas INTEGER;
    END IF;

    -- canary_analysis_window
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'deployments' AND column_name = 'canary_analysis_window'
    ) THEN
        ALTER TABLE deployments
        ADD COLUMN canary_analysis_window INTEGER;
    END IF;
END $$;
