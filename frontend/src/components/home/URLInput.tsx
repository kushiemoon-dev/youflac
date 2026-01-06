import { useState, useCallback } from 'react';
import * as Api from '../../lib/api';
import type { VideoInfo } from '../../lib/api';

// Icons
const LinkIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
  </svg>
);

const SearchIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="11" cy="11" r="8" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </svg>
);

const LoaderIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="animate-spin">
    <line x1="12" y1="2" x2="12" y2="6" />
    <line x1="12" y1="18" x2="12" y2="22" />
    <line x1="4.93" y1="4.93" x2="7.76" y2="7.76" />
    <line x1="16.24" y1="16.24" x2="19.07" y2="19.07" />
    <line x1="2" y1="12" x2="6" y2="12" />
    <line x1="18" y1="12" x2="22" y2="12" />
    <line x1="4.93" y1="19.07" x2="7.76" y2="16.24" />
    <line x1="16.24" y1="7.76" x2="19.07" y2="4.93" />
  </svg>
);

const PlusIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <line x1="12" y1="5" x2="12" y2="19" />
    <line x1="5" y1="12" x2="19" y2="12" />
  </svg>
);

type Tab = 'url' | 'search';

interface URLInputProps {
  onAdd: (videoUrl: string, spotifyUrl?: string) => Promise<void>;
}

export function URLInput({ onAdd }: URLInputProps) {
  const [activeTab, setActiveTab] = useState<Tab>('url');
  const [url, setUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<VideoInfo | null>(null);

  const handleSubmit = useCallback(async () => {
    if (!url.trim()) return;

    setLoading(true);
    setError('');

    try {
      const trimmedUrl = url.trim();

      // Check if it's a playlist URL (contains list= parameter)
      const isPlaylist = trimmedUrl.includes('list=') && !trimmedUrl.includes('v=');

      if (isPlaylist) {
        // For playlists, skip preview and add directly
        await onAdd(trimmedUrl);
        setUrl('');
        setPreview(null);
      } else {
        // For single videos, try to get video info first for preview
        const videoInfo = await Api.GetVideoInfo(trimmedUrl);
        if (videoInfo) {
          setPreview(videoInfo);
          // Auto-add to queue
          await onAdd(trimmedUrl);
          setUrl('');
          setPreview(null);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to process URL');
    } finally {
      setLoading(false);
    }
  }, [url, onAdd]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !loading) {
      handleSubmit();
    }
  };

  return (
    <div className="space-y-4">
      {/* Tabs */}
      <div className="tabs inline-flex">
        <button
          className={`tab ${activeTab === 'url' ? 'active' : ''}`}
          onClick={() => setActiveTab('url')}
        >
          <span className="flex items-center gap-2">
            <LinkIcon />
            URL
          </span>
        </button>
        <button
          className={`tab ${activeTab === 'search' ? 'active' : ''}`}
          onClick={() => setActiveTab('search')}
        >
          <span className="flex items-center gap-2">
            <SearchIcon />
            Search
          </span>
        </button>
      </div>

      {/* Input area */}
      <div className="flex gap-3">
        <div className="relative flex-1">
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={activeTab === 'url'
              ? 'Paste YouTube, Spotify, or music video URL...'
              : 'Search for artist or song...'
            }
            className="w-full pr-12"
            disabled={loading}
          />
          {url && !loading && (
            <button
              className="absolute right-2 top-1/2 -translate-y-1/2 btn-icon"
              onClick={() => setUrl('')}
              style={{ color: 'var(--color-text-tertiary)' }}
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          )}
        </div>
        <button
          className="btn-primary flex items-center gap-2"
          onClick={handleSubmit}
          disabled={loading || !url.trim()}
        >
          {loading ? <LoaderIcon /> : <PlusIcon />}
          {loading ? 'Adding...' : 'Add'}
        </button>
      </div>

      {/* Error message */}
      {error && (
        <div
          className="text-sm p-3 rounded-lg"
          style={{
            background: 'var(--color-error-subtle)',
            color: 'var(--color-error)'
          }}
        >
          {error}
        </div>
      )}

      {/* Preview card */}
      {preview && (
        <div className="card p-4 animate-slide-up">
          <div className="flex gap-4">
            <div
              className="w-20 h-20 rounded-lg overflow-hidden flex-shrink-0"
              style={{ background: 'var(--color-bg-tertiary)' }}
            >
              {preview.thumbnail && (
                <img
                  src={preview.thumbnail}
                  alt=""
                  className="w-full h-full object-cover"
                />
              )}
            </div>
            <div>
              <h4
                className="font-medium"
                style={{ color: 'var(--color-text-primary)' }}
              >
                {preview.title}
              </h4>
              <p
                className="text-sm"
                style={{ color: 'var(--color-text-secondary)' }}
              >
                {preview.artist}
              </p>
              {preview.duration && (
                <p
                  className="text-sm mt-1"
                  style={{ color: 'var(--color-text-tertiary)' }}
                >
                  {Math.floor(preview.duration / 60)}:{(Math.floor(preview.duration) % 60).toString().padStart(2, '0')}
                </p>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Supported platforms hint */}
      <div className="flex items-center gap-4 text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
        <span>Supported:</span>
        <span className="flex items-center gap-1">
          <span className="w-4 h-4 rounded bg-red-500/20 flex items-center justify-center text-red-400">Y</span>
          YouTube
        </span>
        <span className="flex items-center gap-1">
          <span className="w-4 h-4 rounded bg-green-500/20 flex items-center justify-center text-green-400">S</span>
          Spotify
        </span>
        <span className="flex items-center gap-1">
          <span className="w-4 h-4 rounded bg-cyan-500/20 flex items-center justify-center text-cyan-400">T</span>
          Tidal
        </span>
      </div>
    </div>
  );
}
