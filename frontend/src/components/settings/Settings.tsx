import { useState, useEffect } from 'react';
import { Header } from '../layout/Header';
import { Toggle } from '../ui/Toggle';
import { Dropdown } from '../ui/Dropdown';
import { ColorPicker } from '../ui/ColorPicker';
import { useSettings } from '../../hooks/useSettings';
import { applyAccentColor } from '../../hooks/useAccentColor';
import { setSoundEnabled } from '../../hooks/useSoundEffects';
import { AccentColor } from '../../types';
import * as Api from '../../lib/api';

const FolderIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
  </svg>
);

const SaveIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" />
    <polyline points="17 21 17 13 7 13 7 21" />
    <polyline points="7 3 7 8 15 8" />
  </svg>
);

const RefreshIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="23 4 23 10 17 10" />
    <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
  </svg>
);

const videoQualityOptions = [
  { value: 'best', label: 'Best Available', description: 'Up to 4K if available' },
  { value: '1080p', label: '1080p', description: 'Full HD' },
  { value: '720p', label: '720p', description: 'HD' },
  { value: '480p', label: '480p', description: 'SD' },
];

const namingTemplateOptions = [
  { value: 'jellyfin', label: 'Jellyfin', description: '{artist}/{title}/{title}' },
  { value: 'plex', label: 'Plex', description: '{artist} - {title}' },
  { value: 'flat', label: 'Flat', description: '{artist} - {title}' },
  { value: 'album', label: 'Album', description: '{artist}/{album}/{title}' },
  { value: 'year', label: 'Year', description: '{year}/{artist}/{title}' },
];

const themeOptions = [
  { value: 'system', label: 'System', description: 'Follow system preference' },
  { value: 'dark', label: 'Dark', description: 'Always use dark mode' },
  { value: 'light', label: 'Light', description: 'Always use light mode' },
];

const cookiesBrowserOptions = [
  { value: '', label: 'None', description: 'No browser cookies' },
  { value: 'librewolf', label: 'Librewolf', description: 'Use Librewolf cookies' },
  { value: 'firefox', label: 'Firefox', description: 'Use Firefox cookies' },
  { value: 'chrome', label: 'Chrome', description: 'Use Chrome cookies' },
  { value: 'chromium', label: 'Chromium', description: 'Use Chromium cookies' },
  { value: 'brave', label: 'Brave', description: 'Use Brave cookies' },
  { value: 'opera', label: 'Opera', description: 'Use Opera cookies' },
  { value: 'edge', label: 'Edge', description: 'Use Edge cookies' },
];

const lyricsEmbedModeOptions = [
  { value: 'lrc', label: 'LRC File', description: 'Save as .lrc file alongside media' },
  { value: 'embed', label: 'Embed', description: 'Embed lyrics in media file' },
  { value: 'both', label: 'Both', description: 'LRC file + embedded' },
];

