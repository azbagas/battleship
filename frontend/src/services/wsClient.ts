import {
  ConnectionStatus,
  ErrorCode,
  ErrorPayload,
  HelloOKPayload,
  HelloPayload,
  InboundMessage,
  OutboundMessage,
} from '../types/protocol'

export interface ConnectOptions {
  displayName: string
  sessionToken?: string | null
  url?: string
}

export type StatusListener = (status: ConnectionStatus, detail?: string) => void
export type MessageListener<T = unknown> = (payload: T) => void
export type ErrorListener = (error: ErrorPayload) => void
export type HelloOKListener = (payload: HelloOKPayload) => void

export function resolveWebSocketUrl(customUrl?: string): string {
  if (customUrl) return customUrl
  if (typeof window === 'undefined') return 'ws://localhost:8080/ws'
  const envUrl = import.meta.env?.VITE_WS_URL
  if (envUrl) return envUrl
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws`
}

export class WebSocketClient {
  private ws: WebSocket | null = null
  private status: ConnectionStatus = 'disconnected'
  private connectOptions: ConnectOptions | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempts = 0
  private isExplicitDisconnect = false
  private hasSessionConflict = false

  // Event listener registries
  private statusListeners = new Set<StatusListener>()
  private errorListeners = new Set<ErrorListener>()
  private helloOKListeners = new Set<HelloOKListener>()
  private messageListeners = new Map<string, Set<MessageListener<unknown>>>()

  // Reconnection backoff parameters (in ms)
  private readonly initialBackoff = 1000
  private readonly maxBackoff = 10000
  private readonly backoffFactor = 1.5

  getStatus(): ConnectionStatus {
    return this.status
  }

  isConflict(): boolean {
    return this.hasSessionConflict
  }

  onStatusChange(listener: StatusListener): () => void {
    this.statusListeners.add(listener)
    listener(this.status)
    return () => this.statusListeners.delete(listener)
  }

  onError(listener: ErrorListener): () => void {
    this.errorListeners.add(listener)
    return () => this.errorListeners.delete(listener)
  }

  onHelloOK(listener: HelloOKListener): () => void {
    this.helloOKListeners.add(listener)
    return () => this.helloOKListeners.delete(listener)
  }

  onMessage<T = unknown>(type: string, listener: MessageListener<T>): () => void {
    if (!this.messageListeners.has(type)) {
      this.messageListeners.set(type, new Set())
    }
    const set = this.messageListeners.get(type)!
    set.add(listener as MessageListener<unknown>)
    return () => {
      set.delete(listener as MessageListener<unknown>)
      if (set.size === 0) {
        this.messageListeners.delete(type)
      }
    }
  }

  connect(options: ConnectOptions): void {
    this.connectOptions = options
    this.isExplicitDisconnect = false
    this.hasSessionConflict = false
    this.clearReconnectTimer()

    this.establishConnection()
  }

  disconnect(): void {
    this.isExplicitDisconnect = true
    this.clearReconnectTimer()
    if (this.ws) {
      // Avoid firing close reconnect logic
      this.ws.onclose = null
      this.ws.onerror = null
      this.ws.onmessage = null
      this.ws.close()
      this.ws = null
    }
    this.setStatus('disconnected')
  }

  send<T = unknown>(type: string, payload: T): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return false
    }
    const message: OutboundMessage<T> = { type, payload }
    this.ws.send(JSON.stringify(message))
    return true
  }

  private setStatus(status: ConnectionStatus, detail?: string): void {
    this.status = status
    for (const listener of this.statusListeners) {
      try {
        listener(status, detail)
      } catch (err) {
        console.error('Error in status listener:', err)
      }
    }
  }

  private establishConnection(): void {
    if (!this.connectOptions) return

    const url = resolveWebSocketUrl(this.connectOptions.url)
    this.setStatus('connecting')

    try {
      this.ws = new WebSocket(url)
    } catch (err) {
      console.error('Failed to construct WebSocket:', err)
      this.setStatus('error', 'Failed to initialize WebSocket')
      this.scheduleReconnect()
      return
    }

    this.ws.onopen = () => {
      this.reconnectAttempts = 0
      // Send hello handshake immediately
      const helloPayload: HelloPayload = {
        name: this.connectOptions!.displayName,
        ...(this.connectOptions!.sessionToken
          ? { sessionToken: this.connectOptions!.sessionToken }
          : {}),
      }
      this.send('hello', helloPayload)
    }

    this.ws.onmessage = (event: MessageEvent) => {
      if (typeof event.data !== 'string') return

      try {
        const msg = JSON.parse(event.data) as InboundMessage
        this.handleInboundMessage(msg)
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err)
      }
    }

    this.ws.onerror = () => {
      this.setStatus('error', 'Connection error')
    }

    this.ws.onclose = (event: CloseEvent) => {
      this.ws = null
      if (this.isExplicitDisconnect) {
        this.setStatus('disconnected')
        return
      }

      if (this.hasSessionConflict) {
        this.setStatus('error', 'Session active in another tab')
        return
      }

      // Normal closure or network interruption
      this.setStatus('disconnected', event.reason || 'Connection lost')
      this.scheduleReconnect()
    }
  }

  private handleInboundMessage(msg: InboundMessage): void {
    if (msg.type === 'hello.ok') {
      const payload = msg.payload as HelloOKPayload
      this.setStatus('connected')
      // Update sessionToken in options for future reconnects if server issued a new one
      if (this.connectOptions && payload.sessionToken) {
        this.connectOptions.sessionToken = payload.sessionToken
      }
      for (const listener of this.helloOKListeners) {
        try {
          listener(payload)
        } catch (err) {
          console.error('Error in hello.ok listener:', err)
        }
      }
    } else if (msg.type === 'error') {
      const errPayload = msg.payload as ErrorPayload
      if (errPayload.code === ErrorCode.SessionAlreadyActive) {
        this.hasSessionConflict = true
      }
      for (const listener of this.errorListeners) {
        try {
          listener(errPayload)
        } catch (err) {
          console.error('Error in error listener:', err)
        }
      }
    }

    // Dispatch to registered type-specific listeners
    const listeners = this.messageListeners.get(msg.type)
    if (listeners) {
      for (const listener of listeners) {
        try {
          listener(msg.payload)
        } catch (err) {
          console.error(`Error in ${msg.type} listener:`, err)
        }
      }
    }
  }

  private scheduleReconnect(): void {
    if (this.isExplicitDisconnect || this.hasSessionConflict) return

    this.clearReconnectTimer()
    const delay = Math.min(
      this.initialBackoff * Math.pow(this.backoffFactor, this.reconnectAttempts),
      this.maxBackoff,
    )
    this.reconnectAttempts++

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      if (!this.isExplicitDisconnect && !this.hasSessionConflict) {
        this.establishConnection()
      }
    }, delay)
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }
}

// Default singleton instance
export const wsClient = new WebSocketClient()

