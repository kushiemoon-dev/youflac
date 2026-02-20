import type { QueueItem as QueueItemType, QueueStats } from '../../lib/api';
import { QueueItem } from './QueueItem';

// Icons
const TrashIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="3 6 5 6 21 6" />
    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
  </svg>
);

const CheckCircleIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
    <polyline points="22 4 12 14.01 9 11.01" />
  </svg>
);

const RefreshIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="23 4 23 10 17 10" />
    <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
  </svg>
);

const PauseIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="6" y="4" width="4" height="16" />
    <rect x="14" y="4" width="4" height="16" />
  </svg>
);

const PlayIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polygon points="5 3 19 12 5 21 5 3" />
  </svg>
);

interface QueueListProps {
  items: QueueItemType[];
  stats: QueueStats | null;
  onCancel: (id: string) => void;
  onRemove: (id: string) => void;
  onClearCompleted: () => void;
  onRetryFailed: () => void;
  onClearAll: () => void;
  onPauseAll?: () => void;
  onResumeAll?: () => void;
}

export function QueueList({
  items,
  stats,
  onCancel,
  onRemove,
  onClearCompleted,
  onRetryFailed,
  onClearAll,
  onPauseAll,
  onResumeAll,
}: QueueListProps) {
  const hasItems = items.length > 0;
  const hasActive = stats && (stats.active > 0 || stats.pending > 0);
  const hasPaused = items.some((i) => i.status === 'paused');
  const hasCompleted = stats && stats.completed > 0;
  const hasFailed = stats && stats.failed > 0;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h3
            className="text-lg font-medium"
            style={{ color: 'var(--color-text-primary)' }}
          >
            Download Queue
          </h3>
          {stats && (
            <div
              className="flex items-center gap-3 text-sm"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              {stats.active > 0 && (
                <span className="flex items-center gap-1.5">
                  <span
                    className="w-2 h-2 rounded-full animate-pulse"
                    style={{ background: 'var(--color-accent)' }}
                  />
                  {stats.active} active
                </span>
              )}
              {stats.pending > 0 && (
                <span>{stats.pending} pending</span>
              )}
              {stats.completed > 0 && (
                <span className="flex items-center gap-1">
                  <CheckCircleIcon />
                  {stats.completed}
                </span>
              )}
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2">
          {hasItems && onResumeAll && hasPaused && (
            <button
              className="btn-ghost text-sm flex items-center gap-2"
              onClick={onResumeAll}
              style={{ color: 'var(--color-success)' }}
            >
              <PlayIcon />
              Resume all
            </button>
          )}
          {hasItems && onPauseAll && hasActive && (
            <button
              className="btn-ghost text-sm flex items-center gap-2"
              onClick={onPauseAll}
            >
              <PauseIcon />
              Pause all
            </button>
          )}
          {hasFailed && (
            <button
              className="btn-ghost text-sm flex items-center gap-2"
              onClick={onRetryFailed}
              style={{ color: 'var(--color-warning)' }}
            >
              <RefreshIcon />
              Retry failed ({stats?.failed})
            </button>
          )}
          {hasCompleted && (
            <button
              className="btn-ghost text-sm flex items-center gap-2"
              onClick={onClearCompleted}
            >
              <CheckCircleIcon />
              Clear completed
            </button>
          )}
          {hasItems && (
            <button
              className="btn-ghost text-sm flex items-center gap-2"
              onClick={onClearAll}
              style={{ color: 'var(--color-error)' }}
            >
              <TrashIcon />
              Clear all
            </button>
          )}
        </div>
      </div>

      {/* Queue items */}
      {hasItems ? (
        <div className="space-y-3">
          {items.map((item, index) => (
            <div
              key={item.id}
              style={{ animationDelay: `${index * 0.05}s` }}
            >
              <QueueItem
                item={item}
                onCancel={onCancel}
                onRemove={onRemove}
              />
            </div>
          ))}
        </div>
      ) : (
        <div
          className="card p-12 text-center"
          style={{ borderStyle: 'dashed' }}
        >
          <div
            className="w-16 h-16 rounded-2xl mx-auto mb-4 flex items-center justify-center"
            style={{ background: 'var(--color-bg-tertiary)' }}
          >
            <svg
              width="32"
              height="32"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              style={{ color: 'var(--color-text-tertiary)' }}
            >
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7 10 12 15 17 10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
          </div>
          <p
            className="text-sm font-medium mb-1"
            style={{ color: 'var(--color-text-secondary)' }}
          >
            No downloads in queue
          </p>
          <p
            className="text-sm"
            style={{ color: 'var(--color-text-tertiary)' }}
          >
            Paste a YouTube or Spotify URL above to get started
          </p>
        </div>
      )}
    </div>
  );
}
