import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { WebSocketClient } from './wsClient'
import { ErrorCode, HelloOKPayload, ErrorPayload } from '../types/protocol'

// Mock WebSocket implementation
class MockWebSocket {
  static instances: MockWebSocket[] = []
  static OPEN = 1
  static CLOSED = 3

  readyState = MockWebSocket.OPEN
  url: string
  sentMessages: string[] = []

  onopen: (() => void) | null = null
  onclose: ((event: { reason?: string }) => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
    // Automatically trigger onopen next tick if attached
    setTimeout(() => {
      if (this.onopen && this.readyState === MockWebSocket.OPEN) {
        this.onopen()
      }
    }, 0)
  }

  send(data: string) {
    this.sentMessages.push(data)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    if (this.onclose) {
      this.onclose({ reason: 'normal' })
    }
  }

  // Test helpers to simulate server messages
  receiveServerMessage(type: string, payload: unknown) {
    if (this.onmessage) {
      this.onmessage({ data: JSON.stringify({ type, payload }) })
    }
  }
}

describe('WebSocketClient', () => {
  let originalWebSocket: typeof globalThis.WebSocket

  beforeEach(() => {
    vi.useFakeTimers()
    MockWebSocket.instances = []
    originalWebSocket = globalThis.WebSocket
    // @ts-expect-error - mock class assignment
    globalThis.WebSocket = MockWebSocket
  })

  afterEach(() => {
    globalThis.WebSocket = originalWebSocket
    vi.useRealTimers()
  })

  it('connects and immediately sends hello handshake on open', async () => {
    const client = new WebSocketClient()
    client.connect({ displayName: 'Captain Nemo', sessionToken: 'tok_abc' })

    expect(client.getStatus()).toBe('connecting')
    expect(MockWebSocket.instances.length).toBe(1)

    // Advance timers so onopen runs
    await vi.runAllTimersAsync()

    const mockWs = MockWebSocket.instances[0]
    expect(mockWs.sentMessages.length).toBe(1)
    const sent = JSON.parse(mockWs.sentMessages[0])
    expect(sent.type).toBe('hello')
    expect(sent.payload).toEqual({
      name: 'Captain Nemo',
      sessionToken: 'tok_abc',
    })
  })

  it('updates status to connected upon receiving hello.ok', async () => {
    const client = new WebSocketClient()
    const helloOKListener = vi.fn()
    client.onHelloOK(helloOKListener)

    client.connect({ displayName: 'Captain Nemo' })
    await vi.runAllTimersAsync()

    const mockWs = MockWebSocket.instances[0]
    const helloOKPayload: HelloOKPayload = {
      playerId: 'p-1',
      sessionToken: 'tok-new',
      displayName: 'Captain Nemo',
      restoredState: null,
    }

    mockWs.receiveServerMessage('hello.ok', helloOKPayload)

    expect(client.getStatus()).toBe('connected')
    expect(helloOKListener).toHaveBeenCalledWith(helloOKPayload)
  })

  it('identifies SESSION_ALREADY_ACTIVE error and halts reconnection', async () => {
    const client = new WebSocketClient()
    const errorListener = vi.fn()
    client.onError(errorListener)

    client.connect({ displayName: 'Admiral', sessionToken: 'tok_active' })
    await vi.runAllTimersAsync()

    const mockWs = MockWebSocket.instances[0]
    const errPayload: ErrorPayload = {
      code: ErrorCode.SessionAlreadyActive,
      message: 'Session is already active in another connection',
    }

    mockWs.receiveServerMessage('error', errPayload)
    // Server then closes connection
    mockWs.close()

    expect(client.isConflict()).toBe(true)
    expect(errorListener).toHaveBeenCalledWith(errPayload)

    // Fast-forward timers: no new WebSocket instance should be created!
    await vi.advanceTimersByTimeAsync(15000)
    expect(MockWebSocket.instances.length).toBe(1)
  })

  it('reconnects with exponential backoff on unexpected disconnect', async () => {
    const client = new WebSocketClient()
    client.connect({ displayName: 'Commander' })
    await vi.runAllTimersAsync()

    expect(MockWebSocket.instances.length).toBe(1)
    const firstWs = MockWebSocket.instances[0]

    // Simulate unexpected drop
    firstWs.close()
    expect(client.getStatus()).toBe('disconnected')

    // Advance 1 second (initial backoff)
    await vi.advanceTimersByTimeAsync(1100)
    expect(MockWebSocket.instances.length).toBe(2)
  })

  it('does not auto-reconnect after explicit disconnect()', async () => {
    const client = new WebSocketClient()
    client.connect({ displayName: 'Commander' })
    await vi.runAllTimersAsync()

    client.disconnect()
    expect(client.getStatus()).toBe('disconnected')

    await vi.advanceTimersByTimeAsync(15000)
    expect(MockWebSocket.instances.length).toBe(1)
  })

  it('delivers typed messages to registered listeners', async () => {
    const client = new WebSocketClient()
    const customListener = vi.fn()
    const unsubscribe = client.onMessage('custom.event', customListener)

    client.connect({ displayName: 'Commander' })
    await vi.runAllTimersAsync()

    const mockWs = MockWebSocket.instances[0]
    mockWs.receiveServerMessage('custom.event', { foo: 'bar' })

    expect(customListener).toHaveBeenCalledWith({ foo: 'bar' })

    unsubscribe()
    mockWs.receiveServerMessage('custom.event', { foo: 'baz' })
    expect(customListener).toHaveBeenCalledTimes(1)
  })
})

