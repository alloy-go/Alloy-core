'use client';

import { AppCard } from './app-card';
import {AppGridProps} from '@/lib/types';



export function AppGrid({ applications, onToggleFavourite }: AppGridProps) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {applications.map((app) => (
        <AppCard
          key={app.id}
          id={app.id}
          name={app.name}
          status={app.status}
          projects={app.projects}
          isFavourite={app.isFavourite}
          onToggleFavourite={onToggleFavourite}
          deploymentType={app.deploymentType}
          latestCommit={app.latestCommit}
          errorMessage={app.errorMessage}
          totalDeployments={app.totalDeployments}
          readyCount={app.readyCount}
          failedCount={app.failedCount}
          canaryActive={app.canaryActive}
        />
      ))}
    </div>
  );
}
