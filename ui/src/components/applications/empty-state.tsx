'use client';
import { EmptyStateProps } from '@/lib/types';

export function EmptyState({ onCreateProject }: EmptyStateProps) {
  return (
    <div className="mt-12 text-center text-muted-foreground">
      <div className="max-w-md mx-auto">
        <div className="w-20 h-20 mx-auto mb-4 bg-muted rounded-full flex items-center justify-center">
          <svg
            className="w-10 h-10 text-muted-foreground"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
            />
          </svg>
        </div>
        <p className="text-lg font-semibold text-foreground">No projects yet</p>
        <p className="text-sm mt-2 mb-6">
          Get started by creating your first project to deploy applications with Minimon CD
        </p>
        <button
          onClick={onCreateProject}
          className="px-6 py-2.5 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors font-medium"
        >
          Create Your First Project
        </button>
      </div>
    </div>
  );
}
