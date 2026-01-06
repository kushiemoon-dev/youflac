import { useState, useEffect } from 'react';
import * as Api from '../../lib/api';
import type { AudioAnalysis } from '../../lib/api';

interface AudioAnalyzerProps {
  filePath: string;
  fileName: string;
  onClose: () => void;
}

const CloseIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <line x1="18" y1="6" x2="6" y2="18" />
    <line x1="6" y1="6" x2="18" y2="18" />
  </svg>
);

const CheckCircleIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" />
    <polyline points="9 12 12 15 16 10" />
  </svg>
);

const WarningIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
    <line x1="12" y1="9" x2="12" y2="13" />
    <line x1="12" y1="17" x2="12.01" y2="17" />
  </svg>
);

const SpectrogramIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M2 12h2M6 8v8M10 5v14M14 8v8M18 10v4M22 12h0" />
  </svg>
);

export function AudioAnalyzer({ filePath, fileName, onClose }: AudioAnalyzerProps) {
  const [analysis, setAnalysis] = useState<AudioAnalysis | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [spectrogramUrl, setSpectrogramUrl] = useState<string | null>(null);
  const [loadingSpectrogram, setLoadingSpectrogram] = useState(false);

  useEffect(() => {
    analyzeFile();
  }, [filePath]);

  const analyzeFile = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await Api.AnalyzeAudio(filePath);
      setAnalysis(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Analysis failed');
    } finally {
      setLoading(false);
    }
  };

  const generateSpectrogram = async () => {
    setLoadingSpectrogram(true);
    try {
      const path = await Api.GenerateSpectrogram(filePath);
      const dataUrl = await Api.GetImageAsDataURL(path);
      setSpectrogramUrl(dataUrl);
    } catch (err) {
      console.error('Failed to generate spectrogram:', err);
    } finally {
      setLoadingSpectrogram(false);
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDuration = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const formatSampleRate = (hz: number) => {
    if (hz >= 1000) {
      return `${(hz / 1000).toFixed(1)} kHz`;
    }
    return `${hz} Hz`;
  };

  const formatBitrate = (bps: number) => {
    if (bps >= 1000000) {
      return `${(bps / 1000000).toFixed(1)} Mbps`;
    }
    if (bps >= 1000) {
      return `${Math.round(bps / 1000)} kbps`;
    }
    return `${bps} bps`;
  };

  const getScoreColor = (score: number) => {
    if (score >= 90) return 'var(--color-success)';
    if (score >= 75) return 'var(--color-accent)';
    if (score >= 50) return 'var(--color-warning)';
    return 'var(--color-error)';
  };

  const getScoreBackground = (score: number) => {
    if (score >= 90) return 'rgba(34, 197, 94, 0.1)';
    if (score >= 75) return 'rgba(var(--accent-rgb), 0.1)';
    if (score >= 50) return 'rgba(251, 191, 36, 0.1)';
    return 'rgba(239, 68, 68, 0.1)';
  };

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50"
      style={{ background: 'rgba(0, 0, 0, 0.7)' }}
      onClick={onClose}
    >
      <div
        className="rounded-xl shadow-2xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        style={{ background: 'var(--color-bg-primary)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between p-4 border-b"
          style={{ borderColor: 'var(--color-border-subtle)' }}
        >
          <div>
            <h2 className="text-lg font-semibold" style={{ color: 'var(--color-text-primary)' }}>
              Audio Quality Analysis
            </h2>
            <p className="text-sm truncate max-w-md" style={{ color: 'var(--color-text-tertiary)' }}>
              {fileName}
            </p>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-opacity-50 transition-colors"
            style={{ color: 'var(--color-text-secondary)' }}
          >
            <CloseIcon />
          </button>
        </div>

        {/* Content */}
        <div className="p-4">
          {loading ? (
            <div className="flex flex-col items-center justify-center py-12">
              <div
                className="w-8 h-8 border-2 border-t-transparent rounded-full animate-spin"
                style={{ borderColor: 'var(--color-accent)', borderTopColor: 'transparent' }}
              />
              <p className="mt-3" style={{ color: 'var(--color-text-secondary)' }}>
                Analyzing audio...
              </p>
            </div>
          ) : error ? (
            <div className="py-12 text-center">
              <p style={{ color: 'var(--color-error)' }}>{error}</p>
              <button
                onClick={analyzeFile}
                className="mt-4 px-4 py-2 rounded-lg"
                style={{ background: 'var(--color-accent)', color: '#000' }}
              >
                Retry
              </button>
            </div>
          ) : analysis ? (
            <div className="space-y-6">
              {/* Quality Score */}
              <div className="flex items-center gap-6">
                <div
                  className="w-24 h-24 rounded-full flex items-center justify-center relative"
                  style={{ background: getScoreBackground(analysis.qualityScore) }}
                >
                  <svg className="w-24 h-24 absolute" viewBox="0 0 100 100">
                    <circle
                      cx="50"
                      cy="50"
                      r="42"
                      fill="none"
                      stroke="var(--color-border-subtle)"
                      strokeWidth="6"
                    />
                    <circle
                      cx="50"
                      cy="50"
                      r="42"
                      fill="none"
                      stroke={getScoreColor(analysis.qualityScore)}
                      strokeWidth="6"
                      strokeLinecap="round"
                      strokeDasharray={`${(analysis.qualityScore / 100) * 264} 264`}
                      transform="rotate(-90 50 50)"
                    />
                  </svg>
                  <span
                    className="text-2xl font-bold"
                    style={{ color: getScoreColor(analysis.qualityScore) }}
                  >
                    {analysis.qualityScore}
                  </span>
                </div>

                <div>
                  <h3
                    className="text-xl font-semibold"
                    style={{ color: getScoreColor(analysis.qualityScore) }}
                  >
                    {analysis.qualityRating}
                  </h3>

                  {/* Quality Badges */}
                  <div className="flex flex-wrap gap-2 mt-2">
                    {analysis.isTrueLossless && !analysis.fakeLossless && (
                      <span
                        className="flex items-center gap-1 px-2 py-1 rounded text-xs font-medium"
                        style={{ background: 'rgba(34, 197, 94, 0.2)', color: 'var(--color-success)' }}
                      >
                        <CheckCircleIcon />
                        True Lossless
                      </span>
                    )}
                    {analysis.fakeLossless && (
                      <span
                        className="flex items-center gap-1 px-2 py-1 rounded text-xs font-medium"
                        style={{ background: 'rgba(251, 191, 36, 0.2)', color: 'var(--color-warning)' }}
                      >
                        <WarningIcon />
                        Possible Fake Lossless
                      </span>
                    )}
                    {analysis.sampleRate > 44100 && analysis.bitsPerSample > 16 && (
                      <span
                        className="px-2 py-1 rounded text-xs font-medium"
                        style={{ background: 'rgba(147, 51, 234, 0.2)', color: '#a855f7' }}
                      >
                        Hi-Res {analysis.bitsPerSample}/{analysis.sampleRate / 1000}
                      </span>
                    )}
                  </div>
                </div>
              </div>

              {/* Audio Info Grid */}
              <div
                className="grid grid-cols-2 md:grid-cols-3 gap-4 p-4 rounded-lg"
                style={{ background: 'var(--color-bg-secondary)' }}
              >
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    Codec
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.codec?.toUpperCase() || 'Unknown'}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    Sample Rate
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.sampleRate ? formatSampleRate(analysis.sampleRate) : 'Unknown'}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    Bit Depth
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.bitsPerSample ? `${analysis.bitsPerSample}-bit` : 'Unknown'}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    Bitrate
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.bitrate ? formatBitrate(analysis.bitrate) : 'Variable'}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    Channels
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.channels === 1 ? 'Mono' : analysis.channels === 2 ? 'Stereo' : `${analysis.channels} ch`}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    Duration
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.duration ? formatDuration(analysis.duration) : 'Unknown'}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    File Size
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.fileSize ? formatFileSize(analysis.fileSize) : 'Unknown'}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wider" style={{ color: 'var(--color-text-tertiary)' }}>
                    Format
                  </p>
                  <p className="font-medium" style={{ color: 'var(--color-text-primary)' }}>
                    {analysis.format || 'Unknown'}
                  </p>
                </div>
              </div>

              {/* Issues */}
              {analysis.issues && analysis.issues.length > 0 && (
                <div>
                  <h4 className="text-sm font-medium mb-2" style={{ color: 'var(--color-text-secondary)' }}>
                    Detected Issues
                  </h4>
                  <ul className="space-y-1">
                    {analysis.issues.map((issue, idx) => (
                      <li
                        key={idx}
                        className="flex items-center gap-2 text-sm"
                        style={{ color: 'var(--color-warning)' }}
                      >
                        <WarningIcon />
                        {issue}
                      </li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Spectrogram */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h4 className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                    Spectrogram
                  </h4>
                  {!spectrogramUrl && (
                    <button
                      onClick={generateSpectrogram}
                      disabled={loadingSpectrogram}
                      className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm transition-colors"
                      style={{
                        background: 'var(--color-bg-tertiary)',
                        color: 'var(--color-text-secondary)',
                      }}
                    >
                      {loadingSpectrogram ? (
                        <>
                          <div className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                          Generating...
                        </>
                      ) : (
                        <>
                          <SpectrogramIcon />
                          Generate
                        </>
                      )}
                    </button>
                  )}
                </div>

                {spectrogramUrl ? (
                  <div className="rounded-lg overflow-hidden" style={{ background: 'var(--color-bg-secondary)' }}>
                    <img
                      src={spectrogramUrl}
                      alt="Audio Spectrogram"
                      className="w-full h-auto"
                    />
                  </div>
                ) : (
                  <div
                    className="rounded-lg p-8 text-center"
                    style={{ background: 'var(--color-bg-secondary)' }}
                  >
                    <p className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                      Click "Generate" to visualize frequency content
                    </p>
                  </div>
                )}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
