import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import App from './App'
import { sessionStore } from './services/storage'
import { ErrorCode, RoomStatePayload, WaitingRoomView } from './types/protocol'

class MockWebSocket {
  static instances: MockWebSocket[] = []
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
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

  it('allows a new player to enter a display name, connects, and reaches the Lobby', async () => {
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

    // Simulate backend response hello.ok and lobby.snapshot
    ws.receive('hello.ok', {
      playerId: 'player-yi-101',
      sessionToken: 'token-yi-secret',
      displayName: 'Admiral Yi',
      restoredState: null,
    })
    ws.receive('lobby.snapshot', {
      rooms: [],
      totalRooms: 0,
      maxRooms: 10,
    })

    // Player is now in the Lobby screen
    await waitFor(() => {
      expect(screen.getByText('Public Battle Lobby')).toBeInTheDocument()
      expect(screen.getByText('Admiral Yi')).toBeInTheDocument()
      expect(screen.getByText('No Active Waiting Rooms')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Create Game/i })).toBeInTheDocument()
    })

    // Verify localStorage was updated
    const saved = sessionStore.loadSession()
    expect(saved.displayName).toBe('Admiral Yi')
    expect(saved.sessionToken).toBe('token-yi-secret')
  })

  it('automatically restores session from localStorage across page refresh into Lobby', async () => {
    sessionStore.saveSession('saved-token-999', 'Commander Spock')

    render(<App />)

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

    // Server restores into lobby with 1 existing waiting room
    const waitingRoom: WaitingRoomView = {
      id: 'r_existing',
      hostName: 'Captain Kirk',
      createdAt: new Date().toISOString(),
    }

    ws.receive('hello.ok', {
      playerId: 'spock-id-77',
      sessionToken: 'saved-token-999',
      displayName: 'Commander Spock',
      restoredState: null,
    })
    ws.receive('lobby.snapshot', {
      rooms: [waitingRoom],
      totalRooms: 1,
      maxRooms: 10,
    })

    await waitFor(() => {
      expect(screen.getByText('Public Battle Lobby')).toBeInTheDocument()
      expect(screen.getByText('Commander Spock')).toBeInTheDocument()
      expect(screen.getByText('Captain Kirk')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Join' })).toBeInTheDocument()
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

    ws.receive('error', {
      code: ErrorCode.SessionAlreadyActive,
      message: 'Session is already active in another connection',
    })
    ws.close()

    await waitFor(() => {
      expect(screen.getByText(/Session Active in Another Tab/i)).toBeInTheDocument()
    })

    const freshBtn = screen.getByRole('button', { name: /Start Fresh Session/i })
    fireEvent.click(freshBtn)

    expect(sessionStore.loadSession().sessionToken).toBeNull()
    expect(screen.getByLabelText(/Callsign \/ Display Name/i)).toHaveValue('Captain Kirk')
  })

  it('allows host to create a room, see the Host Waiting Room UI, and cancel it', async () => {
    sessionStore.saveSession('token-host', 'Admiral Nimitz')
    render(<App />)

    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1))
    const ws = MockWebSocket.instances[0]

    ws.receive('hello.ok', {
      playerId: 'p_host',
      sessionToken: 'token-host',
      displayName: 'Admiral Nimitz',
      restoredState: null,
    })
    ws.receive('lobby.snapshot', {
      rooms: [],
      totalRooms: 0,
      maxRooms: 10,
    })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Create Game/i })).toBeInTheDocument()
    })

    // Click Create Game
    fireEvent.click(screen.getByRole('button', { name: /Create Game/i }))

    await waitFor(() => {
      const lastMsg = JSON.parse(ws.sentMessages[ws.sentMessages.length - 1])
      expect(lastMsg.type).toBe('lobby.create')
    })

    // Server sends room.state (waiting phase)
    const roomState: RoomStatePayload = {
      roomId: 'r_nimitz100',
      phase: 'waiting',
      role: 'host',
      host: { id: 'p_host', name: 'Admiral Nimitz' },
      createdAt: new Date().toISOString(),
    }
    ws.receive('room.state', roomState)

    // Now in WaitingRoomScreen
    await waitFor(() => {
      expect(screen.getByText('Host Waiting Room')).toBeInTheDocument()
      expect(screen.getByText('Waiting for an opponent')).toBeInTheDocument()
      expect(screen.getByText('r_nimitz100')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Cancel Game & Return to Lobby/i })).toBeInTheDocument()
    })

    // Click Cancel Game
    fireEvent.click(screen.getByRole('button', { name: /Cancel Game & Return to Lobby/i }))

    await waitFor(() => {
      const lastMsg = JSON.parse(ws.sentMessages[ws.sentMessages.length - 1])
      expect(lastMsg.type).toBe('lobby.cancel')
    })

    // Server cancels room and sends updated lobby snapshot
    ws.receive('lobby.snapshot', {
      rooms: [],
      totalRooms: 0,
      maxRooms: 10,
    })

    // Returned back to LobbyScreen
    await waitFor(() => {
      expect(screen.getByText('Public Battle Lobby')).toBeInTheDocument()
      expect(screen.getByText('No Active Waiting Rooms')).toBeInTheDocument()
    })
  })

  it('updates live with lobby.update and allows joining a room to advance to ready-up', async () => {
    sessionStore.saveSession('token-guest', 'Captain Picard')
    render(<App />)

    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1))
    const ws = MockWebSocket.instances[0]

    ws.receive('hello.ok', {
      playerId: 'p_guest',
      sessionToken: 'token-guest',
      displayName: 'Captain Picard',
      restoredState: null,
    })
    ws.receive('lobby.snapshot', {
      rooms: [],
      totalRooms: 0,
      maxRooms: 10,
    })

    await waitFor(() => {
      expect(screen.getByText('No Active Waiting Rooms')).toBeInTheDocument()
    })

    // Another player creates a room -> lobby.update broadcast arrives
    const newRoom: WaitingRoomView = {
      id: 'r_host999',
      hostName: 'Admiral Riker',
      createdAt: new Date().toISOString(),
    }
    ws.receive('lobby.update', {
      action: 'upsert',
      room: newRoom,
      totalRooms: 1,
      maxRooms: 10,
    })

    // Riker's room appears live without manual refresh
    await waitFor(() => {
      expect(screen.getByText('Admiral Riker')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Join' })).toBeInTheDocument()
    })

    // Click Join
    fireEvent.click(screen.getByRole('button', { name: 'Join' }))

    await waitFor(() => {
      const lastMsg = JSON.parse(ws.sentMessages[ws.sentMessages.length - 1])
      expect(lastMsg.type).toBe('lobby.join')
      expect(lastMsg.payload.roomId).toBe('r_host999')
    })

    // Server confirms join -> room.state with phase: ready-up
    const joinedState: RoomStatePayload = {
      roomId: 'r_host999',
      phase: 'ready-up',
      role: 'guest',
      opponentName: 'Admiral Riker',
      host: { id: 'p_riker', name: 'Admiral Riker' },
      guest: { id: 'p_guest', name: 'Captain Picard' },
      createdAt: new Date().toISOString(),
    }
    ws.receive('room.state', joinedState)

    // ReadyUpScreen is displayed
    await waitFor(() => {
      expect(screen.getByText('Pre-Game Readiness Check')).toBeInTheDocument()
      expect(screen.getByText('Sector Engagement Locked')).toBeInTheDocument()
      expect(screen.getByText('Admiral Riker')).toBeInTheDocument()
      expect(screen.getByText('Captain Picard')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Ready for Battle/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Leave Room/i })).toBeInTheDocument()
    })

    // Guest clicks Ready
    fireEvent.click(screen.getByRole('button', { name: /Ready for Battle/i }))
    await waitFor(() => {
      const lastMsg = JSON.parse(ws.sentMessages[ws.sentMessages.length - 1])
      expect(lastMsg.type).toBe('room.ready')
    })

    // Both players ready -> placement phase transition
    ws.receive('room.readyState', {
      roomId: 'r_host999',
      hostReady: true,
      guestReady: true,
    })
    ws.receive('room.state', {
      ...joinedState,
      phase: 'placement',
      hostReady: true,
      guestReady: true,
    })

    // Placement screen is displayed
    await waitFor(() => {
      expect(screen.getByText('Phase: Fleet Placement')).toBeInTheDocument()
      expect(screen.getByText('Fleet Dock Initializing')).toBeInTheDocument()
    })
  })
})
