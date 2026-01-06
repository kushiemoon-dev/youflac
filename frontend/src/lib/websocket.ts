/**
 * WebSocket client for real-time queue updates
 * Replaces Wails EventsOn/EventsOff
 */

import type { QueueEvent } from './api';

type EventCallback = (event: QueueEvent) => void;

class WebSocketClient {
  private ws: WebSocket | null = null;
  private callbacks: Set<EventCallback> = new Set();
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectDelay = 1000;
  private isConnecting = false;

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.isConnecting) {
      return;
    }

    this.isConnecting = true;

    // Build WebSocket URL from current location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('[WebSocket] Connected');
        this.reconnectAttempts = 0;
        this.isConnecting = false;
      };

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as QueueEvent;
          this.callbacks.forEach((callback) => {
            try {
              callback(data);
            } catch (err) {
              console.error('[WebSocket] Callback error:', err);
            }
          });
        } catch (err) {
          console.error('[WebSocket] Failed to parse message:', err);
        }
      };

      this.ws.onclose = (event) => {
        console.log('[WebSocket] Disconnected:', event.code, event.reason);
        this.isConnecting = false;
        this.ws = null;
        this.attemptReconnect();
      };

      this.ws.onerror = (error) => {
        console.error('[WebSocket] Error:', error);
        this.isConnecting = false;
      };
    } catch (err) {
      console.error('[WebSocket] Connection failed:', err);
      this.isConnecting = false;
      this.attemptReconnect();
    }
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('[WebSocket] Max reconnect attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
    console.log(`[WebSocket] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

    setTimeout(() => {
      this.connect();
    }, delay);
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  subscribe(callback: EventCallback): () => void {
    this.callbacks.add(callback);

    // Auto-connect on first subscription
    if (this.callbacks.size === 1) {
      this.connect();
    }

    // Return unsubscribe function
    return () => {
      this.callbacks.delete(callback);

      // Auto-disconnect when no subscribers
      if (this.callbacks.size === 0) {
        this.disconnect();
      }
    };
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }
}

// Singleton instance
export const wsClient = new WebSocketClient();

// Convenience functions matching Wails EventsOn/EventsOff pattern
export function EventsOn(eventName: string, callback: (data: any) => void): () => void {
  // For queue events, we use the WebSocket
  if (eventName === 'queue:event') {
    return wsClient.subscribe(callback);
  }

  // For other events, return a no-op unsubscribe
  console.warn(`[WebSocket] Unknown event: ${eventName}`);
  return () => {};
}

export function EventsOff(eventName: string): void {
  // This is a legacy API - with the subscribe pattern, cleanup happens automatically
  console.log(`[WebSocket] EventsOff called for: ${eventName}`);
}
