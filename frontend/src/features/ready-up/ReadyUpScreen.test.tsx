import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ReadyUpScreen } from './ReadyUpScreen'
import { LobbyContext, LobbyContextValue } from '../../context/LobbyContext'
import { SessionContext, SessionContextValue } from '../../context/SessionContext'
import { WebSocketClient } from '../../services/wsClient'
import { RoomStatePayload } from '../../types/protocol'

describe('ReadyUpScreen', () => {
  const baseSession: SessionContextValue = {
    connectionStatus: 'connected',
    player: {
      playerId: 'p1',
      sessionToken: 'tok1',
      displayName: 'Captain Kirk',
      restoredState: null,
    },
    savedDisplayName: 'Captain Kirk',
    errorMessage: null,
    isSessionConflict: false,
    isRestored: false,
    login: vi.fn(),
    logout: vi.fn(),
    retry: vi.fn(),
    clearConflictAndReset: vi.fn(),
    client: {} as WebSocketClient,
  }

  const baseRoom: RoomStatePayload = {
    roomId: 'r_test_123',
    phase: 'ready-up',
    role: 'host',
    host: { id: 'p1', name: 'Captain Kirk' },
    guest: { id: 'p2', name: 'Commander Spock' },
    createdAt: new Date().toISOString(),
  }

  const createMockLobby = (overrides?: Partial<LobbyContextValue>): LobbyContextValue => ({
    rooms: [],
    totalRooms: 1,
    maxRooms: 10,
    activeRoom: baseRoom,
    isCreating: false,
    joiningRoomId: null,
    isCancelling: false,
    lobbyError: null,
    isRoomCapReached: false,
    createRoom: vi.fn(),
    joinRoom: vi.fn(),
    cancelRoom: vi.fn(),
    clearLobbyError: vi.fn(),
    readyState: { hostReady: false, guestReady: false },
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
    isMyTurn: false,
    targetShots: new Map(),
    ownShots: new Map(),
    opponentSunkShips: [],
    ownSunkShips: [],
    recentEvents: [],
    gameOverResult: null,
    fireShot: vi.fn(),
    opponentDisconnectDeadline: null,
    serverRestartNotice: null,
    clearServerRestartNotice: vi.fn(),
    forfeitGame: vi.fn(),
    rematchStatus: 'idle',
    rematchCancelReason: null,
    acceptRematch: vi.fn(),
    declineRematch: vi.fn(),
    ...overrides,
  })

  const renderComponent = (lobbyValue: LobbyContextValue, sessionValue = baseSession) => {
    return render(
      <SessionContext.Provider value={sessionValue}>
        <LobbyContext.Provider value={lobbyValue}>
          <ReadyUpScreen />
        </LobbyContext.Provider>
      </SessionContext.Provider>,
    )
  }

  it('renders commanders roster with correct roles and standby status', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    expect(screen.getByText('Pre-Game Readiness Check')).toBeInTheDocument()
    expect(screen.getByText('Captain Kirk')).toBeInTheDocument()
    expect(screen.getByText('Commander Spock')).toBeInTheDocument()
    expect(screen.getByText('YOU')).toBeInTheDocument()

    const standbyBadges = screen.getAllByText('STANDBY')
    expect(standbyBadges.length).toBe(2)

    expect(screen.getByRole('button', { name: /Ready for Battle/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Leave Room/i })).toBeInTheDocument()
  })

  it('calls toggleReady when Ready for Battle is clicked', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    const readyBtn = screen.getByRole('button', { name: /Ready for Battle/i })
    fireEvent.click(readyBtn)

    expect(lobby.toggleReady).toHaveBeenCalledTimes(1)
  })

  it('shows Cancel Ready when current user is ready and calls toggleReady on click', () => {
    const lobby = createMockLobby({
      readyState: { hostReady: true, guestReady: false },
    })
    renderComponent(lobby)

    expect(screen.getByText('READY')).toBeInTheDocument()
    expect(screen.getByText('STANDBY')).toBeInTheDocument()

    const cancelReadyBtn = screen.getByRole('button', { name: /Cancel Ready/i })
    expect(cancelReadyBtn).toBeInTheDocument()

    fireEvent.click(cancelReadyBtn)
    expect(lobby.toggleReady).toHaveBeenCalledTimes(1)
  })

  it('calls leaveRoom when Leave Room button is clicked', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    const leaveBtn = screen.getByRole('button', { name: /Leave Room/i })
    fireEvent.click(leaveBtn)

    expect(lobby.leaveRoom).toHaveBeenCalledTimes(1)
  })

  it('disables controls and shows spinner during isTogglingReady', () => {
    const lobby = createMockLobby({ isTogglingReady: true })
    renderComponent(lobby)

    const readyBtn = screen.getByRole('button', { name: /Synchronizing/i })
    expect(readyBtn).toBeDisabled()

    const leaveBtn = screen.getByRole('button', { name: /Leave Room/i })
    expect(leaveBtn).toBeDisabled()
  })

  it('shows leaving state when isLeavingRoom is true', () => {
    const lobby = createMockLobby({ isLeavingRoom: true })
    renderComponent(lobby)

    expect(screen.getByText('Leaving sector...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Ready for Battle/i })).toBeDisabled()
  })

  it('renders opponent disconnect banner when opponent disconnect deadline is present', () => {
    const deadline = new Date(Date.now() + 45000).toISOString()
    const lobby = createMockLobby({ opponentDisconnectDeadline: deadline })
    renderComponent(lobby)

    expect(screen.getByText(/CONNECTION DROPPED/i)).toBeInTheDocument()
    expect(screen.getByText(/Commander Spock is offline/i)).toBeInTheDocument()
  })
})
