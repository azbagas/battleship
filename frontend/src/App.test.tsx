import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import App from './App'
import { sessionStore } from './services/storage'
import { ErrorCode } from './types/protocol'

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
    setTimeout(() => {
      if (this.onopen && this.readyState === MockWebSocket.OPEN) {
        act(() => {
          this.onopen?.()
        })
      }
    }, 0)
  }

  send(data: string) {
    this.sentMessages.push(data)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    if (this.onclose) {
      act(() => {
        this.onclose?.({ reason: 'closed' })
      })
    }
  }

  receive(type: string, payload: unknown) {
    if (this.onmessage) {
      act(() => {
        this.onmessage?.({ data: JSON.stringify({ type, payload }) })
      })
    }
  }
}

describe('App & Session Lifecycle', () => {
  let originalWebSocket: typeof globalThis.WebSocket

  beforeEach(() => {
    localStorage.clear()
    MockWebSocket.instances = []
    originalWebSocket = globalThis.WebSocket
    // @ts-expect-error - mock class assignment
    globalThis.WebSocket = MockWebSocket
  })

  afterEach(() => {
    globalThis.WebSocket = originalWebSocket
  })

  it('allows a new player to enter a display name, connects, and receives session identity', async () => {
    render(<App />)

    // Verify initial home screen
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Battleship')
    const input = screen.getByLabelText(/Callsign \/ Display Name/i)

    fireEvent.change(input, { target: { value: 'Admiral Yi' } })
    fireEvent.click(screen.getByRole('button', { name: /Play Battleship/i }))

    // Wait for mock websocket to be created
    await waitFor(() => {
      expect(MockWebSocket.instances.length).toBe(1)
    })

    const ws = MockWebSocket.instances[0]
    await waitFor(() => {
      expect(ws.sentMessages.length).toBe(1)
    })

    const helloSent = JSON.parse(ws.sentMessages[0])
    expect(helloSent.type).toBe('hello')
    expect(helloSent.payload.name).toBe('Admiral Yi')

    // Simulate backend response hello.ok
    ws.receive('hello.ok', {
      playerId: 'player-yi-101',
      sessionToken: 'token-yi-secret',
      displayName: 'Admiral Yi',
      restoredState: null,
    })

    // Commander identity card should now be visible
    await waitFor(() => {
      expect(screen.getByText('Admiral Yi')).toBeInTheDocument()
      expect(screen.getByText(/ID: player-y.../i)).toBeInTheDocument()
      expect(screen.getByText(/Connected/i)).toBeInTheDocument()
    })

    // Verify localStorage was updated
    const saved = sessionStore.loadSession()
    expect(saved.displayName).toBe('Admiral Yi')
    expect(saved.sessionToken).toBe('token-yi-secret')
  })

  it('automatically restores session from localStorage across page refresh', async () => {
    // Pre-populate localStorage as if player refreshed page
    sessionStore.saveSession('saved-token-999', 'Commander Spock')

    render(<App />)

    // Should immediately attempt to reconnect with saved credentials
    await waitFor(() => {
      expect(MockWebSocket.instances.length).toBe(1)
    })

    const ws = MockWebSocket.instances[0]
    await waitFor(() => {
      expect(ws.sentMessages.length).toBe(1)
    })

    const helloSent = JSON.parse(ws.sentMessages[0])
    expect(helloSent.type).toBe('hello')
    expect(helloSent.payload.name).toBe('Commander Spock')
    expect(helloSent.payload.sessionToken).toBe('saved-token-999')

    // Simulate backend confirming restoration
    ws.receive('hello.ok', {
      playerId: 'spock-id-77',
      sessionToken: 'saved-token-999',
      displayName: 'Commander Spock',
      restoredState: null,
    })

    await waitFor(() => {
      expect(screen.getByText('Commander Spock')).toBeInTheDocument()
      expect(screen.getByText(/Session restored across refresh/i)).toBeInTheDocument()
    })
  })

  it('handles SESSION_ALREADY_ACTIVE by displaying warning and allowing fresh session', async () => {
    sessionStore.saveSession('duplicate-token', 'Captain Kirk')

    render(<App />)

    await waitFor(() => {
      expect(MockWebSocket.instances.length).toBe(1)
    })

    const ws = MockWebSocket.instances[0]
    await waitFor(() => {
      expect(ws.sentMessages.length).toBe(1)
    })

    // Server rejects duplicate session
    ws.receive('error', {
      code: ErrorCode.SessionAlreadyActive,
      message: 'Session is already active in another connection',
    })
    ws.close()

    await waitFor(() => {
      expect(screen.getByText(/Session Active in Another Tab/i)).toBeInTheDocument()
    })

    // User clicks "Start Fresh Session"
    const freshBtn = screen.getByRole('button', { name: /Start Fresh Session/i })
    fireEvent.click(freshBtn)

    // Token should be cleared from storage
    expect(sessionStore.loadSession().sessionToken).toBeNull()
    // Returns to form with saved name prefilled
    expect(screen.getByLabelText(/Callsign \/ Display Name/i)).toHaveValue('Captain Kirk')
  })
})
