'use client';

import { Star } from 'lucide-react';
import { Card } from '@/components/ui/card';
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
}

export function AppCard({ id, name, status, projects, isFavourite = false, onToggleFavourite }: AppCardProps) {
  const [favourite, setFavourite] = useState(isFavourite);

  const statusColors = {
    healthy: 'border-l-green-500',
    unhealthy: 'border-l-red-500',
    unknown: 'border-l-gray-400',
    processing: 'border-l-blue-500',
    crashed: 'border-l-red-600',
    suspended: 'border-l-yellow-500',
  };

  const handleToggleFavourite = () => {
    setFavourite(!favourite);
    onToggleFavourite?.(id);
  };

  return (
    <Card className={`border-l-4 ${statusColors[status]} bg-white hover:shadow-lg transition-shadow`}>
      <div className="p-4 space-y-3">
        {/* Header */}
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-gray-100 rounded-lg flex items-center justify-center">
              <span className="text-xl">📦</span>
            </div>
            <div>
              <h3 className="font-semibold text-foreground">{name}</h3>
            </div>
          </div>
          <button
            onClick={handleToggleFavourite}
            className="text-gray-400 hover:text-yellow-400 transition-colors"
          >
            <Star className={`w-5 h-5 ${favourite ? 'fill-yellow-400 text-yellow-400' : ''}`} />
          </button>
        </div>

        {/* Projects List */}
        <div className="space-y-1 text-sm">
          {projects.slice(0, 4).map((project, index) => (
            <div key={index} className="text-gray-600">
              <span className="font-medium">Project :</span> {project.namespace}
            </div>
          ))}
          {status === 'healthy' && projects.length > 0 && (
            <div className="text-gray-600">
              <span className="font-medium">Status :</span> Healthy
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}
