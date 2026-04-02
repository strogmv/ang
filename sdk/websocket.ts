// This file is auto-generated. Do not edit.
import { useAuthStore } from './auth-store';
import type * as Types from './types';

export type EventMap = {
  CommentCreated: Types.CommentCreated;
  PostCreated: Types.PostCreated;
  PostPublished: Types.PostPublished;
  PostUpdated: Types.PostUpdated;
  UserLoggedIn: Types.UserLoggedIn;
  UserRegistered: Types.UserRegistered;
};

export type WSMessage = {
  [K in keyof EventMap]: { type: K; payload: EventMap[K] }
}[keyof EventMap];

export type ConnectState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

export type WebSocketClientOptions = {
  handleNetworkEvents?: boolean;
  reconnectBaseDelayMs?: number;
  maxReconnectDelayMs?: number;
  heartbeatIntervalMs?: number;
  heartbeatTimeoutMs?: number;
  maxQueuedMessages?: number;
};

type Listener<T> = (data: T) => void;
type StateListener = (state: ConnectState) => void;
type InternalWSMessage = { type: 'pong'; payload?: { ts?: string | number } };

export class WebSocketClient<T = WSMessage> {
  private ws: WebSocket | null = null;
  private readonly url: string;
  private reconnectAttempts = 0;
  private listeners: Map<string, Set<Listener<any>>> = new Map();
  private anyListeners: Set<Listener<T>> = new Set();
  private stateListeners: Set<StateListener> = new Set();
  private shouldReconnect = true;
  private connectPromise: Promise<void> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private heartbeatTimeoutTimer: ReturnType<typeof setTimeout> | null = null;
  private pendingMessages: any[] = [];
  private token: string | null = null;
  private state: ConnectState = 'disconnected';
  private browserHooksInstalled = false;
  private readonly options: Required<WebSocketClientOptions>;

