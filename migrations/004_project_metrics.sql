CREATE TABLE IF NOT EXISTS project_metrics (
    metric_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE,
    
    -- Latest Deployment Reference
    latest_deployment_id UUID,
    latest_image_tag TEXT,
    latest_commit_sha TEXT,
    latest_status TEXT,
    deployed_at TIMESTAMP WITHOUT TIME ZONE,
    
    -- Kubernetes Metrics (Current State)
    total_pods INTEGER DEFAULT 0,
    ready_pods INTEGER DEFAULT 0,
    restart_count INTEGER DEFAULT 0,
    replicas_desired INTEGER DEFAULT 0,
    replicas_available INTEGER DEFAULT 0,
    cpu_usage_cores DECIMAL(10,4),
    memory_usage_mb DECIMAL(10,2),
    
    -- Health
    health_score INTEGER DEFAULT 100,
    health_status TEXT DEFAULT 'healthy',
    
    -- Timestamps
    last_updated TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT fk_project_metrics_project
        FOREIGN KEY (project_id)
        REFERENCES projects(project_id)
        ON DELETE CASCADE,
        
    CONSTRAINT fk_project_metrics_deployment
        FOREIGN KEY (latest_deployment_id)
        REFERENCES deployments(deployment_id)
        ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_project_metrics_project
ON project_metrics(project_id);

CREATE INDEX IF NOT EXISTS idx_project_metrics_updated
ON project_metrics(last_updated DESC);
