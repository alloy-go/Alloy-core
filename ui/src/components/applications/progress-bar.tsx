'use client';
import {ProgressBarProps} from '@/lib/types';

export function ProgressBar({ healthyPercentage, unhealthyPercentage }: ProgressBarProps) {
  return (
    <div className="w-full">
      <div className="flex h-8 rounded-lg overflow-hidden shadow-sm">
        {unhealthyPercentage > 0 && (
          <div
            className="bg-red-500 flex items-center justify-center text-white text-sm font-semibold transition-all"
            style={{ width: `${unhealthyPercentage}%` }}
          >
            {unhealthyPercentage >= 10 && `${unhealthyPercentage}%`}
          </div>
        )}
        
        {healthyPercentage > 0 && (
          <div
            className="bg-green-500 flex items-center justify-center text-white text-sm font-semibold transition-all"
            style={{ width: `${healthyPercentage}%` }}
          >
            {healthyPercentage >= 10 && `${healthyPercentage}%`}
          </div>
        )}
      </div>
      
      <div className="flex items-center justify-center gap-4 mt-2 text-xs text-muted-foreground">
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 bg-red-500 rounded"></div>
          <span>Unhealthy ({unhealthyPercentage}%)</span>
        </div>
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 bg-green-500 rounded"></div>
          <span>Healthy ({healthyPercentage}%)</span>
        </div>
      </div>
    </div>
  );
}
