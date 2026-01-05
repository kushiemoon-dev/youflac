import type { QueueStatus } from '../../types';

interface ProgressBarProps {
  progress: number;
  status: QueueStatus;
  stage?: string;
  showStages?: boolean;
}

// Map status to stage progress (each stage is 0-100, overall is 0-100)
function getStageProgress(status: QueueStatus, progress: number): {
  video: number;
  audio: number;
  mux: number;
} {
  switch (status) {
    case 'pending':
      return { video: 0, audio: 0, mux: 0 };
    case 'fetching_info':
      return { video: progress * 0.5, audio: 0, mux: 0 };
    case 'downloading_video':
      return { video: 50 + progress * 0.5, audio: 0, mux: 0 };
    case 'downloading_audio':
      return { video: 100, audio: progress, mux: 0 };
    case 'muxing':
      return { video: 100, audio: 100, mux: progress };
    case 'organizing':
      return { video: 100, audio: 100, mux: 100 };
    case 'complete':
      return { video: 100, audio: 100, mux: 100 };
    case 'error':
    case 'cancelled':
      return { video: progress > 33 ? 100 : progress * 3, audio: progress > 66 ? 100 : Math.max(0, (progress - 33) * 3), mux: Math.max(0, (progress - 66) * 3) };
    default:
      return { video: 0, audio: 0, mux: 0 };
  }
}

function getStatusColor(status: QueueStatus): string {
  switch (status) {
    case 'complete':
      return 'var(--color-success)';
    case 'error':
      return 'var(--color-error)';
    case 'cancelled':
      return 'var(--color-text-tertiary)';
    default:
      return 'var(--color-accent)';
  }
}

export function ProgressBar({ progress, status, stage, showStages = false }: ProgressBarProps) {
  const stages = getStageProgress(status, progress);
  const isActive = !['complete', 'error', 'cancelled', 'pending'].includes(status);

  if (showStages) {
    return (
      <div className="space-y-2">
        <div className="progress-stages">
          {/* Video stage */}
          <div className="progress-stage" title="Video">
            <div
              className="progress-stage-fill"
              style={{
                width: `${stages.video}%`,
                background: status === 'error' ? 'var(--color-error)' : 'var(--color-stage-video)'
              }}
            />
          </div>
          {/* Audio stage */}
          <div className="progress-stage" title="Audio">
            <div
              className="progress-stage-fill"
              style={{
                width: `${stages.audio}%`,
                background: status === 'error' ? 'var(--color-error)' : 'var(--color-stage-audio)'
              }}
            />
          </div>
          {/* Mux stage */}
          <div className="progress-stage" title="Mux">
            <div
              className="progress-stage-fill"
              style={{
                width: `${stages.mux}%`,
                background: status === 'error' ? 'var(--color-error)' : 'var(--color-stage-mux)'
              }}
            />
          </div>
        </div>
        {stage && (
          <div
            className="text-xs flex items-center gap-2"
            style={{ color: 'var(--color-text-secondary)' }}
          >
            {isActive && (
              <span
                className="inline-block w-1.5 h-1.5 rounded-full animate-pulse"
                style={{ background: 'var(--color-accent)' }}
              />
            )}
            {stage}
          </div>
        )}
      </div>
    );
  }

  // Simple single bar
  return (
    <div className="progress-track">
      <div
        className="progress-fill"
        style={{
          width: `${progress}%`,
          background: getStatusColor(status),
          transition: isActive ? 'width 0.3s ease' : 'none'
        }}
      />
    </div>
  );
}
