'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Sidebar } from '@/components/applications/sidebar';
import { AppHeader } from '@/components/applications/app-header';
import { AppGrid } from '@/components/applications/app-grid';
import { ProgressBar } from '@/components/applications/progress-bar';
import { EmptyState } from '@/components/applications/empty-state';
import { NoResults } from '@/components/applications/no-results';
import { NewProjectDialog } from '@/components/applications/project-dialog';
import { APIError } from '@/lib/api';
import { Project } from '@/lib/types';
import { getDisplayStatus } from '@/lib/utils';
import { projectAPI } from '@/lib/api';
import { Loader2 } from 'lucide-react';

type AppStatus = 'healthy' | 'unhealthy' | 'unknown' | 'processing' | 'crashed' | 'suspended';

interface Application {
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

export function ApplicationsDashboard() {
  const router = useRouter();
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedStatuses, setSelectedStatuses] = useState<string[]>([]);
  const [showFavourites, setShowFavourites] = useState(false);
  const [applications, setApplications] = useState<Application[]>([]);
  const [isNewProjectDialogOpen, setIsNewProjectDialogOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const userId = localStorage.getItem('user_id');
    if (!userId) {
      router.push('/login');
    } else {
      fetchProjects();
    }
  }, [router]);

  const fetchProjects = async () => {
    setIsLoading(true);
    setError('');
    
    try {
      const userId = localStorage.getItem('user_id') || '';
      const response = await projectAPI.getProjects(userId);
      const projects = response.projects || [];
      
      const transformedApps: Application[] = projects.map((project: Project) => ({
        id: project.project_id || project.project_name,
        name: project.project_name,
        status: getDisplayStatus(project.latest_status),
        projects: [
          {
            name: project.latest_deployment_name || project.project_name,
            namespace: project.latest_namespace || project.context_name || 'default',
          },
        ],
        isFavourite: false,
        deploymentType: project.latest_deployment_type,
        latestCommit: project.latest_commit_sha?.slice(0, 7),
        errorMessage: project.latest_error_message,
        totalDeployments: project.total_deployments,
        readyCount: project.ready_count,
        failedCount: project.failed_count,
        canaryActive: project.canary_active,
      }));

      setApplications(transformedApps);
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
        
        if (err.status === 401) {
          localStorage.removeItem('user_id');
          router.push('/login');
        }
      } else {
        setError('Failed to fetch projects');
      }
      console.error('Error fetching projects:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const stats = {
    favourites: applications.filter((app) => app.isFavourite).length,
    unknown: applications.filter((app) => app.status === 'unknown').length,
    healthy: applications.filter((app) => app.status === 'healthy').length,
    processing: applications.filter((app) => app.status === 'processing').length,
    crashed: applications.filter((app) => app.status === 'crashed').length,
    suspended: applications.filter((app) => app.status === 'suspended').length,
  };

  const healthyCount = applications.filter((app) => app.status === 'healthy').length;
  const totalCount = applications.length;
  const healthyPercentage = totalCount > 0 ? Math.round((healthyCount / totalCount) * 100) : 0;
  const unhealthyPercentage = 100 - healthyPercentage;

  const filteredApplications = applications.filter((app) => {
    const matchesSearch = app.name.toLowerCase().includes(searchQuery.toLowerCase());
    const matchesStatus = selectedStatuses.length === 0 || selectedStatuses.includes(app.status);
    const matchesFavourite = !showFavourites || app.isFavourite;
    return matchesSearch && matchesStatus && matchesFavourite;
  });

  const handleStatusChange = (status: string) => {
    setSelectedStatuses((prev) =>
      prev.includes(status) ? prev.filter((s) => s !== status) : [...prev, status]
    );
  };

  const handleToggleFavourite = (id: string) => {
    setApplications((prev) =>
      prev.map((app) => (app.id === id ? { ...app, isFavourite: !app.isFavourite } : app))
    );
  };

  const handleRefresh = () => {
    fetchProjects();
  };

  const handleNewApp = () => {
    setIsNewProjectDialogOpen(true);
  };

  const handleProjectCreated = () => {
    console.log('Project created successfully! Refreshing...');
    handleRefresh();
  };

  const handleLogout = () => {
    localStorage.removeItem('user_id');
    router.push('/login');
  };

  if (isLoading) {
    return (
      <div className="flex min-h-screen bg-background items-center justify-center">
        <div className="text-center">
          <Loader2 className="w-8 h-8 animate-spin mx-auto mb-4 text-primary" />
          <p className="text-muted-foreground">Loading your projects...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar
        selectedStatuses={selectedStatuses}
        onStatusChange={handleStatusChange}
        showFavourites={showFavourites}
        onFavouritesChange={setShowFavourites}
        counts={stats}
      />

      <div className="flex-1 ml-64">
        <div className="p-8 max-w-7xl mx-auto">
          <AppHeader
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            onRefresh={handleRefresh}
            onNewApp={handleNewApp}
            onLogout={handleLogout}
          />

          {error && (
            <div className="mt-6 bg-destructive/10 text-destructive text-sm p-4 rounded-md border border-destructive/20">
              {error}
            </div>
          )}

          {applications.length > 0 && (
            <div className="mt-6">
              <ProgressBar
                healthyPercentage={healthyPercentage}
                unhealthyPercentage={unhealthyPercentage}
              />
            </div>
          )}

          {applications.length > 0 ? (
            <div className="mt-8">
              <AppGrid
                applications={filteredApplications}
                onToggleFavourite={handleToggleFavourite}
              />
            </div>
          ) : (
            <EmptyState onCreateProject={handleNewApp} />
          )}

          {filteredApplications.length === 0 && applications.length > 0 && (
            <NoResults />
          )}
        </div>
      </div>

      <NewProjectDialog
        open={isNewProjectDialogOpen}
        onOpenChange={setIsNewProjectDialogOpen}
        onSuccess={handleProjectCreated}
      />
    </div>
  );
}
