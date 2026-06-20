/**
 * React hook for managing WebSocket connections with automatic reconnection.
 *
 * This hook wraps the WebSocketClient class to provide React-specific
 * integration with state management and cleanup.
 *
 * Features:
 * - Exponential backoff reconnection with jitter
 * - Configurable heartbeat (ping/pong)
 * - Message queue for offline messages
 * - Subscription management for multiplexed channels
 * - Connection state tracking
 * - Automatic cleanup on unmount
 *
 * The reconnection strategy uses truncated exponential backoff:
 *   delay = min(base_delay * 2^attempt, max_delay) + random(0, jitter_ms)
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  WebSocketClient,
  calculateBackoffDelay,
  type WSClientOptions,
  type WSClientState,
  type WSConnectionState,
  type WSMessage,
  type WSSubscription,
} from '../services/websocket';

// Re-export types for convenience
export type { WSClientOptions, WSClientState, WSConnectionState, WSMessage, WSSubscription };

// Re-export for testing
export { calculateBackoffDelay };

export interface UseWebSocketOptions extends WSClientOptions {}

export function useWebSocket(options: UseWebSocketOptions) {
  const clientRef = useRef<WebSocketClient | null>(null);
  const [state, setState] = useState<WSClientState>({
    connectionState: 'disconnected',
    lastMessage: null,
    reconnectAttempt: 0,
    queueSize: 0,
    subscriptions: 0,
    totalMessagesSent: 0,
    totalMessagesReceived: 0,
    errors: 0,
    latencyMs: null,
  });

  // Create client on mount
  useEffect(() => {
    // Disable autoConnect since we'll manage it manually
    const client = new WebSocketClient({
      ...options,
      autoConnect: false,
    });

    clientRef.current = client;

    // Subscribe to state changes
    const unsubscribe = client.onStateChange((newState) => {
      setState(client.getState());
    });

    // Set initial state
    setState(client.getState());

    // Connect if autoConnect is enabled
    if (options.autoConnect !== false) {
      client.connect();
    }

    return () => {
      unsubscribe();
      client.destroy();
      clientRef.current = null;
    };
  }, []);

  // Update client when options change (except url which requires reconnect)
  useEffect(() => {
    const client = clientRef.current;
    if (!client) return;

    // Note: URL changes require manual reconnection
    // Other options could be updated if needed
  }, [options]);

  const connect = useCallback(() => {
    clientRef.current?.connect();
  }, []);

  const disconnect = useCallback(() => {
    clientRef.current?.disconnect();
  }, []);

  const send = useCallback((type: string, payload: unknown, channel?: string) => {
    return clientRef.current?.send(type, payload, channel) ?? '';
  }, []);

  const subscribe = useCallback((subscription: WSSubscription) => {
    clientRef.current?.subscribe(subscription);
  }, []);

  const unsubscribe = useCallback((channel: string) => {
    clientRef.current?.unsubscribe(channel);
  }, []);

  return {
    ...state,
    connect,
    disconnect,
    send,
    subscribe,
    unsubscribe,
    isConnected: clientRef.current?.isConnected ?? false,
  };
}
