'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Sidebar } from '@/components/applications/sidebar';
import { AppHeader } from '@/components/applications/app-header';
import { AppCard } from '@/components/applications/app-card';
import { ProgressBar } from '@/components/applications/progress-bar';

// Define the status type
type AppStatus = 'healthy' | 'unhealthy' | 'unknown' | 'processing' | 'crashed' | 'suspended';

interface Application {
  id: string;
  name: string;
  status: AppStatus;
  projects: { name: string; namespace: string }[];
  isFavourite: boolean;
}

// Mock data - Replace with API calls later
const mockApplications: Application[] = [
  {
    id: '1',
    name: 'Sample-project 1',
    status: 'unhealthy',
    projects: [
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
    ],
    isFavourite: false,
  },
  {
    id: '2',
    name: 'Sample-project 1',
    status: 'healthy',
    projects: [
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
    ],
    isFavourite: true,
  },
  {
    id: '3',
    name: 'Sample-project 1',
    status: 'healthy',
    projects: [
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
    ],
    isFavourite: false,
  },
  {
    id: '4',
    name: 'Sample-project 1',
    status: 'healthy',
    projects: [
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
    ],
    isFavourite: false,
  },
  {
    id: '5',
    name: 'Sample-project 1',
    status: 'healthy',
    projects: [
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
      { name: 'default', namespace: 'default' },
    ],
    isFavourite: false,
  },
];

export default function ApplicationsPage() {
  const router = useRouter();
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedStatuses, setSelectedStatuses] = useState<string[]>([]);
  const [showFavourites, setShowFavourites] = useState(false);
  const [applications, setApplications] = useState(mockApplications);

  // Authentication check
  useEffect(() => {
    const userId = localStorage.getItem('user_id');
    if (!userId) {
      router.push('/login');
    }
  }, [router]);

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
  const healthyPercentage = Math.round((healthyCount / totalCount) * 100);
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
  };

  const handleRefresh = () => {
    // TODO: Implement refresh logic
    console.log('Refreshing applications...');
  };

  const handleNewApp = () => {
    // TODO: Implement new app modal/page
    console.log('Creating new application...');
  };

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

          {/* Progress Bar */}
          <div className="mt-6">
            <ProgressBar
              healthyPercentage={healthyPercentage}
              unhealthyPercentage={unhealthyPercentage}
            />
          </div>

          {/* Application Grid */}
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

          {filteredApplications.length === 0 && (
            <div className="mt-12 text-center text-gray-500">
              <p className="text-lg">No applications found</p>
              <p className="text-sm mt-2">Try adjusting your filters or search query</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
