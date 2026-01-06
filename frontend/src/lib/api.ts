/**
 * HTTP API Client - Replaces Wails bindings for Docker/standalone mode
 */

// Types (matching backend models)
export interface Config {
  outputDirectory: string;
  videoQuality: string;
  audioSourcePriority: string[];
  namingTemplate: string;
  generateNfo: boolean;
  concurrentDownloads: number;
  embedCoverArt: boolean;
  theme: string;
  cookiesBrowser: string;
  accentColor: string;
  soundEffectsEnabled: boolean;
  lyricsEnabled: boolean;
  lyricsEmbedMode: string;
}

export interface DownloadRequest {
  videoUrl: string;
  spotifyUrl?: string;
  quality?: string;
}

export interface QueueItem {
  id: string;
  videoUrl: string;
  spotifyUrl?: string;
  title: string;
  artist: string;
  album?: string;
  playlistName?: string;
  playlistPosition?: number;
  thumbnail?: string;
  duration?: number;
  status: string;
  progress: number;
  stage: string;
  error?: string;
  outputPath?: string;
  videoPath?: string;
  audioPath?: string;
  fileSize?: number;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  matchScore?: number;
  matchConfidence?: string;
  audioSource?: string;
  quality?: string;
  audioOnly?: boolean;
}

export interface QueueStats {
  total: number;
  pending: number;
  active: number;
  completed: number;
  failed: number;
  cancelled: number;
}

export interface QueueEvent {
  type: string;
  itemId: string;
  item?: QueueItem;
  progress?: number;
  stage?: string;
  error?: string;
}

export interface VideoInfo {
  id: string;
  title: string;
  artist: string;
  album?: string;
  duration: number;
  isrc?: string;
  thumbnail: string;
  url: string;
  uploadDate?: string;
  description?: string;
  channel?: string;
  viewCount?: number;
}

export interface HistoryEntry {
  id: string;
  videoUrl: string;
  title: string;
  artist: string;
  audioSource: string;
  quality: string;
  outputPath: string;
  thumbnail?: string;
  duration?: number;
  fileSize: number;
  completedAt: string;
  status: string;
  error?: string;
}

export interface HistoryStats {
  total: number;
  completed: number;
  failed: number;
  totalSize: number;
  sourceCounts: Record<string, number>;
}

export interface AudioAnalysis {
  filePath: string;
  fileName: string;
  codec: string;
  codecLong: string;
  bitrate: number;
  sampleRate: number;
  bitsPerSample: number;
  channels: number;
  duration: number;
  fileSize: number;
  isTrueLossless: boolean;
  fakeLossless: boolean;
  qualityScore: number;
  qualityRating: string;
  issues?: string[];
  spectrogramPath?: string;
  format: string;
  profile?: string;
  maxFreq?: number;
}

export interface LyricsResult {
  plainText: string;
  syncedLyrics?: string;
  source: string;
  hasSync: boolean;
  trackName?: string;
  artistName?: string;
  albumName?: string;
  duration?: number;
}

export interface MatchResult {
  video?: VideoInfo;
  audio?: AudioCandidate;
  confidence: number;
  matchMethod: string;
  durationDiff: number;
  titleScore: number;
  artistScore: number;
  isValid: boolean;
  warnings?: string[];
}

export interface AudioCandidate {
  platform: string;
  url: string;
  title: string;
  artist: string;
  album?: string;
  isrc?: string;
  duration: number;
  quality?: string;
  priority: number;
}

export interface FileInfo {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  extension: string;
  type: string;  // 'video' | 'audio' | 'cover' | 'nfo' | 'other'
}

export interface ParseURLResult {
  type: string;
  videoId?: string;
  playlistId?: string;
  url: string;
}

export interface ReorganizePlaylistResult {
  success: boolean;
  renamed: number;
  errors?: string[];
  newFolder?: string;
}

export interface FlattenPlaylistResult {
  success: boolean;
  moved: number;
  errors?: string[];
}

// API Base URL - empty for same-origin (production), can be set for dev
const API_BASE = import.meta.env.VITE_API_URL || '';

