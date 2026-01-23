import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { ProjectStatus } from "./types";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function getDisplayStatus(latestStatus: string): ProjectStatus {
  switch (latestStatus) {
    case 'ready':
      return 'healthy';
    case 'failed':
      return 'unhealthy';
    case 'pending':
    case 'progressing':
      return 'processing';
    default:
      return 'unknown';
  }
}