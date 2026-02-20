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

type Listener<T> = (data: T) => void;

export class WebSocketClient<T = WSMessage> {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private listeners: Map<string, Set<Listener<any>>> = new Map();
  private anyListeners: Set<Listener<T>> = new Set();
  private shouldReconnect = true;
  private connectPromise: Promise<void> | null = null;

  private token: string | null = null;

  constructor(url: string, token?: string) {
    this.url = url;
    this.token = token || null;
  }

  public async connect() {
    if (this.ws?.readyState === WebSocket.OPEN) return;
    if (this.connectPromise) return this.connectPromise;

    this.connectPromise = new Promise((resolve) => {
      const activeToken = this.token || useAuthStore.getState().token;
      const wsUrl = activeToken ? `${this.url}?token=${activeToken}` : this.url;

      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('WS Connected');
        this.reconnectAttempts = 0;
        this.connectPromise = null;
        resolve();
      };

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          const typed = data as WSMessage;
          this.anyListeners.forEach((listener) => listener(data));
          const bucket = this.listeners.get(String(typed?.type));
          if (bucket) {
            bucket.forEach((listener) => listener(typed));
          }
        } catch (e) {
          console.error("WS Parse error", e);
        }
      };

      this.ws.onclose = () => {
        this.connectPromise = null;
        if (this.shouldReconnect) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = (err) => {
        console.error('WS Error', err);
        // Error will trigger onclose, where reconnect logic resides
      };
    });

    return this.connectPromise;
  }

  private scheduleReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('Max reconnect attempts reached');
      return;
    }

    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    console.log(`Reconnecting in ${delay}ms...`);
    
    setTimeout(() => {
      this.reconnectAttempts++;
      this.connect();
    }, delay);
  }

  public subscribe(listener: Listener<T>): () => void {
    this.anyListeners.add(listener);
    if (this.ws?.readyState !== WebSocket.OPEN) {
      this.connect();
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
      this.connect();
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
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    } else {
      console.warn('WS not connected, cannot send message');
    }
  }

  public close() {
    this.shouldReconnect = false;
    this.ws?.close();
  }
}
