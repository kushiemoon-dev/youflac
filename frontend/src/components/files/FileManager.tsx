import { useState, useEffect } from 'react';
import { Header } from '../layout/Header';
import { ListFiles, BrowseDirectory, OpenFile, OpenDirectory, GetDefaultOutputDirectory, GetImageAsDataURL, GetPlaylistFolders, ReorganizePlaylist, FlattenPlaylistFolder } from '../../../wailsjs/go/main/App';

// Icons
const FolderIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
  </svg>
);

const RefreshIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="23 4 23 10 17 10" />
    <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
  </svg>
);

const FileVideoIcon = () => (
  <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <path d="M10 11l5 3-5 3v-6z" />
  </svg>
);

const ImageIcon = () => (
  <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
    <circle cx="8.5" cy="8.5" r="1.5" />
    <polyline points="21 15 16 10 5 21" />
  </svg>
);

const FileTextIcon = () => (
  <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <line x1="16" y1="13" x2="8" y2="13" />
    <line x1="16" y1="17" x2="8" y2="17" />
    <polyline points="10 9 9 9 8 9" />
  </svg>
);

const PlayIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M8 5v14l11-7z" />
  </svg>
);

const FolderOpenIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
  </svg>
);

const SortIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <line x1="4" y1="6" x2="11" y2="6" />
    <line x1="4" y1="12" x2="11" y2="12" />
    <line x1="4" y1="18" x2="13" y2="18" />
    <polyline points="15 15 18 18 21 15" />
    <line x1="18" y1="6" x2="18" y2="18" />
  </svg>
);

const MusicIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M9 18V5l12-2v13" />
    <circle cx="6" cy="18" r="3" />
    <circle cx="18" cy="16" r="3" />
  </svg>
);

const FileAudioIcon = () => (
  <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <path d="M9 15v-3l6-1v3" />
    <circle cx="7" cy="16" r="2" />
    <circle cx="13" cy="15" r="2" />
  </svg>
);

interface FileInfo {
  name: string;
  path: string;
  size: number;
  type: string;
}