  private readonly handleOnline = () => {
    if (!this.shouldReconnect) {
      return;
    }
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
      return;
    }
    void this.connect().catch(() => {});
  };

  private readonly handleVisibilityChange = () => {
    if (typeof document === 'undefined' || document.visibilityState !== 'visible') {
      return;
    }
    if (!this.shouldReconnect) {
      return;
    }
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
      return;
    }
    void this.connect().catch(() => {});
  };

  constructor(url: string, tokenOrOptions?: string | WebSocketClientOptions, options?: WebSocketClientOptions) {
    this.url = url;
    this.token = typeof tokenOrOptions === 'string' ? tokenOrOptions : null;
    const resolvedOptions = typeof tokenOrOptions === 'string' ? options : tokenOrOptions;
    this.options = {
      handleNetworkEvents: false,
      reconnectBaseDelayMs: 1000,
      maxReconnectDelayMs: 30000,
      heartbeatIntervalMs: 25000,
      heartbeatTimeoutMs: 10000,
      maxQueuedMessages: 100,
      ...(resolvedOptions || {}),
    };
    this.installBrowserHooks();
  }

  public getState(): ConnectState {
    return this.state;
  }

  public onStateChange(listener: StateListener): () => void {
    this.stateListeners.add(listener);
    listener(this.state);
    return () => {
      this.stateListeners.delete(listener);
    };
  }

  public setToken(token: string | null) {
    this.token = token;
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.sendAuthFrame();
    }
  }

  public async connect() {
    this.installBrowserHooks();
    this.shouldReconnect = true;
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }
    if (this.connectPromise) {
      return this.connectPromise;
    }

    this.clearReconnectTimer();
    this.setState(this.reconnectAttempts > 0 || this.state === 'reconnecting' ? 'reconnecting' : 'connecting');

    this.connectPromise = new Promise((resolve, reject) => {
      let opened = false;
      let settled = false;

      const rejectOnce = (error: unknown) => {
        if (settled) {
          return;
        }
        settled = true;
        this.connectPromise = null;
        reject(error instanceof Error ? error : new Error(String(error || 'WebSocket connection failed')));
      };

      const resolveOnce = () => {
        if (settled) {
          return;
        }
        settled = true;
        this.connectPromise = null;
        resolve();
      };

      this.ws = new WebSocket(this.url);

      this.ws.onopen = () => {
        opened = true;
        this.reconnectAttempts = 0;
        this.sendAuthFrame();
        this.flushPendingMessages();
        this.startHeartbeat();
        this.setState('connected');
        console.log('WS Connected');
        resolveOnce();
      };

      this.ws.onmessage = (event) => {
        this.acknowledgeHeartbeat();
        try {
          const message = JSON.parse(event.data) as WSMessage | InternalWSMessage;
          if (message?.type === 'pong') {
            return;
          }
          const typed = message as WSMessage;
          this.anyListeners.forEach((listener) => listener(message as T));
          const bucket = this.listeners.get(String(typed?.type));
          if (bucket) {
            bucket.forEach((listener) => listener(typed));
          }
        } catch (e) {
          console.error('WS Parse error', e);
        }
      };

      this.ws.onclose = (event) => {
        this.stopHeartbeat();
        if (this.ws?.readyState === WebSocket.CLOSED || this.ws?.readyState === WebSocket.CLOSING) {
          this.ws = null;
        }
        this.connectPromise = null;
        if (!opened) {
          rejectOnce(new Error(`WebSocket closed before open (${event.code})`));
        }
        if (this.shouldReconnect) {
          this.scheduleReconnect();
        } else {
          this.setState('disconnected');
        }
      };

      this.ws.onerror = (err) => {
        console.error('WS Error', err);
        if (!opened) {
          rejectOnce(new Error('WebSocket connection error'));
          try {
            this.ws?.close();
          } catch {}
        }
      };
    });

    return this.connectPromise;
  }

  public subscribe(listener: Listener<T>): () => void {
    this.anyListeners.add(listener);
    if (this.ws?.readyState !== WebSocket.OPEN) {
      void this.connect().catch(() => {});
    }
    return () => {
      this.anyListeners.delete(listener);
    };
  }

  public on<K extends keyof EventMap>(type: K, listener: Listener<{ type: K; payload: EventMap[K] }>): () => void {
    const key = String(type);
    let bucket = this.listeners.get(key);
    if (!bucket) {
      bucket = new Set();
      this.listeners.set(key, bucket);
    }
    bucket.add(listener as Listener<any>);
    if (this.ws?.readyState !== WebSocket.OPEN) {
      void this.connect().catch(() => {});
    }
    return () => {
      const current = this.listeners.get(key);
      if (!current) {
        return;
      }
      current.delete(listener as Listener<any>);
      if (current.size === 0) {
        this.listeners.delete(key);
      }
    };
  }

  public send(data: any) {
    if (this.rawSend(data)) {
      return;
    }
    if (this.pendingMessages.length >= this.options.maxQueuedMessages) {
      this.pendingMessages.shift();
    }
    this.pendingMessages.push(data);
    if (this.shouldReconnect && this.ws?.readyState !== WebSocket.CONNECTING) {
      void this.connect().catch(() => {});
    }
  }

  public close() {
    this.shouldReconnect = false;
    this.clearReconnectTimer();
    this.stopHeartbeat();
    this.connectPromise = null;
    this.ws?.close();
    this.ws = null;
    this.setState('disconnected');
    this.uninstallBrowserHooks();
  }

  private setState(next: ConnectState) {
    if (this.state === next) {
      return;
    }
    this.state = next;
    this.stateListeners.forEach((listener) => listener(next));
  }

  private getActiveToken(): string | null {
    return this.token || useAuthStore.getState().token || null;
  }

  private sendAuthFrame() {
    const activeToken = this.getActiveToken();
    if (!activeToken || this.ws?.readyState !== WebSocket.OPEN) {
      return;
    }
    this.rawSend({ type: 'auth', token: activeToken });
  }

  private rawSend(data: any): boolean {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      return false;
    }
    this.ws.send(JSON.stringify(data));
    return true;
  }

  private flushPendingMessages() {
    if (this.ws?.readyState !== WebSocket.OPEN || this.pendingMessages.length === 0) {
      return;
    }
    const queued = [...this.pendingMessages];
    this.pendingMessages = [];
    for (const message of queued) {
      if (!this.rawSend(message)) {
        this.pendingMessages.unshift(message);
        break;
      }
    }
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private scheduleReconnect() {
    if (!this.shouldReconnect || this.reconnectTimer) {
      return;
    }
    this.setState('reconnecting');
    const delay = Math.min(this.options.reconnectBaseDelayMs * Math.pow(2, this.reconnectAttempts), this.options.maxReconnectDelayMs);
    console.log(`Reconnecting in ${delay}ms...`);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.reconnectAttempts++;
      void this.connect().catch((error) => {
        console.warn('WS reconnect attempt failed', error);
      });
    }, delay);
  }

  private startHeartbeat() {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      return;
    }
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState !== WebSocket.OPEN) {
        return;
      }
      if (!this.rawSend({ type: 'ping', ts: Date.now() })) {
        return;
      }
      this.scheduleHeartbeatTimeout();
    }, this.options.heartbeatIntervalMs);
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    if (this.heartbeatTimeoutTimer) {
      clearTimeout(this.heartbeatTimeoutTimer);
      this.heartbeatTimeoutTimer = null;
    }
  }

  private scheduleHeartbeatTimeout() {
    this.acknowledgeHeartbeat();
    this.heartbeatTimeoutTimer = setTimeout(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        console.warn('WS heartbeat timeout, forcing reconnect');
        this.ws.close();
      }
    }, this.options.heartbeatTimeoutMs);
  }

  private acknowledgeHeartbeat() {
    if (this.heartbeatTimeoutTimer) {
      clearTimeout(this.heartbeatTimeoutTimer);
      this.heartbeatTimeoutTimer = null;
    }
  }

  private installBrowserHooks() {
    if (!this.options.handleNetworkEvents || this.browserHooksInstalled) {
      return;
    }
    if (typeof window === 'undefined' || typeof document === 'undefined') {
      return;
    }
    window.addEventListener('online', this.handleOnline);
    document.addEventListener('visibilitychange', this.handleVisibilityChange);
    this.browserHooksInstalled = true;
  }

  private uninstallBrowserHooks() {
    if (!this.browserHooksInstalled) {
      return;
    }
    if (typeof window !== 'undefined') {
      window.removeEventListener('online', this.handleOnline);
    }
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', this.handleVisibilityChange);
    }
    this.browserHooksInstalled = false;
  }
}