export function Settings() {
  const { config, loading, saving, saveConfig, updateField } = useSettings();
  const [defaultPath, setDefaultPath] = useState('');
  const [hasChanges, setHasChanges] = useState(false);

  useEffect(() => {
    Api.GetDefaultOutputDirectory().then(setDefaultPath).catch(console.error);
  }, []);

  if (loading || !config) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin w-8 h-8 border-2 border-current border-t-transparent rounded-full mx-auto mb-4" style={{ color: 'var(--color-accent)' }} />
          <p style={{ color: 'var(--color-text-secondary)' }}>Loading settings...</p>
        </div>
      </div>
    );
  }

  const handleSave = async () => {
    await saveConfig(config);
    setHasChanges(false);
  };

  const handleReset = async () => {
    // Reset to defaults by reloading
    window.location.reload();
  };

  const handleChange = <K extends keyof typeof config>(field: K, value: typeof config[K]) => {
    updateField(field, value);
    setHasChanges(true);
  };

  return (
    <div className="min-h-screen pb-24">
      <Header title="Settings" subtitle="Configure your preferences" />

      <div className="px-8 pb-8">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 max-w-5xl">
          {/* Left Column - Download Settings */}
          <div className="space-y-6">
            <h3 className="text-lg font-medium mb-4" style={{ color: 'var(--color-text-primary)' }}>
              Download Settings
            </h3>

            {/* Output Directory */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                Output Directory
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={config.outputDirectory || defaultPath}
                  onChange={(e) => handleChange('outputDirectory', e.target.value)}
                  className="flex-1"
                  placeholder={defaultPath}
                />
                <button className="btn-secondary flex items-center gap-2">
                  <FolderIcon />
                  Browse
                </button>
              </div>
            </div>

            {/* Video Quality */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                Video Quality
              </label>
              <Dropdown
                value={config.videoQuality}
                options={videoQualityOptions}
                onChange={(v) => handleChange('videoQuality', v)}
              />
            </div>

            {/* Concurrent Downloads */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                Concurrent Downloads
              </label>
              <div className="flex items-center gap-4">
                <input
                  type="range"
                  min="1"
                  max="5"
                  value={config.concurrentDownloads}
                  onChange={(e) => handleChange('concurrentDownloads', parseInt(e.target.value))}
                  className="flex-1 accent-pink-500"
                  style={{ accentColor: 'var(--color-accent)' }}
                />
                <span
                  className="w-8 text-center font-mono"
                  style={{ color: 'var(--color-text-primary)' }}
                >
                  {config.concurrentDownloads}
                </span>
              </div>
            </div>

            {/* Naming Template */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                Folder Structure
              </label>
              <Dropdown
                value={config.namingTemplate}
                options={namingTemplateOptions}
                onChange={(v) => handleChange('namingTemplate', v)}
              />
              <p className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                Preview: {config.outputDirectory || defaultPath}/{namingTemplateOptions.find(o => o.value === config.namingTemplate)?.description || '{artist}/{title}'}
              </p>
            </div>

            {/* Cookies Browser */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                YouTube Cookies (Anti-Bot)
              </label>
              <Dropdown
                value={config.cookiesBrowser || ''}
                options={cookiesBrowserOptions}
                onChange={(v) => handleChange('cookiesBrowser', v)}
              />
              <p className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                Use browser cookies to bypass YouTube bot detection
              </p>
            </div>
          </div>

          {/* Right Column - Preferences */}
          <div className="space-y-6">
            <h3 className="text-lg font-medium mb-4" style={{ color: 'var(--color-text-primary)' }}>
              Preferences
            </h3>

            {/* Theme */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                Theme
              </label>
              <Dropdown
                value={config.theme}
                options={themeOptions}
                onChange={(v) => handleChange('theme', v)}
              />
            </div>

            {/* Accent Color */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                Accent Color
              </label>
              <ColorPicker
                value={(config.accentColor || 'pink') as AccentColor}
                onChange={(color) => {
                  handleChange('accentColor', color);
                  applyAccentColor(color);
                }}
              />
            </div>

            {/* Audio Source Priority */}
            <div className="space-y-2">
              <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                Audio Source Priority
              </label>
              <div className="flex gap-2">
                {(config.audioSourcePriority || ['tidal', 'qobuz', 'amazon']).map((source, index) => (
                  <span
                    key={source}
                    className="badge badge-neutral cursor-move"
                    draggable
                  >
                    {index + 1}. {source}
                  </span>
                ))}
              </div>
              <p className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                Drag to reorder priority
              </p>
            </div>

            {/* Toggle Options */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    Generate NFO Files
                  </p>
                  <p className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                    Create metadata files for Jellyfin/Kodi
                  </p>
                </div>
                <Toggle
                  checked={config.generateNfo}
                  onChange={(v) => handleChange('generateNfo', v)}
                />
              </div>

              <div className="flex items-center justify-between">
                <div>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    Embed Cover Art
                  </p>
                  <p className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                    Include album art in MKV files
                  </p>
                </div>
                <Toggle
                  checked={config.embedCoverArt}
                  onChange={(v) => handleChange('embedCoverArt', v)}
                />
              </div>

              <div className="flex items-center justify-between">
                <div>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    Sound Effects
                  </p>
                  <p className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                    Play sounds on download complete, error, etc.
                  </p>
                </div>
                <Toggle
                  checked={config.soundEffectsEnabled ?? true}
                  onChange={(v) => {
                    handleChange('soundEffectsEnabled', v);
                    setSoundEnabled(v);
                  }}
                />
              </div>
            </div>

            {/* Lyrics Section */}
            <div className="pt-4 border-t" style={{ borderColor: 'var(--color-border-subtle)' }}>
              <h4 className="text-sm font-medium mb-4" style={{ color: 'var(--color-text-secondary)' }}>
                Lyrics
              </h4>

              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                      Fetch Lyrics
                    </p>
                    <p className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                      Automatically fetch lyrics from LRCLIB
                    </p>
                  </div>
                  <Toggle
                    checked={config.lyricsEnabled ?? false}
                    onChange={(v) => handleChange('lyricsEnabled', v)}
                  />
                </div>

                {config.lyricsEnabled && (
                  <div className="space-y-2">
                    <label className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                      Lyrics Format
                    </label>
                    <Dropdown
                      value={config.lyricsEmbedMode || 'lrc'}
                      options={lyricsEmbedModeOptions}
                      onChange={(v) => handleChange('lyricsEmbedMode', v)}
                    />
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Fixed Footer */}
      <div
        className="fixed bottom-0 left-[64px] right-0 p-4 glass"
        style={{ borderTop: '1px solid var(--color-border-subtle)' }}
      >
        <div className="flex items-center justify-between max-w-5xl mx-auto">
          <button
            className="btn-ghost flex items-center gap-2"
            onClick={handleReset}
          >
            <RefreshIcon />
            Reset to Defaults
          </button>
          <button
            className={`btn-primary flex items-center gap-2 ${!hasChanges ? 'opacity-50' : ''}`}
            onClick={handleSave}
            disabled={!hasChanges || saving}
          >
            <SaveIcon />
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>
    </div>
  );
}