type Tab = 'videos' | 'audio' | 'covers' | 'nfo';

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export function FileManager() {
  const [activeTab, setActiveTab] = useState<Tab>('videos');
  const [path, setPath] = useState('');
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [posterCache, setPosterCache] = useState<Record<string, string>>({});
  const [playlistFolders, setPlaylistFolders] = useState<string[]>([]);
  const [reorganizing, setReorganizing] = useState<string | null>(null);
  const [flattening, setFlattening] = useState<string | null>(null);

  const loadFiles = async (directory?: string) => {
    setLoading(true);
    try {
      // Load all file types to get accurate counts
      const result = await ListFiles(directory || path, "all");
      setFiles(result || []);

      // Load poster images for videos
      const videos = (result || []).filter(f => f.type === 'video');
      const covers = (result || []).filter(f => f.type === 'cover');
      const newCache: Record<string, string> = {};

      for (const video of videos) {
        const baseName = video.path.replace(/\.(mkv|mp4|webm|avi)$/i, '');
        const posterPath = baseName + '-poster.jpg';
        const poster = covers.find(c => c.path === posterPath);
        if (poster) {
          try {
            const dataUrl = await GetImageAsDataURL(poster.path);
            newCache[video.path] = dataUrl;
          } catch (e) {
            console.error('Failed to load poster:', e);
          }
        }
      }
      setPosterCache(newCache);
    } catch (err) {
      console.error('Failed to load files:', err);
      setFiles([]);
    } finally {
      setLoading(false);
    }
  };

  const handleBrowse = async () => {
    try {
      const dir = await BrowseDirectory();
      if (dir) {
        setPath(dir);
        loadFiles(dir);
      }
    } catch (err) {
      console.error('Failed to browse:', err);
    }
  };

  const handleOpenFile = async (filePath: string) => {
    try {
      await OpenFile(filePath);
    } catch (err) {
      console.error('Failed to open file:', err);
    }
  };

  const handleOpenDirectory = async (filePath: string) => {
    try {
      await OpenDirectory(filePath);
    } catch (err) {
      console.error('Failed to open directory:', err);
    }
  };

  const loadPlaylistFolders = async () => {
    try {
      const folders = await GetPlaylistFolders();
      setPlaylistFolders(folders || []);
    } catch (err) {
      console.error('Failed to load playlist folders:', err);
      setPlaylistFolders([]);
    }
  };

  const handleReorganizePlaylist = async (folder: string) => {
    setReorganizing(folder);
    try {
      const result = await ReorganizePlaylist(folder);
      if (result.renamed > 0) {
        // Reload files to show new names
        loadFiles();
      }
      alert(`Reorganized ${result.renamed} files${result.errors?.length ? `, ${result.errors.length} errors` : ''}`);
    } catch (err) {
      console.error('Failed to reorganize playlist:', err);
      alert('Failed to reorganize playlist');
    } finally {
      setReorganizing(null);
    }
  };

  const handleFlattenPlaylist = async (folder: string) => {
    setFlattening(folder);
    try {
      const result = await FlattenPlaylistFolder(folder);
      if (result.moved > 0) {
        // Reload files to show new structure
        loadFiles();
      }
      alert(`Moved ${result.moved} files to root${result.errors?.length ? `, ${result.errors.length} errors` : ''}`);
    } catch (err) {
      console.error('Failed to flatten playlist:', err);
      alert('Failed to flatten playlist');
    } finally {
      setFlattening(null);
    }
  };

  // Load default directory on mount
  useEffect(() => {
    const init = async () => {
      try {
        const defaultDir = await GetDefaultOutputDirectory();
        setPath(defaultDir);
        loadFiles(defaultDir);
        loadPlaylistFolders();
      } catch (err) {
        console.error('Failed to get default directory:', err);
      }
    };
    init();
  }, []);

  // No need to reload when tab changes - we load all files and filter in frontend

  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: 'videos', label: 'Videos', count: files.filter(f => f.type === 'video').length },
    { id: 'audio', label: 'Audio', count: files.filter(f => f.type === 'audio').length },
    { id: 'covers', label: 'Covers', count: files.filter(f => f.type === 'cover').length },
    { id: 'nfo', label: 'NFO', count: files.filter(f => f.type === 'nfo').length },
  ];

  // Filter files by current tab
  const filteredFiles = files.filter(f => {
    if (activeTab === 'videos') return f.type === 'video';
    if (activeTab === 'audio') return f.type === 'audio';
    if (activeTab === 'covers') return f.type === 'cover';
    if (activeTab === 'nfo') return f.type === 'nfo';
    return true;
  });

  // Get cached poster for a video file
  const getPoster = (videoPath: string): string | null => {
    return posterCache[videoPath] || null;
  };

  const getFileIcon = (type: string) => {
    switch (type) {
      case 'video': return <FileVideoIcon />;
      case 'audio': return <FileAudioIcon />;
      case 'cover': return <ImageIcon />;
      case 'nfo': return <FileTextIcon />;
      default: return <FileVideoIcon />;
    }
  };

  return (
    <div className="min-h-screen">
      <Header title="File Manager" subtitle="Browse and manage your downloads" />

      <div className="px-8 pb-8 space-y-6">
        {/* Path input */}
        <div className="flex gap-3">
          <input
            type="text"
            value={path}
            onChange={(e) => setPath(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && loadFiles()}
            placeholder="Enter path or browse..."
            className="flex-1"
          />
          <button className="btn-secondary flex items-center gap-2" onClick={handleBrowse}>
            <FolderIcon />
            Browse
          </button>
          <button className="btn-icon" title="Refresh" onClick={() => loadFiles()}>
            <RefreshIcon />
          </button>
        </div>

        {/* Playlist Folders */}
        {playlistFolders.length > 0 && (
          <div className="card p-4">
            <div className="flex items-center gap-2 mb-3">
              <MusicIcon />
              <span className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                Playlist Folders
              </span>
              <span className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                ({playlistFolders.length})
              </span>
            </div>
            <div className="space-y-2">
              {playlistFolders.map(folder => {
                const folderName = folder.split('/').pop() || folder;
                const isReorganizing = reorganizing === folder;
                const isFlattening = flattening === folder;
                const isBusy = isReorganizing || isFlattening;
                return (
                  <div
                    key={folder}
                    className="flex items-center justify-between p-3 rounded-lg"
                    style={{ background: 'var(--color-bg-tertiary)' }}
                  >
                    <div className="flex items-center gap-3">
                      <FolderIcon />
                      <span style={{ color: 'var(--color-text-primary)' }}>{folderName}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        className="btn-secondary flex items-center gap-2 text-sm"
                        onClick={() => handleFlattenPlaylist(folder)}
                        disabled={isBusy}
                        title="Move all files to root folder"
                      >
                        {isFlattening ? (
                          <>
                            <div className="animate-spin w-4 h-4 border-2 border-current border-t-transparent rounded-full" />
                            Flattening...
                          </>
                        ) : (
                          'Flatten'
                        )}
                      </button>
                      <button
                        className="btn-secondary flex items-center gap-2 text-sm"
                        onClick={() => handleReorganizePlaylist(folder)}
                        disabled={isBusy}
                      >
                        {isReorganizing ? (
                          <>
                            <div className="animate-spin w-4 h-4 border-2 border-current border-t-transparent rounded-full" />
                            Reorganizing...
                          </>
                        ) : (
                          <>
                            <SortIcon />
                            Reorganize
                          </>
                        )}
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* Tabs */}
        <div className="tabs inline-flex">
          {tabs.map(tab => (
            <button
              key={tab.id}
              className={`tab ${activeTab === tab.id ? 'active' : ''}`}
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.label} ({tab.count})
            </button>
          ))}
        </div>

        {/* File list */}
        {loading ? (
          <div className="card p-12 text-center">
            <div className="animate-spin w-8 h-8 border-2 border-current border-t-transparent rounded-full mx-auto" style={{ color: 'var(--color-accent)' }} />
            <p className="mt-4 text-sm" style={{ color: 'var(--color-text-secondary)' }}>Loading files...</p>
          </div>
        ) : filteredFiles.length === 0 ? (
          <div
            className="card p-12 text-center"
            style={{ borderStyle: 'dashed' }}
          >
            <div
              className="w-20 h-20 rounded-2xl mx-auto mb-4 flex items-center justify-center"
              style={{ background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)' }}
            >
              {getFileIcon(activeTab === 'videos' ? 'video' : activeTab === 'audio' ? 'audio' : activeTab === 'covers' ? 'cover' : 'nfo')}
            </div>
            <p
              className="text-sm font-medium mb-1"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              No files found
            </p>
            <p
              className="text-sm"
              style={{ color: 'var(--color-text-tertiary)' }}
            >
              Browse to a folder containing {activeTab === 'videos' ? 'MKV files' : activeTab === 'audio' ? 'FLAC files' : activeTab === 'covers' ? 'cover images' : 'NFO files'}
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {filteredFiles.map((file, index) => (
              <div
                key={file.path}
                className="card p-4 flex items-center gap-4 hover:border-[var(--color-accent)] transition-colors cursor-pointer group"
                style={{ animationDelay: `${index * 0.05}s` }}
              >
                {file.type === 'video' && getPoster(file.path) ? (
                  <img
                    src={getPoster(file.path)!}
                    alt=""
                    className="w-16 h-16 rounded-xl object-cover flex-shrink-0"
                  />
                ) : (
                  <div
                    className="w-12 h-12 rounded-xl flex items-center justify-center flex-shrink-0"
                    style={{ background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)' }}
                  >
                    {getFileIcon(file.type)}
                  </div>
                )}

                <div className="flex-1 min-w-0">
                  <p className="font-medium truncate" style={{ color: 'var(--color-text-primary)' }}>
                    {file.name}
                  </p>
                  <p className="text-sm truncate" style={{ color: 'var(--color-text-tertiary)' }}>
                    {formatFileSize(file.size)}
                  </p>
                </div>

                <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    className="btn-icon"
                    title="Play/Open"
                    onClick={() => handleOpenFile(file.path)}
                  >
                    <PlayIcon />
                  </button>
                  <button
                    className="btn-icon"
                    title="Open folder"
                    onClick={() => handleOpenDirectory(file.path)}
                  >
                    <FolderOpenIcon />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
