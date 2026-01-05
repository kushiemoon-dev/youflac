import { useState, useEffect, useCallback } from 'react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import * as App from '../../wailsjs/go/main/App';
import { backend } from '../../wailsjs/go/models';
import type { QueueEvent } from '../types';

export function useQueue() {
  const [items, setItems] = useState<backend.QueueItem[]>([]);
  const [stats, setStats] = useState<backend.QueueStats | null>(null);
  const [loading, setLoading] = useState(true);

  // Fetch initial queue
  const fetchQueue = useCallback(async () => {
    try {
      const [queueItems, queueStats] = await Promise.all([
        App.GetQueue(),
        App.GetQueueStats()
      ]);
      setItems(queueItems || []);
      setStats(queueStats);
    } catch (err) {
      console.error('Failed to fetch queue:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  // Listen for queue events
  useEffect(() => {
    fetchQueue();

    const handleEvent = (event: QueueEvent) => {
      switch (event.type) {
        case 'added':
          if (event.item) {
            setItems(prev => [...prev, event.item!]);
          }
          break;
        case 'updated':
          if (event.item) {
            setItems(prev =>
              prev.map(item => item.id === event.itemId ? event.item! : item)
            );
          }
          break;
        case 'removed':
          setItems(prev => prev.filter(item => item.id !== event.itemId));
          break;
        case 'completed':
        case 'error':
          if (event.item) {
            setItems(prev =>
              prev.map(item => item.id === event.itemId ? event.item! : item)
            );
          }
          break;
      }
      // Refresh stats on any event
      App.GetQueueStats().then(setStats).catch(console.error);
    };

    EventsOn('queue:event', handleEvent);
    return () => EventsOff('queue:event');
  }, [fetchQueue]);

  // Actions
  const addToQueue = useCallback(async (videoUrl: string, spotifyUrl?: string) => {
    const request = new backend.DownloadRequest({
      videoUrl,
      spotifyUrl
    });
    return App.AddToQueue(request);
  }, []);

  const removeFromQueue = useCallback(async (id: string) => {
    await App.RemoveFromQueue(id);
  }, []);

  const cancelItem = useCallback(async (id: string) => {
    await App.CancelQueueItem(id);
  }, []);

  const clearCompleted = useCallback(async () => {
    await App.ClearCompleted();
    fetchQueue();
  }, [fetchQueue]);

  const retryFailed = useCallback(async () => {
    await App.RetryFailed();
    fetchQueue();
  }, [fetchQueue]);

  const clearAll = useCallback(async () => {
    await App.ClearQueue();
    fetchQueue();
  }, [fetchQueue]);

  const moveItem = useCallback(async (id: string, newIndex: number) => {
    await App.MoveQueueItem(id, newIndex);
    fetchQueue();
  }, [fetchQueue]);

  return {
    items,
    stats,
    loading,
    addToQueue,
    removeFromQueue,
    cancelItem,
    clearCompleted,
    retryFailed,
    clearAll,
    moveItem,
    refresh: fetchQueue
  };
}
