import { useState } from 'react';
import { Header } from '../layout/Header';
import { useHistory } from '../../hooks/useHistory';
import { HistoryItem } from './HistoryItem';

const SearchIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="11" cy="11" r="8" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </svg>
);

const FilterIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
  </svg>
);

const TrashIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="3 6 5 6 21 6" />
    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
  </svg>
);

const sourceLabels: Record<string, string> = {
  tidal: 'Tidal',
  qobuz: 'Qobuz',
  amazon: 'Amazon',
  deezer: 'Deezer',
  extracted: 'Extracted',
  'tidal-search': 'Tidal Search',
};

const sourceColors: Record<string, string> = {
  tidal: '#00FFFF',
  qobuz: '#4169E1',
  amazon: '#FF9900',
  deezer: '#A238FF',
  extracted: '#808080',
  'tidal-search': '#00FFFF',
};

export function History() {
  const {
    entries,
    stats,
    loading,
    searchQuery,
    sourceFilter,
    search,
    filterBySource,
    clearFilters,
    deleteEntry,
    clearHistory,
    redownload,
    groupedByDate,
  } = useHistory();

  const [searchInput, setSearchInput] = useState('');
  const [showClearConfirm, setShowClearConfirm] = useState(false);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    search(searchInput);
  };

  const handleClearHistory = async () => {
    await clearHistory();
    setShowClearConfirm(false);
  };

  const grouped = groupedByDate();
  const dateKeys = Object.keys(grouped);

  // Format file size
  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  return (
    <div className="min-h-screen pb-24">
      <Header title="History" subtitle="Your download history" />

      <div className="px-8 pb-8">
        {/* Search and Filters */}
        <div className="flex flex-wrap items-center gap-4 mb-6">
          {/* Search */}
          <form onSubmit={handleSearch} className="flex-1 min-w-[200px] max-w-md">
            <div className="relative">
              <input
                type="text"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Search by title or artist..."
                className="w-full pl-10 pr-4 py-2 rounded-lg"
                style={{
                  background: 'var(--color-bg-secondary)',
                  color: 'var(--color-text-primary)',
                  border: '1px solid var(--color-border-subtle)',
                }}
              />
              <div className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: 'var(--color-text-tertiary)' }}>
                <SearchIcon />
              </div>
            </div>
          </form>

          {/* Source Filter */}
          <div className="flex items-center gap-2">
            <FilterIcon />
            <select
              value={sourceFilter}
              onChange={(e) => filterBySource(e.target.value)}
              className="px-3 py-2 rounded-lg"
              style={{
                background: 'var(--color-bg-secondary)',
                color: 'var(--color-text-primary)',
                border: '1px solid var(--color-border-subtle)',
              }}
            >
              <option value="">All Sources</option>
              {stats?.sourceCounts && Object.keys(stats.sourceCounts).map((source) => (
                <option key={source} value={source}>
                  {sourceLabels[source] || source} ({stats.sourceCounts[source]})
                </option>
              ))}
            </select>
          </div>

          {/* Clear Filter Button */}
          {(searchQuery || sourceFilter) && (
            <button
              onClick={clearFilters}
              className="btn-ghost text-sm"
            >
              Clear Filters
            </button>
          )}

          {/* Clear All Button */}
          {entries.length > 0 && (
            <button
              onClick={() => setShowClearConfirm(true)}
              className="btn-ghost text-sm flex items-center gap-2"
              style={{ color: 'var(--color-error)' }}
            >
              <TrashIcon />
              Clear All
            </button>
          )}
        </div>

        {/* Stats Bar */}
        {stats && stats.total > 0 && (
          <div
            className="flex items-center gap-6 mb-6 p-4 rounded-lg"
            style={{ background: 'var(--color-bg-secondary)' }}
          >
            <span style={{ color: 'var(--color-text-secondary)' }}>
              <strong style={{ color: 'var(--color-text-primary)' }}>{stats.total}</strong> total
            </span>
            <span style={{ color: 'var(--color-success)' }}>
              <strong>{stats.completed}</strong> completed
            </span>
            {stats.failed > 0 && (
              <span style={{ color: 'var(--color-error)' }}>
                <strong>{stats.failed}</strong> failed
              </span>
            )}
            <span style={{ color: 'var(--color-text-tertiary)' }}>
              {formatSize(stats.totalSize)} total size
            </span>
          </div>
        )}

        {/* Loading State */}
        {loading && (
          <div className="flex items-center justify-center py-12">
            <div
              className="animate-spin w-8 h-8 border-2 border-current border-t-transparent rounded-full"
              style={{ color: 'var(--color-accent)' }}
            />
          </div>
        )}

        {/* Empty State */}
        {!loading && entries.length === 0 && (
          <div className="text-center py-12">
            <div
              className="w-16 h-16 mx-auto mb-4 rounded-full flex items-center justify-center"
              style={{ background: 'var(--color-bg-secondary)' }}
            >
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--color-text-tertiary)' }}>
                <path d="M12 8v4l3 3" />
                <circle cx="12" cy="12" r="10" />
              </svg>
            </div>
            <h3 className="text-lg font-medium mb-2" style={{ color: 'var(--color-text-primary)' }}>
              No history yet
            </h3>
            <p style={{ color: 'var(--color-text-tertiary)' }}>
              {searchQuery || sourceFilter
                ? 'No results match your filters'
                : 'Your completed downloads will appear here'}
            </p>
          </div>
        )}

        {/* History List Grouped by Date */}
        {!loading && dateKeys.length > 0 && (
          <div className="space-y-6">
            {dateKeys.map((date) => (
              <div key={date}>
                <h3
                  className="text-sm font-medium mb-3 sticky top-0 py-2"
                  style={{
                    color: 'var(--color-text-secondary)',
                    background: 'var(--color-bg-primary)',
                  }}
                >
                  {date}
                </h3>
                <div className="space-y-2">
                  {grouped[date].map((entry) => (
                    <HistoryItem
                      key={entry.id}
                      entry={entry}
                      onDelete={deleteEntry}
                      onRedownload={redownload}
                      sourceColor={sourceColors[entry.audioSource] || '#808080'}
                      sourceLabel={sourceLabels[entry.audioSource] || entry.audioSource}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Clear Confirmation Modal */}
      {showClearConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div
            className="p-6 rounded-lg max-w-sm w-full mx-4"
            style={{ background: 'var(--color-bg-primary)' }}
          >
            <h3 className="text-lg font-medium mb-2" style={{ color: 'var(--color-text-primary)' }}>
              Clear History?
            </h3>
            <p className="mb-6" style={{ color: 'var(--color-text-secondary)' }}>
              This will permanently delete all {stats?.total} history entries. This action cannot be undone.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowClearConfirm(false)}
                className="btn-ghost"
              >
                Cancel
              </button>
              <button
                onClick={handleClearHistory}
                className="btn-primary"
                style={{ background: 'var(--color-error)' }}
              >
                Clear All
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