// Generic fetch helper
async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}/api${path}`, {
    headers: {
      'Content-Type': 'application/json',
    },
    ...options,
  });

  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(errorText || `HTTP ${res.status}`);
  }

  // Handle empty responses
  const text = await res.text();
  if (!text) return {} as T;

  return JSON.parse(text);
}

// ============== Queue API ==============

export async function GetQueue(): Promise<QueueItem[]> {
  return api<QueueItem[]>('/queue');
}

export async function AddToQueue(request: DownloadRequest): Promise<string> {
  const res = await api<{ id: string }>('/queue', {
    method: 'POST',
    body: JSON.stringify(request),
  });
  return res.id;
}

export async function GetQueueItem(id: string): Promise<QueueItem> {
  return api<QueueItem>(`/queue/${id}`);
}

export async function RemoveFromQueue(id: string): Promise<void> {
  await api<void>(`/queue/${id}`, { method: 'DELETE' });
}

export async function CancelQueueItem(id: string): Promise<void> {
  await api<void>(`/queue/${id}/cancel`, { method: 'POST' });
}

export async function MoveQueueItem(id: string, newPosition: number): Promise<void> {
  await api<void>(`/queue/${id}/move`, {
    method: 'PUT',
    body: JSON.stringify({ newPosition }),
  });
}

export async function GetQueueStats(): Promise<QueueStats> {
  return api<QueueStats>('/queue/stats');
}

export async function ClearCompleted(): Promise<number> {
  const res = await api<{ cleared: number }>('/queue/clear', { method: 'POST' });
  return res.cleared;
}

export async function RetryFailed(): Promise<number> {
  const res = await api<{ retried: number }>('/queue/retry', { method: 'POST' });
  return res.retried;
}

export async function ClearQueue(): Promise<void> {
  await api<void>('/queue/clear', { method: 'POST' });
}

export async function SaveQueue(): Promise<void> {
  // No-op for HTTP API - queue is auto-saved
}

// ============== Playlist API ==============

export async function AddPlaylistToQueue(url: string, quality?: string): Promise<string[]> {
  const res = await api<{ ids: string[]; playlistTitle: string }>('/playlist', {
    method: 'POST',
    body: JSON.stringify({ url, quality }),
  });
  return res.ids;
}

// ============== Config API ==============

export async function GetConfig(): Promise<Config> {
  return api<Config>('/config');
}

export async function SaveConfig(config: Config): Promise<void> {
  await api<void>('/config', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

export async function GetDefaultOutputDirectory(): Promise<string> {
  const res = await api<{ path: string }>('/config/default-output');
  return res.path;
}

// ============== History API ==============

export async function GetHistory(): Promise<HistoryEntry[]> {
  return api<HistoryEntry[]>('/history');
}

export async function GetHistoryStats(): Promise<HistoryStats> {
  return api<HistoryStats>('/history/stats');
}

export async function SearchHistory(query: string): Promise<HistoryEntry[]> {
  return api<HistoryEntry[]>(`/history/search?q=${encodeURIComponent(query)}`);
}

export async function DeleteHistoryEntry(id: string): Promise<void> {
  await api<void>(`/history/${id}`, { method: 'DELETE' });
}

export async function ClearHistory(): Promise<void> {
  await api<void>('/history/clear', { method: 'POST' });
}

export async function RedownloadFromHistory(id: string): Promise<string> {
  const res = await api<{ id: string }>(`/history/${id}/redownload`, { method: 'POST' });
  return res.id;
}

export async function FilterHistoryBySource(source: string): Promise<HistoryEntry[]> {
  return api<HistoryEntry[]>(`/history/search?source=${encodeURIComponent(source)}`);
}

export async function FilterHistoryByStatus(status: string): Promise<HistoryEntry[]> {
  return api<HistoryEntry[]>(`/history/search?status=${encodeURIComponent(status)}`);
}

// ============== Video/URL API ==============

export async function ParseURL(url: string): Promise<ParseURLResult> {
  return api<ParseURLResult>('/video/parse', {
    method: 'POST',
    body: JSON.stringify({ url }),
  });
}

export async function GetVideoInfo(url: string): Promise<VideoInfo> {
  return api<VideoInfo>(`/video/info?url=${encodeURIComponent(url)}`);
}

export async function FindAudioMatch(videoInfo: VideoInfo): Promise<MatchResult> {
  return api<MatchResult>('/video/match', {
    method: 'POST',
    body: JSON.stringify(videoInfo),
  });
}

// ============== Files API ==============

export async function ListFiles(dir: string, filter?: string): Promise<FileInfo[]> {
  let url = `/files?dir=${encodeURIComponent(dir)}`;
  if (filter) url += `&filter=${encodeURIComponent(filter)}`;
  return api<FileInfo[]>(url);
}

export async function GetPlaylistFolders(): Promise<string[]> {
  return api<string[]>('/files/playlists');
}

export async function ReorganizePlaylist(folderPath: string): Promise<ReorganizePlaylistResult> {
  return api<ReorganizePlaylistResult>('/files/reorganize', {
    method: 'POST',
    body: JSON.stringify({ folderPath }),
  });
}

export async function FlattenPlaylistFolder(folderPath: string): Promise<FlattenPlaylistResult> {
  return api<FlattenPlaylistResult>('/files/flatten', {
    method: 'POST',
    body: JSON.stringify({ folderPath }),
  });
}

// ============== Analyzer API ==============

export async function AnalyzeAudio(filePath: string): Promise<AudioAnalysis> {
  return api<AudioAnalysis>('/analyze', {
    method: 'POST',
    body: JSON.stringify({ filePath }),
  });
}

export async function GenerateSpectrogram(filePath: string): Promise<string> {
  const res = await api<{ path: string }>('/analyze/spectrogram', {
    method: 'POST',
    body: JSON.stringify({ filePath }),
  });
  return res.path;
}

export async function GenerateWaveform(filePath: string): Promise<string> {
  const res = await api<{ path: string }>('/analyze/waveform', {
    method: 'POST',
    body: JSON.stringify({ filePath }),
  });
  return res.path;
}

// ============== Lyrics API ==============

export async function FetchLyrics(artist: string, title: string): Promise<LyricsResult> {
  return api<LyricsResult>(`/lyrics?artist=${encodeURIComponent(artist)}&title=${encodeURIComponent(title)}`);
}

export async function FetchLyricsWithAlbum(artist: string, title: string, album: string): Promise<LyricsResult> {
  return api<LyricsResult>(`/lyrics?artist=${encodeURIComponent(artist)}&title=${encodeURIComponent(title)}&album=${encodeURIComponent(album)}`);
}

export async function EmbedLyrics(mediaPath: string, lyrics: LyricsResult): Promise<void> {
  await api<void>('/lyrics/embed', {
    method: 'POST',
    body: JSON.stringify({ mediaPath, lyrics }),
  });
}

export async function SaveLRCFile(mediaPath: string, lyrics: LyricsResult): Promise<string> {
  const res = await api<{ path: string }>('/lyrics/save', {
    method: 'POST',
    body: JSON.stringify({ mediaPath, lyrics }),
  });
  return res.path;
}

export async function HasLyrics(mediaPath: string): Promise<boolean> {
  // For HTTP API, this would need a separate endpoint - simplified for now
  return false;
}

export async function ExtractLyrics(mediaPath: string): Promise<LyricsResult> {
  // Would need a separate endpoint
  throw new Error('Not implemented in HTTP API');
}

export async function FetchAndEmbedLyrics(mediaPath: string, artist: string, title: string, mode: string): Promise<void> {
  const lyrics = await FetchLyrics(artist, title);
  if (mode === 'embed' || mode === 'both') {
    await EmbedLyrics(mediaPath, lyrics);
  }
  if (mode === 'lrc' || mode === 'both') {
    await SaveLRCFile(mediaPath, lyrics);
  }
}

// ============== Image API ==============

export async function GetImageAsDataURL(path: string): Promise<string> {
  const res = await api<{ dataUrl: string }>(`/image?path=${encodeURIComponent(path)}`);
  return res.dataUrl;
}

// ============== Misc API ==============

export async function GetAppVersion(): Promise<string> {
  const res = await api<{ version: string }>('/version');
  return res.version;
}

// ============== Desktop-specific stubs ==============
// These functions are no-ops or simplified for HTTP mode

export async function BrowseDirectory(): Promise<string> {
  // In browser, we can't open native file dialogs
  // Return empty - UI should show a text input instead
  return '';
}

export async function OpenDirectory(path: string): Promise<void> {
  // Can't open file manager from browser
  console.log('OpenDirectory not available in browser mode:', path);
}

export async function OpenFile(path: string): Promise<void> {
  // Can't open files directly from browser
  console.log('OpenFile not available in browser mode:', path);
}

// Wails runtime compatibility - these are no-ops in HTTP mode
export function BrowserOpenURL(url: string): void {
  window.open(url, '_blank');
}
