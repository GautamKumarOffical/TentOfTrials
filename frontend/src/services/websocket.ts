/**
 * Standalone WebSocket client with exponential backoff reconnection.
 *
 * This module provides a framework-agnostic WebSocket client that can be
 * used outside of React components. For React integration, see the
 * useWebSocket hook.
 *
 * Features:
 * - Exponential backoff with jitter for reconnection
 * - Configurable max delay (default: 30s) and max retries
 * - Connection state tracking with typed states
 * - Message queue for offline buffering
 * - Subscription management for multiplexed channels
 * - Heartbeat (ping/pong) support
 * - Automatic cleanup and resource management
 */

// ---------------------------------------------------------------------------
// TYPES
// ---------------------------------------------------------------------------

export type WSConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'error';

export interface WSMessage {
  type: string;
  channel?: string;
  payload: unknown;
  id?: string;
  timestamp?: number;
}

export interface WSSubscription {
  channel: string;
  filter?: Record<string, unknown>;
  callback: (data: unknown) => void;
}

export interface WSClientOptions {
  url: string;
  protocols?: string | string[];
  autoConnect?: boolean;
  reconnect?: boolean;
  maxReconnectAttempts?: number;
  reconnectBaseDelay?: number;
  reconnectMaxDelay?: number;
  reconnectJitter?: number;
  pingInterval?: number;
  pongTimeout?: number;
  messageQueueSize?: number;
  debug?: boolean;
  onOpen?: (event: Event) => void;
  onClose?: (event: CloseEvent) => void;
  onError?: (event: Event) => void;
  onMessage?: (message: WSMessage) => void;
  onStateChange?: (state: WSConnectionState) => void;
}

export interface WSClientState {
  connectionState: WSConnectionState;
  lastMessage: WSMessage | null;
  reconnectAttempt: number;
  queueSize: number;
  subscriptions: number;
  totalMessagesSent: number;
  totalMessagesReceived: number;
  errors: number;
  latencyMs: number | null;
}

export type WSClientStateListener = (state: WSClientState) => void;

// ---------------------------------------------------------------------------
// CONSTANTS
// ---------------------------------------------------------------------------

const DEFAULT_OPTIONS: Required<Omit<WSClientOptions, 'url' | 'protocols' | 'onOpen' | 'onClose' | 'onError' | 'onMessage' | 'onStateChange'>> = {
  autoConnect: true,
  reconnect: true,
  maxReconnectAttempts: 10,
  reconnectBaseDelay: 1000,
  reconnectMaxDelay: 30000,
  reconnectJitter: 1000,
  pingInterval: 30000,
  pongTimeout: 10000,
  messageQueueSize: 100,
  debug: false,
};

// ---------------------------------------------------------------------------
// BACKOFF UTILITIES (exported for testing)
// ---------------------------------------------------------------------------

/**
 * Calculate exponential backoff delay with jitter.
 *
 * Formula: min(baseDelay * 2^attempt, maxDelay) + random(0, jitter)
 *
 * @param attempt - Current reconnection attempt (0-indexed)
 * @param baseDelay - Base delay in milliseconds
 * @param maxDelay - Maximum delay cap in milliseconds
 * @param jitter - Maximum random jitter in milliseconds
 * @returns Delay in milliseconds
 */
export function calculateBackoffDelay(
  attempt: number,
  baseDelay: number,
  maxDelay: number,
  jitter: number
): number {
  const exponentialDelay = baseDelay * Math.pow(2, attempt);
  const cappedDelay = Math.min(exponentialDelay, maxDelay);
  const jitterAmount = Math.random() * jitter;
  return cappedDelay + jitterAmount;
}

// ---------------------------------------------------------------------------
// WEBSOCKET CLIENT CLASS
// ---------------------------------------------------------------------------

export class WebSocketClient {
  private options: Required<Omit<WSClientOptions, 'url' | 'protocols' | 'onOpen' | 'onClose' | 'onError' | 'onMessage' | 'onStateChange'>> & Pick<WSClientOptions, 'url' | 'protocols' | 'onOpen' | 'onClose' | 'onError' | 'onMessage' | 'onStateChange'>;
  private ws: WebSocket | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private pingTimer: ReturnType<typeof setInterval> | null = null;
  private pongTimer: ReturnType<typeof setTimeout> | null = null;
  private messageQueue: Array<{ message: WSMessage; timestamp: number; retries: number }> = [];
  private subscriptions = new Map<string, WSSubscription>();
  private reconnectAttempt = 0;
  private mounted = true;
  private messageId = 0;
  private pingStart = 0;
  private stateListeners = new Set<WSClientStateListener>();

