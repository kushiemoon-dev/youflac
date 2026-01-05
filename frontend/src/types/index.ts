// Re-export types from Wails bindings
export { backend, main } from '../../wailsjs/go/models';

// Page navigation
export type Page = 'home' | 'settings' | 'files' | 'terminal' | 'about';

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
export type AccentColor = 'pink' | 'blue' | 'green' | 'purple' | 'orange';

// Queue event from Wails
export interface QueueEvent {
  type: 'added' | 'updated' | 'removed' | 'completed' | 'error';
  itemId: string;
  item?: import('../../wailsjs/go/models').backend.QueueItem;
  progress?: number;
  status?: QueueStatus;
  error?: string;
}
