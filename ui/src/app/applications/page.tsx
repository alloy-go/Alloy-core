'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Sidebar } from '@/components/applications/sidebar';
import { AppHeader } from '@/components/applications/app-header';
import { AppCard } from '@/components/applications/app-card';
import { ProgressBar } from '@/components/applications/progress-bar';
import { NewProjectDialog } from '@/components/applications/project-dialog';
import { projectAPI, APIError } from '@/lib/api';
import { Loader2 } from 'lucide-react';

// Define the status type
type AppStatus = 'healthy' | 'unhealthy' | 'unknown' | 'processing' | 'crashed' | 'suspended';

interface Application {
  id: string;
  name: string;
  status: AppStatus;
  projects: { name: string; namespace: string }[];
  isFavourite: boolean;
}

// Backend project type (adjust based on your actual response)
interface BackendProject {
  project_id: string;
  project_name: string;
  deployment_type: string;
  context_name: string;
  user_id: string;
  created_at?: string;
  status?: string;
}

export default function ApplicationsPage() {
  const router = useRouter();
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedStatuses, setSelectedStatuses] = useState<string[]>([]);
  const [showFavourites, setShowFavourites] = useState(false);
  const [applications, setApplications] = useState<Application[]>([]);
  const [isNewProjectDialogOpen, setIsNewProjectDialogOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  // Authentication check
  useEffect(() => {
    const userId = localStorage.getItem('user_id');
    if (!userId) {
      router.push('/login');
    } else {
      fetchProjects(userId);
    }
  }, [router]);

  // Fetch projects from API
  const fetchProjects = async (userId: string) => {
    setIsLoading(true);
    setError('');
    
    try {
      const response = await projectAPI.getProjects(userId);
      const projects = response.projects || [];
      
      // Transform backend projects to frontend applications
      const transformedApps: Application[] = projects.map((project: BackendProject) => ({
        id: project.project_id,
        name: project.project_name,
        status: mapStatusToAppStatus(project.status),
        projects: [
          {
            name: project.deployment_type,
            namespace: project.context_name,
          },
        ],
        isFavourite: false, // TODO: Add favourites to backend
      }));

      setApplications(transformedApps);
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError('Failed to fetch projects');
      }
      console.error('Error fetching projects:', err);
    } finally {
      setIsLoading(false);
    }
  };

  // Map backend status to frontend status
  const mapStatusToAppStatus = (backendStatus?: string): AppStatus => {
    if (!backendStatus) return 'unknown';
    
    const statusMap: Record<string, AppStatus> = {
      healthy: 'healthy',
      unhealthy: 'unhealthy',
      unknown: 'unknown',
      processing: 'processing',
      crashed: 'crashed',
      suspended: 'suspended',
    };

    return statusMap[backendStatus.toLowerCase()] || 'unknown';
  };

  // Calculate stats
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

  // Filter applications
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
    // TODO: Persist favourite state to backend
  };

  const handleRefresh = () => {
    const userId = localStorage.getItem('user_id');
    if (userId) {
      fetchProjects(userId);
    }
  };

  const handleNewApp = () => {
    setIsNewProjectDialogOpen(true);
  };

  const handleProjectCreated = () => {
    console.log('Project created successfully! Refreshing...');
    handleRefresh();
  };

  // Loading state
  if (isLoading) {
    return (
      <div className="flex min-h-screen bg-background items-center justify-center">
        <div className="text-center">
          <Loader2 className="w-8 h-8 animate-spin mx-auto mb-4 text-[oklch(0.145_0_0)]" />
          <p className="text-gray-600">Loading your projects...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen bg-background">
      {/* Sidebar with Filters */}
      <Sidebar
        selectedStatuses={selectedStatuses}
        onStatusChange={handleStatusChange}
        showFavourites={showFavourites}
        onFavouritesChange={setShowFavourites}
        counts={stats}
      />

      {/* Main Content */}
      <div className="flex-1 ml-64">
        <div className="p-8 max-w-7xl mx-auto">
          {/* Header */}
          <AppHeader
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            onRefresh={handleRefresh}
            onNewApp={handleNewApp}
          />

          {/* Error Message */}
          {error && (
            <div className="mt-6 bg-destructive/10 text-destructive text-sm p-4 rounded-md border border-destructive/20">
              {error}
            </div>
          )}

          {/* Progress Bar */}
          {applications.length > 0 && (
            <div className="mt-6">
              <ProgressBar
                healthyPercentage={healthyPercentage}
                unhealthyPercentage={unhealthyPercentage}
              />
            </div>
          )}

          {/* Application Grid */}
          {applications.length > 0 ? (
            <div className="mt-8 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {filteredApplications.map((app) => (
                <AppCard
                  key={app.id}
                  id={app.id}
                  name={app.name}
                  status={app.status}
                  projects={app.projects}
                  isFavourite={app.isFavourite}
                  onToggleFavourite={handleToggleFavourite}
                />
              ))}
            </div>
          ) : (
            <div className="mt-12 text-center text-gray-500">
              <p className="text-lg font-semibold">No projects yet</p>
              <p className="text-sm mt-2 mb-4">Get started by creating your first project</p>
              <button
                onClick={handleNewApp}
                className="px-4 py-2 bg-[oklch(0.145_0_0)] text-white rounded-lg hover:bg-[oklch(0.2_0_0)] transition-colors"
              >
                Create Your First Project
              </button>
            </div>
          )}

          {filteredApplications.length === 0 && applications.length > 0 && (
            <div className="mt-12 text-center text-gray-500">
              <p className="text-lg">No applications found</p>
              <p className="text-sm mt-2">Try adjusting your filters or search query</p>
            </div>
          )}
        </div>
      </div>

      {/* Project Creation Dialog */}
      <NewProjectDialog
        open={isNewProjectDialogOpen}
        onOpenChange={setIsNewProjectDialogOpen}
        onSuccess={handleProjectCreated}
      />
    </div>
  );
}
