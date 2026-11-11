import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { OpponentDisconnectBanner } from './OpponentDisconnectBanner'
import { LobbyContext, LobbyContextValue } from '../context/LobbyContext'

describe('OpponentDisconnectBanner', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  const mockLobbyValue = (opponentDisconnectDeadline: string | null): LobbyContextValue => ({
    rooms: [],
    totalRooms: 0,
    maxRooms: 10,
    activeRoom: null,
    isCreating: false,
    joiningRoomId: null,
    isCancelling: false,
    lobbyError: null,
    isRoomCapReached: false,
    createRoom: vi.fn(),
    joinRoom: vi.fn(),
    cancelRoom: vi.fn(),
    clearLobbyError: vi.fn(),
    readyState: null,
    isTogglingReady: false,
    isLeavingRoom: false,
    toggleReady: vi.fn(),
    leaveRoom: vi.fn(),
    isFleetConfirmed: false,
    isOpponentConfirmed: false,
    ownFleet: [],
    placementDeadline: null,
    submitFleet: vi.fn(),
    turnPlayerId: null,
    turnDeadline: null,
    opponentDisconnectDeadline,
    serverRestartNotice: null,
    clearServerRestartNotice: vi.fn(),
    forfeitGame: vi.fn(),
    isMyTurn: false,
    targetShots: new Map(),
    ownShots: new Map(),
    opponentSunkShips: [],
    ownSunkShips: [],
    recentEvents: [],
    gameOverResult: null,
    fireShot: vi.fn(),
    rematchStatus: 'idle',
    rematchCancelReason: null,
    acceptRematch: vi.fn(),
    declineRematch: vi.fn(),
  })

  it('renders nothing when no deadline is set', () => {
    const { container } = render(
      <LobbyContext.Provider value={mockLobbyValue(null)}>
        <OpponentDisconnectBanner opponentName="Admiral Alpha" />
      </LobbyContext.Provider>,
    )

    expect(container.firstChild).toBeNull()
  })

  it('renders countdown alert with opponent name and live ticking timer', () => {
    const now = 1000000000000
    vi.setSystemTime(now)
    const deadline = new Date(now + 45000).toISOString()

    render(
      <LobbyContext.Provider value={mockLobbyValue(deadline)}>
        <OpponentDisconnectBanner opponentName="Captain Jack" />
      </LobbyContext.Provider>,
    )

    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('CONNECTION DROPPED')).toBeInTheDocument()
    expect(screen.getByText(/Captain Jack is offline/i)).toBeInTheDocument()
    expect(screen.getByText('45s')).toBeInTheDocument()

    // Advance 5 seconds
    act(() => {
      vi.advanceTimersByTime(5000)
    })

    expect(screen.getByText('40s')).toBeInTheDocument()
  })

  it('applies urgent styling when remaining time is 10 seconds or less', () => {
    const now = 1000000000000
    vi.setSystemTime(now)
    const deadline = new Date(now + 8000).toISOString()

    render(
      <LobbyContext.Provider value={mockLobbyValue(deadline)}>
        <OpponentDisconnectBanner opponentName="Commander Jane" />
      </LobbyContext.Provider>,
    )

    const timer = screen.getByText('8s')
    expect(timer.className).toContain('text-rose-400')
    expect(timer.className).toContain('animate-pulse')
  })

  it('renders pre-game room-return description for host in ready-up phase', () => {
    const deadline = new Date(Date.now() + 30000).toISOString()
    const lobby = mockLobbyValue(deadline)
    lobby.activeRoom = {
      roomId: 'r_ready1',
      phase: 'ready-up',
      role: 'host',
      host: { id: 'p1', name: 'Host Commander' },
      guest: { id: 'p2', name: 'Guest Commander' },
      createdAt: new Date().toISOString(),
    }

    render(
      <LobbyContext.Provider value={lobby}>
        <OpponentDisconnectBanner opponentName="Guest Commander" />
      </LobbyContext.Provider>,
    )

    expect(screen.getByText('Timeout in:')).toBeInTheDocument()
    expect(
      screen.getByText(/this room will return to the public waiting lobby/i),
    ).toBeInTheDocument()
  })

  it('renders pre-game room-close description for guest in ready-up phase', () => {
    const deadline = new Date(Date.now() + 30000).toISOString()
    const lobby = mockLobbyValue(deadline)
    lobby.activeRoom = {
      roomId: 'r_ready1',
      phase: 'ready-up',
      role: 'guest',
      host: { id: 'p1', name: 'Host Commander' },
      guest: { id: 'p2', name: 'Guest Commander' },
      createdAt: new Date().toISOString(),
    }

    render(
      <LobbyContext.Provider value={lobby}>
        <OpponentDisconnectBanner opponentName="Host Commander" />
      </LobbyContext.Provider>,
    )

    expect(screen.getByText('Timeout in:')).toBeInTheDocument()
    expect(
      screen.getByText(/the room will close and you will return to the lobby/i),
    ).toBeInTheDocument()
  })

  it('renders forfeit victory description during battle/placement phase', () => {
    const deadline = new Date(Date.now() + 30000).toISOString()
    const lobby = mockLobbyValue(deadline)
    lobby.activeRoom = {
      roomId: 'r_battle1',
      phase: 'battle',
      role: 'host',
      host: { id: 'p1', name: 'Host Commander' },
      guest: { id: 'p2', name: 'Guest Commander' },
      createdAt: new Date().toISOString(),
    }

    render(
      <LobbyContext.Provider value={lobby}>
        <OpponentDisconnectBanner opponentName="Guest Commander" />
      </LobbyContext.Provider>,
    )

    expect(screen.getByText('Forfeit in:')).toBeInTheDocument()
    expect(screen.getByText(/victory is awarded to you/i)).toBeInTheDocument()
  })
})
