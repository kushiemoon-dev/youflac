import { useState, useEffect, useRef } from 'react';
import { Header } from './layout/Header';
import type { LogEntry } from '../lib/api';
import * as Api from '../lib/api';

const LEVEL_COLORS: Record<string, string> = {
  ERROR: 'var(--color-error)',
  WARN: 'var(--color-warning)',
  WARNING: 'var(--color-warning)',
  INFO: 'var(--color-text-secondary)',
  DEBUG: '#a78bfa', // purple
};

function levelColor(level: string): string {
  return LEVEL_COLORS[level.toUpperCase()] ?? 'var(--color-text-secondary)';
}

export function Terminal() {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [lastId, setLastId] = useState(0);
  const bottomRef = useRef<HTMLDivElement>(null);

  // Poll /api/logs every 2 seconds
  useEffect(() => {
    let current = lastId;

    const poll = async () => {
      try {
        const newEntries = await Api.FetchLogs(current);
        if (newEntries && newEntries.length > 0) {
          current = newEntries[newEntries.length - 1].id;
          setLastId(current);
          setEntries((prev) => {
            const combined = [...prev, ...newEntries];
            // Keep max 500 entries in view
            return combined.length > 500 ? combined.slice(combined.length - 500) : combined;
          });
        }
      } catch {
        // Backend unavailable — skip silently
      }
    };

    poll();
    const timer = setInterval(poll, 2000);
    return () => clearInterval(timer);
  }, []);

  // Auto-scroll to bottom when new entries arrive
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [entries]);

  const handleClear = () => setEntries([]);

  return (
    <div className="min-h-screen">
      <Header title="Terminal" subtitle="Application logs" />

      <div className="px-8 pb-8">
        {/* Toolbar */}
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-3">
            {entries.length > 0 && (
              <span className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                {entries.length} entries
              </span>
            )}
          </div>
          <button
            className="btn-ghost text-xs"
            onClick={handleClear}
            style={{ color: 'var(--color-text-tertiary)' }}
          >
            Clear
          </button>
        </div>

        {/* Log pane */}
        <div
          className="card font-mono text-sm p-4 h-[calc(100vh-220px)] overflow-y-auto"
          style={{
            background: 'var(--color-bg-void)',
            color: 'var(--color-text-secondary)',
          }}
        >
          {entries.length === 0 ? (
            <div className="space-y-1">
              <div style={{ color: 'var(--color-text-tertiary)' }}>[YouFLAC] Waiting for logs...</div>
              <div className="flex items-center gap-2">
                <span style={{ color: 'var(--color-accent)' }}>$</span>
                <span
                  className="inline-block w-2 h-4 animate-pulse"
                  style={{ background: 'var(--color-accent)' }}
                />
              </div>
            </div>
          ) : (
            <div className="space-y-0.5">
              {entries.map((entry) => (
                <div key={entry.id} className="flex gap-2 leading-5">
                  <span style={{ color: 'var(--color-text-tertiary)', flexShrink: 0 }}>
                    [{entry.time}]
                  </span>
                  <span
                    className="font-semibold uppercase w-5"
                    style={{ color: levelColor(entry.level), flexShrink: 0 }}
                  >
                    {entry.level.slice(0, 1)}
                  </span>
                  <span style={{ color: levelColor(entry.level), wordBreak: 'break-all' }}>
                    {entry.message}
                  </span>
                </div>
              ))}
              <div ref={bottomRef} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