  private state: WSClientState = {
    connectionState: 'disconnected',
    lastMessage: null,
    reconnectAttempt: 0,
    queueSize: 0,
    subscriptions: 0,
    totalMessagesSent: 0,
    totalMessagesReceived: 0,
    errors: 0,
    latencyMs: null,
  };

  constructor(options: WSClientOptions) {
    this.options = { ...DEFAULT_OPTIONS, ...options };
    if (this.options.autoConnect) {
      this.connect();
    }
  }

  /**
   * Subscribe to state changes.
   * @returns Unsubscribe function
   */
  onStateChange(listener: WSClientStateListener): () => void {
    this.stateListeners.add(listener);
    return () => {
      this.stateListeners.delete(listener);
    };
  }

  /**
   * Get current connection state.
   */
  getState(): WSClientState {
    return { ...this.state };
  }

  /**
   * Check if WebSocket is currently connected.
   */
  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  /**
   * Connect to the WebSocket server.
   */
  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
      return;
    }

    this.updateState({ connectionState: 'connecting', reconnectAttempt: this.reconnectAttempt });

    try {
      const ws = new WebSocket(this.options.url, this.options.protocols);
      this.ws = ws;

      ws.onopen = (event) => {
        if (!this.mounted) return;
        this.reconnectAttempt = 0;
        this.updateState({ connectionState: 'connected', reconnectAttempt: 0 });

        // Resubscribe to all channels
        this.subscriptions.forEach((sub, channel) => {
          this.sendMessage({
            type: 'subscribe',
            channel,
            payload: sub.filter || {},
          });
        });

        // Flush queued messages
        while (this.messageQueue.length > 0) {
          const queued = this.messageQueue.shift()!;
          this.sendMessage(queued.message);
        }
        this.updateState({ queueSize: 0 });

        // Start ping interval
        this.startPing();

        this.options.onOpen?.(event);
      };

      ws.onmessage = (event) => {
        if (!this.mounted) return;

        try {
          const message: WSMessage = JSON.parse(event.data);

          // Handle pong response
          if (message.type === 'pong') {
            const latency = Date.now() - this.pingStart;
            this.updateState({ latencyMs: latency });
            this.clearPongTimeout();
            return;
          }

          this.updateState({
            lastMessage: message,
            totalMessagesReceived: this.state.totalMessagesReceived + 1,
          });

          // Route to channel subscribers
          if (message.channel) {
            const sub = this.subscriptions.get(message.channel);
            if (sub) {
              try {
                sub.callback(message.payload);
              } catch (err) {
                if (this.options.debug) {
                  console.error(`[WS] Subscriber error for channel ${message.channel}:`, err);
                }
              }
            }
          }

          // Route to global message handler
          this.options.onMessage?.(message);
        } catch (err) {
          if (this.options.debug) {
            console.error('[WS] Failed to parse message:', err);
          }
        }
      };

      ws.onclose = (event) => {
        if (!this.mounted) return;
        this.ws = null;
        this.stopPing();
        this.updateState({ connectionState: 'disconnected' });
        this.options.onClose?.(event);
        this.scheduleReconnect();
      };

      ws.onerror = (event) => {
        if (!this.mounted) return;
        this.updateState({ errors: this.state.errors + 1, connectionState: 'error' });
        this.options.onError?.(event);
      };
    } catch (err) {
      if (!this.mounted) return;
      this.updateState({ errors: this.state.errors + 1, connectionState: 'error' });
      if (this.options.debug) {
        console.error('[WS] Connection error:', err);
      }
      this.scheduleReconnect();
    }
  }

  /**
   * Disconnect from the WebSocket server.
   */
  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
    this.stopPing();
    this.updateState({ connectionState: 'disconnected', reconnectAttempt: 0 });
  }

  /**
   * Send a message.
   */
  send(type: string, payload: unknown, channel?: string): string {
    const id = `msg_${++this.messageId}`;
    this.sendMessage({
      id,
      type,
      channel,
      payload,
      timestamp: Date.now(),
    });
    return id;
  }

  /**
   * Subscribe to a channel.
   */
  subscribe(subscription: WSSubscription): void {
    this.subscriptions.set(subscription.channel, subscription);
    this.updateState({ subscriptions: this.subscriptions.size });

    if (this.ws?.readyState === WebSocket.OPEN) {
      this.sendMessage({
        type: 'subscribe',
        channel: subscription.channel,
        payload: subscription.filter || {},
      });
    }
  }

  /**
   * Unsubscribe from a channel.
   */
  unsubscribe(channel: string): void {
    this.subscriptions.delete(channel);
    this.updateState({ subscriptions: this.subscriptions.size });

    if (this.ws?.readyState === WebSocket.OPEN) {
      this.sendMessage({
        type: 'unsubscribe',
        channel,
        payload: null,
      });
    }
  }

  /**
   * Destroy the client and clean up all resources.
   */
  destroy(): void {
    this.mounted = false;
    this.disconnect();
    this.stateListeners.clear();
    this.messageQueue = [];
    this.subscriptions.clear();
  }

  // ---------------------------------------------------------------------------
  // PRIVATE METHODS
  // ---------------------------------------------------------------------------

  private updateState(partial: Partial<WSClientState>): void {
    const newState = { ...this.state, ...partial };
    const stateChanged = newState.connectionState !== this.state.connectionState ||
      newState.reconnectAttempt !== this.state.reconnectAttempt;

    this.state = newState;

    if (stateChanged) {
      this.options.onStateChange?.(newState.connectionState);
    }

    // Notify all listeners
    this.stateListeners.forEach(listener => {
      try {
        listener(this.state);
      } catch (err) {
        if (this.options.debug) {
          console.error('[WS] State listener error:', err);
        }
      }
    });
  }

  private sendMessage(message: WSMessage): void {
    const msgStr = JSON.stringify(message);

    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(msgStr);
      this.updateState({ totalMessagesSent: this.state.totalMessagesSent + 1 });
    } else {
      // Queue message for later delivery
      if (this.messageQueue.length < this.options.messageQueueSize) {
        this.messageQueue.push({
          message,
          timestamp: Date.now(),
          retries: 0,
        });
        this.updateState({ queueSize: this.messageQueue.length });
      } else if (this.options.debug) {
        console.warn('[WS] Message queue full, dropping message:', message.type);
      }
    }
  }

  private scheduleReconnect(): void {
    if (!this.options.reconnect || this.reconnectAttempt >= this.options.maxReconnectAttempts) {
      this.updateState({ connectionState: 'error' });
      return;
    }

    const delay = calculateBackoffDelay(
      this.reconnectAttempt,
      this.options.reconnectBaseDelay,
      this.options.reconnectMaxDelay,
      this.options.reconnectJitter
    );

    this.reconnectAttempt++;

    if (this.options.debug) {
      console.log(`[WS] Reconnecting in ${Math.round(delay)}ms (attempt ${this.reconnectAttempt})`);
    }

    this.updateState({ connectionState: 'reconnecting', reconnectAttempt: this.reconnectAttempt });

    this.reconnectTimer = setTimeout(() => {
      if (this.mounted) this.connect();
    }, delay);
  }

  private startPing(): void {
    this.stopPing();
    this.pingTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.pingStart = Date.now();
        this.ws.send(JSON.stringify({ type: 'ping' }));

        // Set pong timeout
        this.pongTimer = setTimeout(() => {
          if (this.options.debug) {
            console.warn('[WS] Pong timeout, closing connection');
          }
          this.updateState({ latencyMs: null });
          this.ws?.close(4000, 'Pong timeout');
        }, this.options.pongTimeout);
      }
    }, this.options.pingInterval);
  }

  private stopPing(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
    this.clearPongTimeout();
  }

  private clearPongTimeout(): void {
    if (this.pongTimer) {
      clearTimeout(this.pongTimer);
      this.pongTimer = null;
    }
  }
}

/**
 * Create a new WebSocket client instance.
 */
export function createWebSocketClient(options: WSClientOptions): WebSocketClient {
  return new WebSocketClient(options);
}
