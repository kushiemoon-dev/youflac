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

const audioQualityOptions = [
  { value: 'highest', label: 'Highest', description: 'Best quality available' },
  { value: '24bit', label: '24-bit', description: 'Hi-Res lossless' },
  { value: '16bit', label: '16-bit', description: 'CD quality lossless' },
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

const logLevelOptions = [
  { value: 'debug', label: 'Debug', description: 'Very verbose, for troubleshooting' },
  { value: 'info', label: 'Info', description: 'Normal operation logs' },
  { value: 'warn', label: 'Warn', description: 'Warnings and errors only' },
  { value: 'error', label: 'Error', description: 'Errors only' },
];

function SectionTitle({ title }: { title: string }) {
  return (
    <div className="mb-4">
      <h3 className="text-sm font-semibold uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
        {title}
      </h3>
      <hr className="mt-2" style={{ borderColor: 'var(--color-border-subtle)' }} />
    </div>
  );
}

function SettingRow({ label, description, children }: { label: string; description?: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
      <div className="flex-1 min-w-0">
        <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>{label}</p>
        {description && (
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-text-tertiary)' }}>{description}</p>
        )}
      </div>
      <div className="flex-shrink-0">
        {children}
      </div>
    </div>
  );
}

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
    window.location.reload();
  };

  const handleChange = <K extends keyof typeof config>(field: K, value: typeof config[K]) => {
    updateField(field, value);
    setHasChanges(true);
  };

  return (
    <div className="min-h-screen pb-24">
      <Header title="Settings" subtitle="Configure your preferences" />

      <div className="px-8 pb-8 max-w-2xl">

        {/* ── Appearance ── */}
        <section className="mb-8">
          <SectionTitle title="Appearance" />
          <div className="space-y-1">
            <SettingRow label="Theme">
              <div style={{ minWidth: 160 }}>
                <Dropdown value={config.theme} options={themeOptions} onChange={(v) => handleChange('theme', v)} />
              </div>
            </SettingRow>
            <SettingRow label="Accent Color">
              <ColorPicker
                value={(config.accentColor || 'pink') as AccentColor}
                onChange={(color) => {
                  handleChange('accentColor', color);
                  applyAccentColor(color);
                }}
              />
            </SettingRow>
          </div>
        </section>

        {/* ── Downloads ── */}
        <section className="mb-8">
          <SectionTitle title="Downloads" />
          <div className="space-y-1">
            <SettingRow label="Output Directory">
              <div className="flex gap-2" style={{ minWidth: 280 }}>
                <input
                  type="text"
                  value={config.outputDirectory || defaultPath}
                  onChange={(e) => handleChange('outputDirectory', e.target.value)}
                  className="flex-1"
                  placeholder={defaultPath}
                />
                <button className="btn-secondary flex items-center gap-1.5">
                  <FolderIcon />
                  Browse
                </button>
              </div>
            </SettingRow>
            <SettingRow label="Video Quality">
              <div style={{ minWidth: 160 }}>
                <Dropdown value={config.videoQuality} options={videoQualityOptions} onChange={(v) => handleChange('videoQuality', v)} />
              </div>
            </SettingRow>
            <SettingRow label="Concurrent Downloads">
              <div className="flex items-center gap-3" style={{ minWidth: 140 }}>
                <input
                  type="range"
                  min="1"
                  max="5"
                  value={config.concurrentDownloads}
                  onChange={(e) => handleChange('concurrentDownloads', parseInt(e.target.value))}
                  className="flex-1"
                  style={{ accentColor: 'var(--color-accent)' }}
                />
                <span className="w-6 text-center font-mono" style={{ color: 'var(--color-text-primary)' }}>
                  {config.concurrentDownloads}
                </span>
              </div>
            </SettingRow>
            <SettingRow label="Folder Structure" description={`Preview: ${config.outputDirectory || defaultPath}/${namingTemplateOptions.find(o => o.value === config.namingTemplate)?.description || '{artist}/{title}'}`}>
              <div style={{ minWidth: 160 }}>
                <Dropdown value={config.namingTemplate} options={namingTemplateOptions} onChange={(v) => handleChange('namingTemplate', v)} />
              </div>
            </SettingRow>
            <SettingRow label="YouTube Cookies (Anti-Bot)" description="Use browser cookies to bypass bot detection">
              <div style={{ minWidth: 160 }}>
                <Dropdown value={config.cookiesBrowser || ''} options={cookiesBrowserOptions} onChange={(v) => handleChange('cookiesBrowser', v)} />
              </div>
            </SettingRow>
          </div>
        </section>

        {/* ── Metadata ── */}
        <section className="mb-8">
          <SectionTitle title="Metadata" />
          <div className="space-y-1">
            <SettingRow label="Generate NFO Files" description="Create metadata files for Jellyfin/Kodi">
              <Toggle checked={config.generateNfo} onChange={(v) => handleChange('generateNfo', v)} />
            </SettingRow>
            <SettingRow label="Embed Cover Art" description="Include album art in MKV files">
              <Toggle checked={config.embedCoverArt} onChange={(v) => handleChange('embedCoverArt', v)} />
            </SettingRow>
            <SettingRow label="Save Cover File" description="Also save album art as a separate .jpg file">
              <Toggle checked={config.saveCoverFile ?? false} onChange={(v) => handleChange('saveCoverFile', v)} />
            </SettingRow>
            <SettingRow label="First Artist Only" description="Strip featured artists from artist tag (e.g. &quot;Artist feat. X&quot; → &quot;Artist&quot;)">
              <Toggle checked={config.firstArtistOnly ?? false} onChange={(v) => handleChange('firstArtistOnly', v)} />
            </SettingRow>
            <SettingRow label="Audio Source Priority" description="Drag to reorder">
              <div className="flex gap-2">
                {(config.audioSourcePriority || ['tidal', 'qobuz', 'amazon']).map((source, index) => (
                  <span key={source} className="badge badge-neutral cursor-move">
                    {index + 1}. {source}
                  </span>
                ))}
              </div>
            </SettingRow>
          </div>
        </section>

        {/* ── Quality ── */}
        <section className="mb-8">
          <SectionTitle title="Quality" />
          <div className="space-y-1">
            <SettingRow label="Preferred Audio Quality" description="Target quality tier for lossless downloads">
              <div style={{ minWidth: 160 }}>
                <Dropdown value={config.preferredQuality || 'highest'} options={audioQualityOptions} onChange={(v) => handleChange('preferredQuality', v)} />
              </div>
            </SettingRow>
            <SettingRow label="Skip Explicit Tracks" description="Skip tracks flagged as explicit content">
              <Toggle checked={config.skipExplicit ?? false} onChange={(v) => handleChange('skipExplicit', v)} />
            </SettingRow>
          </div>
        </section>

        {/* ── Playlist ── */}
        <section className="mb-8">
          <SectionTitle title="Playlist" />
          <div className="space-y-1">
            <SettingRow label="Generate M3U8" description="Create an .m3u8 playlist file when a batch completes">
              <Toggle checked={config.generateM3u8 ?? false} onChange={(v) => handleChange('generateM3u8', v)} />
            </SettingRow>
          </div>
        </section>

        {/* ── Lyrics ── */}
        <section className="mb-8">
          <SectionTitle title="Lyrics" />
          <div className="space-y-1">
            <SettingRow label="Fetch Lyrics" description="Automatically fetch lyrics from LRCLIB">
              <Toggle checked={config.lyricsEnabled ?? false} onChange={(v) => handleChange('lyricsEnabled', v)} />
            </SettingRow>
            {config.lyricsEnabled && (
              <SettingRow label="Lyrics Format">
                <div style={{ minWidth: 160 }}>
                  <Dropdown value={config.lyricsEmbedMode || 'lrc'} options={lyricsEmbedModeOptions} onChange={(v) => handleChange('lyricsEmbedMode', v)} />
                </div>
              </SettingRow>
            )}
          </div>
        </section>

        {/* ── Playback ── */}
        <section className="mb-8">
          <SectionTitle title="Playback" />
          <div className="space-y-1">
            <SettingRow label="Sound Effects" description="Play sounds on download complete, error, etc.">
              <Toggle
                checked={config.soundEffectsEnabled ?? true}
                onChange={(v) => {
                  handleChange('soundEffectsEnabled', v);
                  setSoundEnabled(v);
                }}
              />
            </SettingRow>
            {config.soundEffectsEnabled && (
              <SettingRow label="Sound Volume">
                <div className="flex items-center gap-3" style={{ minWidth: 140 }}>
                  <input
                    type="range"
                    min="0"
                    max="100"
                    value={config.soundVolume ?? 70}
                    onChange={(e) => handleChange('soundVolume', parseInt(e.target.value))}
                    className="flex-1"
                    style={{ accentColor: 'var(--color-accent)' }}
                  />
                  <span className="w-8 text-center font-mono text-sm" style={{ color: 'var(--color-text-primary)' }}>
                    {config.soundVolume ?? 70}%
                  </span>
                </div>
              </SettingRow>
            )}
          </div>
        </section>

        {/* ── Network ── */}
        <section className="mb-8">
          <SectionTitle title="Network" />
          <div className="space-y-1">
            <SettingRow label="Proxy URL" description="HTTP or SOCKS5 proxy for all requests">
              <input
                type="text"
                value={config.proxyUrl || ''}
                onChange={(e) => handleChange('proxyUrl', e.target.value)}
                placeholder="socks5://127.0.0.1:1080"
                style={{ minWidth: 240 }}
              />
            </SettingRow>
          </div>
        </section>

        {/* ── Advanced ── */}
        <section className="mb-8">
          <SectionTitle title="Advanced" />
          <div className="space-y-1">
            <SettingRow label="Download Timeout" description="Per-file timeout in minutes (0 = default 10m)">
              <input
                type="number"
                min="0"
                max="120"
                value={config.downloadTimeoutMinutes ?? 10}
                onChange={(e) => handleChange('downloadTimeoutMinutes', parseFloat(e.target.value) || 0)}
                style={{ width: 80, textAlign: 'right' }}
              />
            </SettingRow>
            <SettingRow label="Log Level" description="Verbosity of application logs">
              <div style={{ minWidth: 140 }}>
                <Dropdown value={config.logLevel || 'info'} options={logLevelOptions} onChange={(v) => handleChange('logLevel', v)} />
              </div>
            </SettingRow>
          </div>
        </section>

      </div>

      {/* Fixed Footer */}
      <div
        className="fixed bottom-0 left-[64px] right-0 p-4 glass"
        style={{ borderTop: '1px solid var(--color-border-subtle)' }}
      >
        <div className="flex items-center justify-between max-w-2xl mx-auto">
          <button className="btn-ghost flex items-center gap-2" onClick={handleReset}>
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
