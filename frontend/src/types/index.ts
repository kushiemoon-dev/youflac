// Re-export types from lib/api
export * from '../lib/api';

// Page navigation
export type Page = 'home' | 'history' | 'settings' | 'files' | 'terminal' | 'about';

// Queue status mapping
export type QueueStatus =
  | 'pending'
  | 'fetching_info'
  | 'downloading_video'
  | 'downloading_audio'
  | 'muxing'
  | 'organizing'
  | 'complete'
  | 'error'
  | 'cancelled';

// Theme types
export type Theme = 'dark' | 'light' | 'system';
export type AccentColor = 'pink' | 'blue' | 'green' | 'purple' | 'orange' | 'teal' | 'red' | 'yellow';
