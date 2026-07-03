-- Add canary metrics table
CREATE TABLE IF NOT EXISTS canary_metrics (
    canary_metric_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL,
    project_id UUID NOT NULL,
    
    -- Canary Identity
    canary_deployment_name TEXT NOT NULL,
    canary_namespace TEXT NOT NULL,
    canary_stage INTEGER NOT NULL,
    
    -- Snapshot Metadata
    snapshot_time TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    analysis_window_minutes INTEGER DEFAULT 5,
    
    -- Pod Metrics (at time of snapshot)
    total_pods INTEGER DEFAULT 0,
    ready_pods INTEGER DEFAULT 0,
    restart_count INTEGER DEFAULT 0,
    target_replicas INTEGER DEFAULT 0,
    
    -- Health Metrics
    health_score INTEGER DEFAULT 100,
    health_status TEXT DEFAULT 'unknown',
    error_rate DECIMAL(5,2) DEFAULT 0.0,
    
    -- Canary Result
    canary_result TEXT DEFAULT 'analyzing', -- 'analyzing', 'passed', 'failed', 'aborted'
    promotion_decision TEXT DEFAULT 'pending', -- 'promote', 'abort', 'pending'
    
    -- Timestamps
    started_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    
    CONSTRAINT fk_canary_metrics_deployment
        FOREIGN KEY (deployment_id)
        REFERENCES deployments(deployment_id)
        ON DELETE CASCADE,
        
    CONSTRAINT fk_canary_metrics_project
        FOREIGN KEY (project_id)
        REFERENCES projects(project_id)
        ON DELETE CASCADE,
        
    CONSTRAINT unique_deployment_canary
        UNIQUE (deployment_id)
);

CREATE INDEX IF NOT EXISTS idx_canary_metrics_deployment 
ON canary_metrics(deployment_id);

CREATE INDEX IF NOT EXISTS idx_canary_metrics_project 
ON canary_metrics(project_id, completed_at DESC);

-- Add deployment_strategy column to projects if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'projects' AND column_name = 'deployment_strategy'
    ) THEN
        ALTER TABLE projects ADD COLUMN deployment_strategy TEXT DEFAULT 'canary';
    END IF;
END $$;