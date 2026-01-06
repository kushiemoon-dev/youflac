import { useState, useEffect } from 'react';
import * as Api from '../../lib/api';

interface HeaderProps {
  title: string;
  subtitle?: string;
}

export function Header({ title, subtitle }: HeaderProps) {
  const [version, setVersion] = useState('');

  useEffect(() => {
    Api.GetAppVersion().then(setVersion).catch(console.error);
  }, []);

  return (
    <header className="flex items-center justify-between py-6 px-8">
      <div className="flex items-center gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1
              className="text-2xl font-semibold tracking-tight"
              style={{ color: 'var(--color-text-primary)' }}
            >
              {title}
            </h1>
            {version && (
              <span className="badge badge-accent">v{version}</span>
            )}
          </div>
          {subtitle && (
            <p
              className="text-sm mt-1"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              {subtitle}
            </p>
          )}
        </div>
      </div>

      {/* Right side - could add notification bell, etc */}
      <div className="flex items-center gap-3">
        {/* Waveform animation indicator */}
        <div className="waveform-bg opacity-60">
          <div className="waveform-bar" />
          <div className="waveform-bar" />
          <div className="waveform-bar" />
          <div className="waveform-bar" />
          <div className="waveform-bar" />
          <div className="waveform-bar" />
        </div>
      </div>
    </header>
  );
}
