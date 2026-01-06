import { useState } from 'react';
import type { HistoryEntry } from '../../lib/api';

interface HistoryItemProps {
  entry: HistoryEntry;
  onDelete: (id: string) => void;
  onRedownload: (id: string) => void;
  sourceColor: string;
  sourceLabel: string;
}

const DownloadIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
    <polyline points="7 10 12 15 17 10" />
    <line x1="12" y1="15" x2="12" y2="3" />
  </svg>
);

const FolderIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
  </svg>
);

const TrashIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="3 6 5 6 21 6" />
    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
  </svg>
);

const ErrorIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" />
    <line x1="15" y1="9" x2="9" y2="15" />
    <line x1="9" y1="9" x2="15" y2="15" />
  </svg>
);

const CheckIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="20 6 9 17 4 12" />
  </svg>
);

export function HistoryItem({ entry, onDelete, onRedownload, sourceColor, sourceLabel }: HistoryItemProps) {
  const [redownloading, setRedownloading] = useState(false);

  const formatSize = (bytes: number) => {
    if (!bytes || bytes === 0) return '';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const formatDuration = (seconds: number) => {
    if (!seconds) return '';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const formatTime = (dateStr: string) => {
    return new Date(dateStr).toLocaleTimeString('fr-FR', {
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const handleRedownload = async () => {
    setRedownloading(true);
    try {
      await onRedownload(entry.id);
    } finally {
      setRedownloading(false);
    }
  };

  const handleOpenFolder = () => {
    if (entry.outputPath) {
      // In web mode, this is not available
      alert('Opening folders is not available in web mode.');
    }
  };

  const isError = entry.status === 'error';

  return (
    <div
      className="flex items-center gap-4 p-3 rounded-lg transition-colors hover:bg-opacity-50"
      style={{
        background: 'var(--color-bg-secondary)',
        borderLeft: `3px solid ${isError ? 'var(--color-error)' : sourceColor}`,
      }}
    >
      {/* Thumbnail */}
      <div className="w-12 h-12 rounded overflow-hidden flex-shrink-0">
        {entry.thumbnail ? (
          <img
            src={entry.thumbnail}
            alt={entry.title}
            className="w-full h-full object-cover"
          />
        ) : (
          <div
            className="w-full h-full flex items-center justify-center"
            style={{ background: 'var(--color-bg-tertiary)' }}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{ color: 'var(--color-text-tertiary)' }}>
              <rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18" />
              <line x1="7" y1="2" x2="7" y2="22" />
              <line x1="17" y1="2" x2="17" y2="22" />
              <line x1="2" y1="12" x2="22" y2="12" />
              <line x1="2" y1="7" x2="7" y2="7" />
              <line x1="2" y1="17" x2="7" y2="17" />
              <line x1="17" y1="17" x2="22" y2="17" />
              <line x1="17" y1="7" x2="22" y2="7" />
            </svg>
          </div>
        )}
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <h4 className="font-medium truncate" style={{ color: 'var(--color-text-primary)' }}>
            {entry.title || 'Unknown Title'}
          </h4>
          {isError ? (
            <span
              className="flex items-center gap-1 px-2 py-0.5 rounded text-xs"
              style={{ background: 'rgba(239, 68, 68, 0.2)', color: 'var(--color-error)' }}
            >
              <ErrorIcon />
              Error
            </span>
          ) : (
            <span
              className="flex items-center gap-1 px-2 py-0.5 rounded text-xs"
              style={{
                background: `${sourceColor}20`,
                color: sourceColor,
              }}
            >
              <CheckIcon />
              {sourceLabel}
            </span>
          )}
        </div>
        <div className="flex items-center gap-3 text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
          <span>{entry.artist || 'Unknown Artist'}</span>
          {entry.duration && entry.duration > 0 && (
            <>
              <span>•</span>
              <span>{formatDuration(entry.duration)}</span>
            </>
          )}
          {entry.fileSize && entry.fileSize > 0 && (
            <>
              <span>•</span>
              <span>{formatSize(entry.fileSize)}</span>
            </>
          )}
          {entry.quality && (
            <>
              <span>•</span>
              <span>{entry.quality}</span>
            </>
          )}
          <span>•</span>
          <span>{formatTime(entry.completedAt)}</span>
        </div>
        {isError && entry.error && (
          <p className="text-xs mt-1 truncate" style={{ color: 'var(--color-error)' }}>
            {entry.error}
          </p>
        )}
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2 flex-shrink-0">
        {/* Redownload */}
        <button
          onClick={handleRedownload}
          disabled={redownloading}
          className="p-2 rounded-lg transition-colors hover:bg-opacity-50"
          style={{ color: 'var(--color-accent)' }}
          title="Redownload"
        >
          {redownloading ? (
            <div
              className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin"
            />
          ) : (
            <DownloadIcon />
          )}
        </button>

        {/* Open Folder */}
        {entry.outputPath && entry.status === 'complete' && (
          <button
            onClick={handleOpenFolder}
            className="p-2 rounded-lg transition-colors hover:bg-opacity-50"
            style={{ color: 'var(--color-text-secondary)' }}
            title="Open folder"
          >
            <FolderIcon />
          </button>
        )}

        {/* Delete */}
        <button
          onClick={() => onDelete(entry.id)}
          className="p-2 rounded-lg transition-colors hover:bg-opacity-50"
          style={{ color: 'var(--color-text-tertiary)' }}
          title="Remove from history"
        >
          <TrashIcon />
        </button>
      </div>
    </div>
  );
}
