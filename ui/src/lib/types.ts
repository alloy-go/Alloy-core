export type ProjectStatus = 'healthy' | 'unhealthy' | 'processing' | 'suspended' | 'unknown';
export type DeploymentType = 'rollout' | 'canary';

export interface Project {
  project_id?: string;                    // Optional if not returned
  project_name: string;
  context_name: string;
  latest_status: string;                  
  latest_commit_sha?: string;             
  latest_image_tag?: string;              
  latest_namespace?: string;              
  latest_deployment_name: string;
  latest_updated_at: string;              
  latest_error_message?: string;        
  latest_deployment_type: DeploymentType;
  
  total_deployments: number;
  ready_count: number;
  failed_count: number;
  processing_count: number;
  canary_active: boolean;
  
  created_at: string;                
}

export interface ProjectsResponse {
  projects: Project[];
  count?: number;                         
}

export interface AppHeaderProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onRefresh: () => void;
  onNewApp: () => void;
  onLogout: () => void;
}

export interface ProgressBarProps {
  healthyPercentage: number;
  unhealthyPercentage: number;
}

export type AppStatus = 'healthy' | 'unhealthy' | 'unknown' | 'processing' | 'crashed' | 'suspended';

export interface Application {
  id: string;
  name: string;
  status: AppStatus;
  projects: { name: string; namespace: string }[];
  isFavourite: boolean;
  deploymentType?: string;
  latestCommit?: string;
  errorMessage?: string;
  totalDeployments: number;
  readyCount: number;
  failedCount: number;
  canaryActive: boolean;
}

export interface AppGridProps {
  applications: Application[];
  onToggleFavourite: (id: string) => void;
}

export interface EmptyStateProps {
  onCreateProject: () => void;
}