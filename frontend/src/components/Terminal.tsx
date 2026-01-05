import { Header } from './layout/Header';

export function Terminal() {
  return (
    <div className="min-h-screen">
      <Header title="Terminal" subtitle="View application logs" />

      <div className="px-8 pb-8">
        <div
          className="card font-mono text-sm p-4 h-[calc(100vh-200px)] overflow-y-auto"
          style={{
            background: 'var(--color-bg-void)',
            color: 'var(--color-text-secondary)'
          }}
        >
          <div className="space-y-1">
            <div style={{ color: 'var(--color-text-tertiary)' }}>
              [YouFLAC] Application started
            </div>
            <div style={{ color: 'var(--color-text-tertiary)' }}>
              [YouFLAC] Queue loaded: 0 items
            </div>
            <div style={{ color: 'var(--color-success)' }}>
              [YouFLAC] Ready for downloads
            </div>
            <div className="flex items-center gap-2">
              <span style={{ color: 'var(--color-accent)' }}>$</span>
              <span
                className="inline-block w-2 h-4 animate-pulse"
                style={{ background: 'var(--color-accent)' }}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
