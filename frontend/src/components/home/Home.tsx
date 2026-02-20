import { Header } from '../layout/Header';
import { URLInput } from './URLInput';
import { QueueList } from '../queue/QueueList';
import { useQueue } from '../../hooks/useQueue';

export function Home() {
  const {
    items,
    stats,
    addToQueue,
    cancelItem,
    removeFromQueue,
    clearCompleted,
    retryFailed,
    clearAll,
    pauseAll,
    resumeAll,
  } = useQueue();

  const handleAdd = async (videoUrl: string, spotifyUrl?: string) => {
    await addToQueue(videoUrl, spotifyUrl);
  };

  return (
    <div className="min-h-screen">
      <Header
        title="YouFLAC"
        subtitle="YouTube Video + Lossless FLAC = Perfect MKV"
      />

      <div className="px-8 pb-8 space-y-8">
        {/* URL Input Section */}
        <section className="animate-slide-up" style={{ animationDelay: '0.1s', animationFillMode: 'backwards' }}>
          <URLInput onAdd={handleAdd} />
        </section>

        {/* Queue Section */}
        <section className="animate-slide-up" style={{ animationDelay: '0.2s', animationFillMode: 'backwards' }}>
          <QueueList
            items={items}
            stats={stats}
            onCancel={cancelItem}
            onRemove={removeFromQueue}
            onClearCompleted={clearCompleted}
            onRetryFailed={retryFailed}
            onClearAll={clearAll}
            onPauseAll={pauseAll}
            onResumeAll={resumeAll}
          />
        </section>

        {/* Stats bar */}
        {stats && stats.total > 0 && (
          <section
            className="fixed bottom-0 left-[64px] right-0 p-4 glass animate-slide-up"
            style={{ animationDelay: '0.3s' }}
          >
            <div className="flex items-center justify-between max-w-4xl mx-auto">
              <div className="flex items-center gap-6 text-sm">
                <span style={{ color: 'var(--color-text-secondary)' }}>
                  <strong style={{ color: 'var(--color-text-primary)' }}>{stats.total}</strong> total
                </span>
                {stats.active > 0 && (
                  <span className="flex items-center gap-2">
                    <span
                      className="w-2 h-2 rounded-full animate-pulse"
                      style={{ background: 'var(--color-accent)' }}
                    />
                    <strong style={{ color: 'var(--color-accent)' }}>{stats.active}</strong>
                    <span style={{ color: 'var(--color-text-secondary)' }}>processing</span>
                  </span>
                )}
                {stats.completed > 0 && (
                  <span style={{ color: 'var(--color-success)' }}>
                    <strong>{stats.completed}</strong> completed
                  </span>
                )}
                {stats.failed > 0 && (
                  <span style={{ color: 'var(--color-error)' }}>
                    <strong>{stats.failed}</strong> failed
                  </span>
                )}
              </div>
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
