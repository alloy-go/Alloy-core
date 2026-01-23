'use client';

export function NoResults() {
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
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
        </div>
        <p className="text-lg font-semibold text-foreground">No applications found</p>
        <p className="text-sm mt-2">
          Try adjusting your filters or search query to find what you're looking for
        </p>
      </div>
    </div>
  );
}
