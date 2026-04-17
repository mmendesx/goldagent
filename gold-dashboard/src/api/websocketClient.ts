import type { WebSocketMessage } from "../types";

type MessageHandler = (message: WebSocketMessage) => void;
type ConnectionStateHandler = (state: ConnectionState) => void;
type ReconnectAttemptHandler = (attempt: number) => void;

export type ConnectionState = "connecting" | "open" | "closed" | "reconnecting";

const WS_URL = import.meta.env.VITE_WS_URL ?? "ws://localhost:8080/ws/v1/stream";

export class WebSocketClient {
  private socket: WebSocket | null = null;
  private messageHandlers = new Set<MessageHandler>();
  private connectionStateHandlers = new Set<ConnectionStateHandler>();
  private reconnectAttemptHandlers = new Set<ReconnectAttemptHandler>();
  private reconnectAttempt = 0;
  private reconnectTimer: number | null = null;
  private isExplicitlyClosed = false;
  private currentState: ConnectionState = "closed";

  getReconnectAttempts(): number {
    return this.reconnectAttempt;
  }

  connect(): void {
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (
      this.socket !== null &&
      (this.socket.readyState === WebSocket.CONNECTING ||
        this.socket.readyState === WebSocket.OPEN)
    ) {
      return;
    }
    this.isExplicitlyClosed = false;
    this.openConnection();
  }

  disconnect(): void {
    this.isExplicitlyClosed = true;
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
    this.setState("closed");
  }

  subscribe(handler: MessageHandler): () => void {
    this.messageHandlers.add(handler);
    return () => this.messageHandlers.delete(handler);
  }

  onConnectionStateChange(handler: ConnectionStateHandler): () => void {
    this.connectionStateHandlers.add(handler);
    handler(this.currentState);
    return () => this.connectionStateHandlers.delete(handler);
  }

  onReconnectAttempt(handler: ReconnectAttemptHandler): () => void {
    this.reconnectAttemptHandlers.add(handler);
    handler(this.reconnectAttempt);
    return () => this.reconnectAttemptHandlers.delete(handler);
  }

  subscribeToSymbols(symbols: string[]): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ action: "subscribe", symbols }));
    }
  }

  private openConnection(): void {
    this.setState(this.reconnectAttempt > 0 ? "reconnecting" : "connecting");
    try {
      this.socket = new WebSocket(WS_URL);
    } catch {
      this.scheduleReconnect();
      return;
    }

    this.socket.onopen = () => {
      this.reconnectAttempt = 0;
      for (const handler of this.reconnectAttemptHandlers) {
        handler(0);
      }
      this.setState("open");
    };

    this.socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data as string) as WebSocketMessage;
        for (const handler of this.messageHandlers) {
          handler(message);
        }
      } catch (error) {
        console.error("Failed to parse WebSocket message", error);
      }
    };

    this.socket.onerror = (event) => {
      console.error("WebSocket error", event);
    };

    this.socket.onclose = () => {
      this.socket = null;
      if (!this.isExplicitlyClosed) {
        this.scheduleReconnect();
      } else {
        this.setState("closed");
      }
    };
  }

  private scheduleReconnect(): void {
    this.setState("reconnecting");
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    const delayMilliseconds = Math.min(1000 * 2 ** this.reconnectAttempt, 30000);
    this.reconnectAttempt += 1;
    for (const handler of this.reconnectAttemptHandlers) {
      handler(this.reconnectAttempt);
    }
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.openConnection();
    }, delayMilliseconds);
  }

  private setState(state: ConnectionState): void {
    if (this.currentState === state) return;
    this.currentState = state;
    for (const handler of this.connectionStateHandlers) {
      handler(state);
    }
  }
}

export const webSocketClient = new WebSocketClient();
