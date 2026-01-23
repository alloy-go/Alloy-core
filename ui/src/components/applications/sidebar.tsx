'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { LayoutGrid, Settings, User, Star, HelpCircle, CheckCircle2, RefreshCw, AlertTriangle, PauseCircle } from 'lucide-react';
import { Checkbox } from '@/components/ui/checkbox';
import { Separator } from '@/components/ui/separator';

interface SidebarProps {
  selectedStatuses: string[];
  onStatusChange: (status: string) => void;
  showFavourites: boolean;
  onFavouritesChange: (checked: boolean) => void;
  counts: {
    favourites: number;
    unknown: number;
    healthy: number;
    processing: number;
    crashed: number;
    suspended: number;
  };
}

export function Sidebar({
  selectedStatuses,
  onStatusChange,
  showFavourites,
  onFavouritesChange,
  counts,
}: SidebarProps) {
  const pathname = usePathname();

  const navItems = [
    { icon: LayoutGrid, label: 'Applications', href: '/applications' },
    { icon: Settings, label: 'Settings', href: '/settings' },
  ];

  const healthStatuses = [
    { id: 'unknown', label: 'Unknown', icon: HelpCircle, color: 'text-gray-400' },
    { id: 'healthy', label: 'Healthy', icon: CheckCircle2, color: 'text-green-500' },
    { id: 'processing', label: 'Processing', icon: RefreshCw, color: 'text-blue-500' },
    { id: 'crashed', label: 'Crashed', icon: AlertTriangle, color: 'text-red-500' },
    { id: 'suspended', label: 'Suspended', icon: PauseCircle, color: 'text-yellow-500' },
  ];

  return (
    <div className="w-64 h-screen bg-[oklch(0.145_0_0)] text-white flex flex-col fixed left-0 top-0 overflow-y-auto">
      {/* Branding */}
      <div className="p-6 border-b border-white/10">
        <div className="flex items-center gap-3 mb-2">
          <div className="w-10 h-10 bg-white rounded-lg flex items-center justify-center">
            <span className="text-2xl">🚀</span>
          </div>
          <div>
            <h1 className="text-xl font-bold">Minimon CD</h1>
          </div>
        </div>
        <p className="text-sm text-white/60 ml-[52px]">v1.0.0</p>
      </div>

      {/* Navigation */}
      <nav className="p-4 border-b border-white/10">
        <div className="space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${
                  isActive
                    ? 'bg-white/10 text-white'
                    : 'text-white/70 hover:bg-white/5 hover:text-white'
                }`}
              >
                <Icon className="w-5 h-5" />
                <span className="font-medium">{item.label}</span>
              </Link>
            );
          })}
        </div>
      </nav>

      {/* Filters Section */}
      <div className="p-4 flex-1">
        <h3 className="text-sm font-semibold text-white/40 uppercase tracking-wider mb-4">
          Filters
        </h3>
        
        {/* Favourites */}
        <div className="flex items-center justify-between mb-4">
          <label className="flex items-center gap-3 cursor-pointer">
            <Checkbox
              checked={showFavourites}
              onCheckedChange={onFavouritesChange}
              className="border-white/20 data-[state=checked]:bg-white data-[state=checked]:border-white"
            />
            <Star className="w-4 h-4 text-yellow-400 fill-yellow-400" />
            <span className="text-white/90">Favourites</span>
          </label>
          <span className="text-sm text-white/50">{counts.favourites}</span>
        </div>

        <Separator className="bg-white/10 mb-4" />

        {/* Health Status */}
        <div>
          <div className="flex items-center justify-between mb-4">
            <h4 className="text-sm font-semibold text-white/40 uppercase tracking-wider">
              Health Status
            </h4>
            <button className="text-white/40 hover:text-white/60 transition-colors">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
              </svg>
            </button>
          </div>
          
          <div className="space-y-3">
            {healthStatuses.map((status) => {
              const Icon = status.icon;
              const count = counts[status.id as keyof typeof counts];
              return (
                <div key={status.id} className="flex items-center justify-between">
                  <label className="flex items-center gap-3 cursor-pointer">
                    <Checkbox
                      checked={selectedStatuses.includes(status.id)}
                      onCheckedChange={() => onStatusChange(status.id)}
                      className="border-white/20 data-[state=checked]:bg-white data-[state=checked]:border-white"
                    />
                    <Icon className={`w-4 h-4 ${status.color}`} />
                    <span className="text-white/90">{status.label}</span>
                  </label>
                  <span className="text-sm text-white/50">{count}</span>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
