import { useState, useEffect, useCallback } from 'react';
import * as Api from '../lib/api';
import type { HistoryEntry, HistoryStats } from '../lib/api';

export function useHistory() {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [stats, setStats] = useState<HistoryStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [sourceFilter, setSourceFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string>('');

  // Fetch history
  const fetchHistory = useCallback(async () => {
    setLoading(true);
    try {
      let historyEntries: HistoryEntry[];

      if (searchQuery) {
        historyEntries = await Api.SearchHistory(searchQuery);
      } else if (sourceFilter) {
        historyEntries = await Api.FilterHistoryBySource(sourceFilter);
      } else if (statusFilter) {
        historyEntries = await Api.FilterHistoryByStatus(statusFilter);
      } else {
        historyEntries = await Api.GetHistory();
      }

      setEntries(historyEntries || []);

      // Also fetch stats
      const historyStats = await Api.GetHistoryStats();
      setStats(historyStats);
    } catch (err) {
      console.error('Failed to fetch history:', err);
    } finally {
      setLoading(false);
    }
  }, [searchQuery, sourceFilter, statusFilter]);

  // Initial fetch
  useEffect(() => {
    fetchHistory();
  }, [fetchHistory]);

  // Search
  const search = useCallback((query: string) => {
    setSearchQuery(query);
    setSourceFilter('');
    setStatusFilter('');
  }, []);

  // Filter by source
  const filterBySource = useCallback((source: string) => {
    setSourceFilter(source);
    setSearchQuery('');
    setStatusFilter('');
  }, []);

  // Filter by status
  const filterByStatus = useCallback((status: string) => {
    setStatusFilter(status);
    setSearchQuery('');
    setSourceFilter('');
  }, []);

  // Clear filters
  const clearFilters = useCallback(() => {
    setSearchQuery('');
    setSourceFilter('');
    setStatusFilter('');
  }, []);

  // Delete entry
  const deleteEntry = useCallback(async (id: string) => {
    try {
      await Api.DeleteHistoryEntry(id);
      setEntries(prev => prev.filter(e => e.id !== id));
      // Update stats
      const historyStats = await Api.GetHistoryStats();
      setStats(historyStats);
    } catch (err) {
      console.error('Failed to delete history entry:', err);
    }
  }, []);

  // Clear all history
  const clearHistory = useCallback(async () => {
    try {
      await Api.ClearHistory();
      setEntries([]);
      const historyStats = await Api.GetHistoryStats();
      setStats(historyStats);
    } catch (err) {
      console.error('Failed to clear history:', err);
    }
  }, []);

  // Redownload from history
  const redownload = useCallback(async (id: string) => {
    try {
      await Api.RedownloadFromHistory(id);
    } catch (err) {
      console.error('Failed to redownload:', err);
      throw err;
    }
  }, []);

  // Group entries by date
  const groupedByDate = useCallback(() => {
    const groups: Record<string, HistoryEntry[]> = {};

    for (const entry of entries) {
      const date = new Date(entry.completedAt).toLocaleDateString('fr-FR', {
        weekday: 'long',
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      });

      if (!groups[date]) {
        groups[date] = [];
      }
      groups[date].push(entry);
    }

    return groups;
  }, [entries]);

  return {
    entries,
    stats,
    loading,
    searchQuery,
    sourceFilter,
    statusFilter,
    search,
    filterBySource,
    filterByStatus,
    clearFilters,
    deleteEntry,
    clearHistory,
    redownload,
    refresh: fetchHistory,
    groupedByDate,
  };
}
