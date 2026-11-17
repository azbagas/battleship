import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { GameOverScreen } from './GameOverScreen'
import { LobbyContext, LobbyContextValue } from '../../context/LobbyContext'
import { SessionContext, SessionContextValue } from '../../context/SessionContext'
import { WebSocketClient } from '../../services/wsClient'
import { RoomStatePayload } from '../../types/protocol'

describe('GameOverScreen', () => {
  const baseSession: SessionContextValue = {
    connectionStatus: 'connected',
    player: {
      playerId: 'p_host',
      sessionToken: 'tok_host',
      displayName: 'Admiral Nimitz',
      restoredState: null,
    },
    savedDisplayName: 'Admiral Nimitz',
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
    roomId: 'r_finished_sector_123456',
    phase: 'finished',
    role: 'host',
    host: { id: 'p_host', name: 'Admiral Nimitz' },
    guest: { id: 'p_guest', name: 'Admiral Yamamoto' },
    opponentName: 'Admiral Yamamoto',
    createdAt: new Date().toISOString(),
    winnerPlayerId: 'p_host',
    endReason: 'all_sunk',
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
    readyState: null,
    isTogglingReady: false,
    isLeavingRoom: false,
    toggleReady: vi.fn(),
    leaveRoom: vi.fn(),
    isFleetConfirmed: true,
    isOpponentConfirmed: true,
    ownFleet: [],
    placementDeadline: null,
    submitFleet: vi.fn(),
    turnPlayerId: null,
    turnDeadline: null,
    isMyTurn: false,
    targetShots: new Map(),
    ownShots: new Map(),
    opponentSunkShips: [
      { type: 'carrier', cells: [] },
      { type: 'battleship', cells: [] },
      { type: 'cruiser', cells: [] },
      { type: 'submarine', cells: [] },
      { type: 'destroyer', cells: [] },
    ],
    ownSunkShips: [
      { type: 'destroyer', cells: [] },
      { type: 'submarine', cells: [] },
    ],
    recentEvents: [],
    gameOverResult: {
      winnerPlayerId: 'p_host',
      reason: 'all_sunk',
      totalShots: 32,
    },
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
          <GameOverScreen />
        </LobbyContext.Provider>
      </SessionContext.Provider>,
    )
  }

  it('renders victory outcome and statistics when the user is the winner', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    expect(screen.getByText('VICTORY ACHIEVED')).toBeInTheDocument()
    expect(screen.getByText('Hostile fleet successfully eliminated.')).toBeInTheDocument()
    expect(screen.getByText('Admiral Nimitz')).toBeInTheDocument()
    expect(screen.getByText('All Ships Sunk')).toBeInTheDocument()
    expect(screen.getByText('2/5 Sunk')).toBeInTheDocument()
    expect(screen.getByText('5/5 Sunk')).toBeInTheDocument()
    expect(screen.getByText('32')).toBeInTheDocument()
  })

  it('renders defeat outcome when the user is the loser', () => {
    const lobby = createMockLobby({
      gameOverResult: {
        winnerPlayerId: 'p_guest',
        reason: 'all_sunk',
        totalShots: 40,
      },
    })
    renderComponent(lobby)

    expect(screen.getByText('DEFEAT — FLEET LOST')).toBeInTheDocument()
    expect(screen.getByText('All friendly naval forces have been destroyed.')).toBeInTheDocument()
    expect(screen.getByText('Admiral Yamamoto')).toBeInTheDocument()
  })

  it('renders abandoned outcome when both players disconnected', () => {
    const lobby = createMockLobby({
      gameOverResult: {
        winnerPlayerId: '',
        reason: 'abandoned',
        totalShots: 10,
      },
    })
    renderComponent(lobby)

    expect(screen.getByText('ENGAGEMENT ABANDONED')).toBeInTheDocument()
    expect(screen.getByText('None (Abandoned)')).toBeInTheDocument()
    expect(screen.getByText('Both Disconnected')).toBeInTheDocument()
  })

  it('hides the Rematch button on forfeit', () => {
    const lobby = createMockLobby({
      gameOverResult: {
        winnerPlayerId: 'p_host',
        reason: 'forfeit',
        totalShots: 15,
      },
    })
    renderComponent(lobby)

    expect(screen.getByText('Enemy Forfeited')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Request Rematch/i })).toBeNull()
    expect(screen.queryByText(/Combat Rematch/i)).toBeNull()
  })

  it('hides the Rematch button on disconnect timeout', () => {
    const lobby = createMockLobby({
      gameOverResult: {
        winnerPlayerId: 'p_host',
        reason: 'disconnect_timeout',
        totalShots: 18,
      },
    })
    renderComponent(lobby)

    expect(screen.getByText('Opponent Disconnected')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Request Rematch/i })).toBeNull()
  })

  it('shows Request Rematch button when idle and calls acceptRematch on click', () => {
    const acceptRematchMock = vi.fn()
    const lobby = createMockLobby({
      rematchStatus: 'idle',
      acceptRematch: acceptRematchMock,
    })
    renderComponent(lobby)

    const rematchBtn = screen.getByRole('button', { name: /Request Rematch/i })
    expect(rematchBtn).toBeInTheDocument()

    fireEvent.click(rematchBtn)
    expect(acceptRematchMock).toHaveBeenCalledTimes(1)
  })

  it('shows waiting status when rematch has been requested by self', () => {
    const lobby = createMockLobby({
      rematchStatus: 'offered_by_me',
    })
    renderComponent(lobby)

    expect(
      screen.getByText(/Rematch request sent\. Awaiting opponent response\.\.\./i),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Request Rematch/i })).toBeNull()
  })

  it('shows incoming challenge with Accept and Decline buttons when opponent requested rematch', () => {
    const acceptRematchMock = vi.fn()
    const declineRematchMock = vi.fn()
    const lobby = createMockLobby({
      rematchStatus: 'offered_by_opponent',
      acceptRematch: acceptRematchMock,
      declineRematch: declineRematchMock,
    })
    renderComponent(lobby)

    expect(
      screen.getByText(/Admiral Yamamoto has requested a rematch!/i),
    ).toBeInTheDocument()

    const acceptBtn = screen.getByRole('button', { name: /Accept Rematch/i })
    const declineBtn = screen.getByRole('button', { name: /Decline/i })

    expect(acceptBtn).toBeInTheDocument()
    expect(declineBtn).toBeInTheDocument()

    fireEvent.click(acceptBtn)
    expect(acceptRematchMock).toHaveBeenCalledTimes(1)

    fireEvent.click(declineBtn)
    expect(declineRematchMock).toHaveBeenCalledTimes(1)
  })

  it('displays cancellation reason when rematch offer is cancelled', () => {
    const lobby = createMockLobby({
      rematchStatus: 'cancelled',
      rematchCancelReason: 'declined',
    })
    renderComponent(lobby)

    expect(screen.getByText('Rematch offer was declined.')).toBeInTheDocument()
  })

  it('triggers leaveRoom when Return to Lobby button is clicked', () => {
    const leaveRoomMock = vi.fn()
    const lobby = createMockLobby({
      leaveRoom: leaveRoomMock,
    })
    renderComponent(lobby)

    const returnBtn = screen.getByRole('button', { name: /Return to Lobby/i })
    expect(returnBtn).toBeInTheDocument()

    fireEvent.click(returnBtn)
    expect(leaveRoomMock).toHaveBeenCalledTimes(1)
  })
})

