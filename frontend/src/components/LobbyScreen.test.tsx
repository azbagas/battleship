import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LobbyScreen } from './LobbyScreen'
import { LobbyContext, LobbyContextValue } from '../context/LobbyContext'
import { SessionContext, SessionContextValue } from '../context/SessionContext'
import { WebSocketClient } from '../services/wsClient'
import { WaitingRoomView } from '../types/protocol'

describe('LobbyScreen', () => {
  const defaultMockLobby: LobbyContextValue = {
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
    isMyTurn: false,
    targetShots: new Map(),
    ownShots: new Map(),
    opponentSunkShips: [],
    ownSunkShips: [],
    recentEvents: [],
    gameOverResult: null,
    fireShot: vi.fn(),
  }

  const defaultMockSession: SessionContextValue = {
    connectionStatus: 'connected',
    player: {
      playerId: 'p1',
      sessionToken: 'token-1',
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

  const renderComponent = (
    lobbyOverrides?: Partial<LobbyContextValue>,
    sessionOverrides?: Partial<SessionContextValue>,
  ) => {
    const lobbyValue = { ...defaultMockLobby, ...lobbyOverrides }
    const sessionValue = { ...defaultMockSession, ...sessionOverrides }

    return render(
      <SessionContext.Provider value={sessionValue}>
        <LobbyContext.Provider value={lobbyValue}>
          <LobbyScreen />
        </LobbyContext.Provider>
      </SessionContext.Provider>,
    )
  }

  it('renders empty state when no rooms are available', () => {
    renderComponent({ rooms: [] })

    expect(screen.getByTestId('lobby-empty-state')).toBeInTheDocument()
    expect(screen.getByText('No Active Waiting Rooms')).toBeInTheDocument()
    expect(screen.getByText('Admiral Nimitz')).toBeInTheDocument()
  })

  it('triggers createRoom when Create Game button is clicked', () => {
    const createRoom = vi.fn()
    renderComponent({ createRoom, rooms: [] })

    const buttons = screen.getAllByRole('button', { name: /create/i })
    expect(buttons.length).toBeGreaterThan(0)
    fireEvent.click(buttons[0])

    expect(createRoom).toHaveBeenCalled()
  })

  it('renders waiting rooms list with host name, wait duration, and join action', () => {
    const joinRoom = vi.fn()
    const sampleRooms: WaitingRoomView[] = [
      {
        id: 'r_1',
        hostName: 'Captain Kirk',
        createdAt: new Date(Date.now() - 35000).toISOString(),
      },
      {
        id: 'r_2',
        hostName: 'Spock',
        createdAt: new Date(Date.now() - 10000).toISOString(),
      },
    ]

    renderComponent({ rooms: sampleRooms, totalRooms: 2, joinRoom })

    expect(screen.getByText('Captain Kirk')).toBeInTheDocument()
    expect(screen.getByText('Spock')).toBeInTheDocument()
    expect(screen.getByTestId('wait-duration-r_1')).toBeInTheDocument()
    expect(screen.getByTestId('wait-duration-r_2')).toBeInTheDocument()

    const joinButtons = screen.getAllByRole('button', { name: 'Join' })
    expect(joinButtons).toHaveLength(2)

    fireEvent.click(joinButtons[0])
    expect(joinRoom).toHaveBeenCalledWith('r_1')
  })

  it('disables Create Game and shows warning banner when 10-room cap is reached', () => {
    renderComponent({
      totalRooms: 10,
      maxRooms: 10,
      isRoomCapReached: true,
      rooms: [],
    })

    const createBtn = screen.getByRole('button', { name: /create game/i })
    expect(createBtn).toBeDisabled()

    expect(screen.getByText(/maximum capacity reached \(10 rooms\)/i)).toBeInTheDocument()
  })

  it('displays and dismisses lobby error alert', () => {
    const clearLobbyError = vi.fn()
    renderComponent({
      lobbyError: 'Game is no longer available.',
      clearLobbyError,
    })

    expect(screen.getByText('Game is no longer available.')).toBeInTheDocument()
    const dismissBtn = screen.getByRole('button', { name: /dismiss/i })
    fireEvent.click(dismissBtn)

    expect(clearLobbyError).toHaveBeenCalled()
  })
})
