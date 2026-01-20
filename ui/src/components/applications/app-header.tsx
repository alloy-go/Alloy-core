'use client';

import { Plus, RefreshCw, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useRouter } from 'next/navigation';

interface AppHeaderProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onRefresh: () => void;
  onNewApp: () => void;
}

export function AppHeader({ searchQuery, onSearchChange, onRefresh, onNewApp }: AppHeaderProps) {
  const router = useRouter();

  const handleLogout = () => {
    localStorage.removeItem('user_id');
    router.push('/login');
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold text-foreground">Applications</h1>
        <Button
          onClick={handleLogout}
          variant="outline"
          className="bg-[oklch(0.145_0_0)] text-white border-0 hover:bg-[oklch(0.2_0_0)] hover:text-white"
        >
          Logout
        </Button>
      </div>

      <div className="flex items-center gap-3">
        <Button
          onClick={onNewApp}
          className="bg-[oklch(0.145_0_0)] text-white hover:bg-[oklch(0.2_0_0)]"
        >
          <Plus className="w-4 h-4 mr-2" />
          New App
        </Button>
        <Button
          onClick={onRefresh}
          variant="outline"
          className="border-green-500 text-green-600 hover:bg-green-50"
        >
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh Apps
        </Button>
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <Input
            type="text"
            placeholder="Search Applications"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            className="pl-10"
          />
        </div>
      </div>
    </div>
  );
}
