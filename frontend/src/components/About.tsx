import { useState, useEffect } from 'react';
import { Header } from './layout/Header';
import * as App from '../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

const GithubIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
  </svg>
);

const HeartIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
  </svg>
);

export function About() {
  const [version, setVersion] = useState('');

  useEffect(() => {
    App.GetAppVersion().then(setVersion).catch(console.error);
  }, []);

  return (
    <div className="min-h-screen">
      <Header title="About" subtitle="Application information" />

      <div className="px-8 pb-8 max-w-2xl">
        <div className="card p-8 text-center mb-8 animate-slide-up">
          {/* Logo */}
          <div className="mb-6">
            <div
              className="w-20 h-20 rounded-2xl mx-auto flex items-center justify-center relative"
              style={{
                background: 'linear-gradient(135deg, var(--color-accent) 0%, #a855f7 100%)'
              }}
            >
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none">
                <path
                  d="M4 4h16v16H4V4z"
                  fill="none"
                  stroke="#000"
                  strokeWidth="2"
                />
                <path
                  d="M9 8v8l7-4-7-4z"
                  fill="#000"
                />
              </svg>
              {/* Glow */}
              <div
                className="absolute inset-0 rounded-2xl blur-xl opacity-50 -z-10"
                style={{ background: 'var(--color-accent)' }}
              />
            </div>
          </div>

          <h2
            className="text-2xl font-semibold mb-2"
            style={{ color: 'var(--color-text-primary)' }}
          >
            YouFLAC
          </h2>
          <p
            className="mb-4"
            style={{ color: 'var(--color-text-secondary)' }}
          >
            YouTube Video + Lossless FLAC = Perfect MKV
          </p>

          <span className="badge badge-accent mb-6">
            Version {version || '0.1.0'}
          </span>

          <p
            className="text-sm mb-6"
            style={{ color: 'var(--color-text-tertiary)' }}
          >
            Create high-quality music video files by combining YouTube video with lossless FLAC audio from Tidal, Qobuz, or Amazon Music.
          </p>

          {/* Tech stack */}
          <div className="flex flex-wrap justify-center gap-2 mb-6">
            <span className="badge badge-neutral">Go</span>
            <span className="badge badge-neutral">Wails v2</span>
            <span className="badge badge-neutral">React</span>
            <span className="badge badge-neutral">TypeScript</span>
            <span className="badge badge-neutral">FFmpeg</span>
            <span className="badge badge-neutral">yt-dlp</span>
          </div>

          {/* Links */}
          <div className="flex justify-center gap-4">
            <button
              className="btn-secondary flex items-center gap-2"
              onClick={() => BrowserOpenURL('https://github.com/kushie/youflac')}
            >
              <GithubIcon />
              GitHub
            </button>
            <button
              className="btn-primary flex items-center gap-2"
              onClick={() => BrowserOpenURL('https://ko-fi.com/username')}
            >
              <HeartIcon />
              Support
            </button>
          </div>
        </div>

        {/* Credits */}
        <div className="card p-6 animate-slide-up" style={{ animationDelay: '0.1s' }}>
          <h3
            className="font-medium mb-4"
            style={{ color: 'var(--color-text-primary)' }}
          >
            Powered by
          </h3>
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span style={{ color: 'var(--color-text-secondary)' }}>yt-dlp</span>
              <span className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Video downloading</span>
            </div>
            <div className="flex items-center justify-between">
              <span style={{ color: 'var(--color-text-secondary)' }}>FFmpeg</span>
              <span className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Video/audio muxing</span>
            </div>
            <div className="flex items-center justify-between">
              <span style={{ color: 'var(--color-text-secondary)' }}>Wails</span>
              <span className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Desktop framework</span>
            </div>
            <div className="flex items-center justify-between">
              <span style={{ color: 'var(--color-text-secondary)' }}>song.link</span>
              <span className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Audio source discovery</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
