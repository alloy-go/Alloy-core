'use client';

import { Star } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useState } from 'react';

interface Project {
  name: string;
  namespace: string;
}

interface AppCardProps {
  id: string;
  name: string;
  status: 'healthy' | 'unhealthy' | 'unknown' | 'processing' | 'crashed' | 'suspended';
  projects: Project[];
  isFavourite?: boolean;
  onToggleFavourite?: (id: string) => void;
  deploymentType?: string;
  latestCommit?: string;
  errorMessage?: string;
  totalDeployments: number;
  readyCount: number;
  failedCount: number;
  canaryActive: boolean;
}

export function AppCard({ 
  id, 
  name, 
  status, 
  projects, 
  isFavourite = false, 
  onToggleFavourite,
  deploymentType,
  latestCommit,
  errorMessage,
  totalDeployments,
  readyCount,
  failedCount,
  canaryActive 
}: AppCardProps) {
  const [favourite, setFavourite] = useState(isFavourite);

  const statusColors = {
    healthy: 'border-l-green-500 bg-green-50/50 dark:bg-green-950/20',
    unhealthy: 'border-l-red-500 bg-red-50/50 dark:bg-red-950/20',
    unknown: 'border-l-gray-400 bg-gray-50/50 dark:bg-gray-950/20',
    processing: 'border-l-blue-500 bg-blue-50/50 dark:bg-blue-950/20',
    crashed: 'border-l-red-600 bg-red-50/50 dark:bg-red-950/20',
    suspended: 'border-l-yellow-500 bg-yellow-50/50 dark:bg-yellow-950/20',
  };

  const statusBadgeVariant = {
    healthy: 'default',
    unhealthy: 'destructive',
    unknown: 'secondary',
    processing: 'outline',
    crashed: 'destructive',
    suspended: 'secondary',
  } as const;

  const handleToggleFavourite = () => {
    setFavourite(!favourite);
    onToggleFavourite?.(id);
  };

  return (
    <Card className={`border-l-4 ${statusColors[status]} hover:shadow-lg transition-shadow`}>
      <div className="p-4 space-y-3">
        {/* Header */}
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-primary/10 rounded-lg flex items-center justify-center">
              <span className="text-primary font-semibold text-sm">
                {name.slice(0, 2).toUpperCase()}
              </span>
            </div>
            <div>
              <h3 className="font-semibold text-foreground">{name}</h3>
              <p className="text-xs text-muted-foreground">
                {projects[0]?.namespace || 'default'}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {canaryActive && (
              <Badge variant="outline" className="text-xs">
                Canary
              </Badge>
            )}
            <button
              onClick={handleToggleFavourite}
              className="text-gray-400 hover:text-yellow-400 transition-colors"
            >
              <Star className={`w-5 h-5 ${favourite ? 'fill-yellow-400 text-yellow-400' : ''}`} />
            </button>
          </div>
        </div>

        {/* Deployment Info */}
        <div className="space-y-2 text-sm">
          <div className="flex justify-between items-center">
            <span className="text-muted-foreground">Status:</span>
            <Badge variant={statusBadgeVariant[status]}>
              {status.charAt(0).toUpperCase() + status.slice(1)}
            </Badge>
          </div>

          {latestCommit && (
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Commit:</span>
              <code className="text-xs bg-muted px-2 py-0.5 rounded">
                {latestCommit}
              </code>
            </div>
          )}

          {deploymentType && (
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Type:</span>
              <span className="text-xs font-medium">
                {deploymentType.charAt(0).toUpperCase() + deploymentType.slice(1)}
              </span>
            </div>
          )}

          {totalDeployments > 0 && (
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Deployments:</span>
              <span className="text-xs">
                <span className="text-green-600 font-semibold">{readyCount}</span>
                {' / '}
                <span className="text-red-600 font-semibold">{failedCount}</span>
                {' / '}
                <span className="font-semibold">{totalDeployments}</span>
              </span>
            </div>
          )}

          {projects[0]?.name && (
            <div className="flex justify-between items-center">
              <span className="text-muted-foreground">Deployment:</span>
              <span className="text-xs truncate max-w-[150px]" title={projects[0].name}>
                {projects[0].name}
              </span>
            </div>
          )}
        </div>

        {/* Error Message */}
        {errorMessage && (
          <div className="mt-3 p-2 bg-destructive/10 border border-destructive/20 rounded text-xs text-destructive">
            <p className="font-semibold mb-1">Error:</p>
            <p className="line-clamp-2" title={errorMessage}>
              {errorMessage}
            </p>
          </div>
        )}
      </div>
    </Card>
  );
}
