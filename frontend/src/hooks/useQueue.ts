import { useState, useEffect, useCallback } from 'react';
import { EventsOn } from '../lib/websocket';
import * as Api from '../lib/api';
import type { QueueItem, QueueStats, QueueEvent } from '../lib/api';
import { playSound } from './useSoundEffects';

export function useQueue() {
  const [items, setItems] = useState<QueueItem[]>([]);
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [loading, setLoading] = useState(true);

  // Fetch initial queue
  const fetchQueue = useCallback(async () => {
    try {
      const [queueItems, queueStats] = await Promise.all([
        Api.GetQueue(),
        Api.GetQueueStats()
      ]);
      setItems(queueItems || []);
      setStats(queueStats);
    } catch (err) {
      console.error('Failed to fetch queue:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  // Listen for queue events via WebSocket
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
          if (event.item) {
            setItems(prev =>
              prev.map(item => item.id === event.itemId ? event.item! : item)
            );
          }
          playSound('complete');
          break;
        case 'error':
          if (event.item) {
            setItems(prev =>
              prev.map(item => item.id === event.itemId ? event.item! : item)
            );
          }
          playSound('error');
          break;
      }
      // Refresh stats on any event
      Api.GetQueueStats().then(setStats).catch(console.error);
    };

    const unsubscribe = EventsOn('queue:event', handleEvent);
    return () => unsubscribe();
  }, [fetchQueue]);

  // Actions
  const addToQueue = useCallback(async (videoUrl: string, spotifyUrl?: string) => {
    const request: Api.DownloadRequest = {
      videoUrl,
      spotifyUrl
    };
    return Api.AddToQueue(request);
  }, []);

  const removeFromQueue = useCallback(async (id: string) => {
    await Api.RemoveFromQueue(id);
  }, []);

  const cancelItem = useCallback(async (id: string) => {
    await Api.CancelQueueItem(id);
  }, []);

  const clearCompleted = useCallback(async () => {
    await Api.ClearCompleted();
    fetchQueue();
  }, [fetchQueue]);

  const retryFailed = useCallback(async () => {
    await Api.RetryFailed();
    fetchQueue();
  }, [fetchQueue]);

  const clearAll = useCallback(async () => {
    await Api.ClearQueue();
    fetchQueue();
  }, [fetchQueue]);

  const moveItem = useCallback(async (id: string, newIndex: number) => {
    await Api.MoveQueueItem(id, newIndex);
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
