CREATE TABLE IF NOT EXISTS dora_metrics (
    metric_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    
    -- Deployment Frequency
    deployment_count INTEGER NOT NULL DEFAULT 0,
    deployments_per_day DECIMAL(5,2),
    
    -- Lead Time for Changes (commit to production)
    avg_lead_time_minutes INTEGER,
    median_lead_time_minutes INTEGER,
    
    -- Mean Time to Recovery
    avg_mttr_minutes INTEGER,
    incident_count INTEGER DEFAULT 0,
    
    -- Change Failure Rate
    total_deployments INTEGER NOT NULL DEFAULT 0,
    failed_deployments INTEGER NOT NULL DEFAULT 0,
    change_failure_rate DECIMAL(5,2),
    
    -- Rollback Metrics
    rollback_count INTEGER DEFAULT 0,
    rollback_rate DECIMAL(5,2),
    
    -- Time Period (rolling 30 days)
    period_start TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    period_end TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    calculated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT fk_dora_metrics_project
        FOREIGN KEY (project_id)
        REFERENCES projects(project_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_dora_metrics_project ON dora_metrics(project_id, calculated_at DESC);

