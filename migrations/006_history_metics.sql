CREATE TABLE IF NOT EXISTS metrics_history (
    history_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    deployment_id UUID,
    
    metric_type TEXT NOT NULL, -- 'kubernetes', 'dora'
    metric_data JSONB NOT NULL,
    
    collected_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT fk_metrics_history_project
        FOREIGN KEY (project_id)
        REFERENCES projects(project_id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_metrics_history_project ON metrics_history(project_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_history_type ON metrics_history(metric_type, collected_at DESC);

-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================
CREATE OR REPLACE FUNCTION calculate_health_score(
    p_ready_pods INTEGER,
    p_total_pods INTEGER,
    p_restart_count INTEGER
) RETURNS INTEGER AS $$
DECLARE
    score INTEGER := 100;
BEGIN
    -- Pod availability
    IF p_total_pods > 0 THEN
        IF p_ready_pods = 0 THEN
            score := score - 60;
        ELSIF p_ready_pods < p_total_pods THEN
            score := score - 25;
        END IF;
    END IF;

    -- Restart impact
    IF p_restart_count IS NOT NULL THEN
        IF p_restart_count > 10 THEN
            score := score - 20;
        ELSIF p_restart_count > 5 THEN
            score := score - 10;
        END IF;
    END IF;

    RETURN GREATEST(0, LEAST(100, score));
END;
$$ LANGUAGE plpgsql;

-- Determine health status from score
CREATE OR REPLACE FUNCTION get_health_status(score INTEGER) RETURNS TEXT AS $$
BEGIN
    IF score >= 80 THEN RETURN 'healthy';
    ELSIF score >= 50 THEN RETURN 'degraded';
    ELSE RETURN 'critical';
    END IF;
END;
$$ LANGUAGE plpgsql;
