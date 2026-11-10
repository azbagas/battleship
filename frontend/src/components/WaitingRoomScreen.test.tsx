import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { WaitingRoomScreen } from './WaitingRoomScreen'
import { LobbyContext, LobbyContextValue } from '../context/LobbyContext'
import { SessionContext, SessionContextValue } from '../context/SessionContext'
import { WebSocketClient } from '../services/wsClient'
import { RoomStatePayload } from '../types/protocol'

describe('WaitingRoomScreen', () => {
  const mockActiveRoom: RoomStatePayload = {
    roomId: 'r_alpha123',
    phase: 'waiting',
    role: 'host',
    host: { id: 'p1', name: 'Admiral Nimitz' },
    createdAt: new Date(Date.now() - 42000).toISOString(),
  }

  const defaultMockLobby: LobbyContextValue = {
    rooms: [],
    totalRooms: 1,
    maxRooms: 10,
    activeRoom: mockActiveRoom,
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
          <WaitingRoomScreen />
        </LobbyContext.Provider>
      </SessionContext.Provider>,
    )
  }

  it('renders host waiting room with host details, sector ID, and wait duration', () => {
    renderComponent()

    expect(screen.getByText('Host Waiting Room')).toBeInTheDocument()
    expect(screen.getByText('Waiting for an opponent')).toBeInTheDocument()
    expect(screen.getByText('Admiral Nimitz')).toBeInTheDocument()
    expect(screen.getByText('r_alpha123')).toBeInTheDocument()
    expect(screen.getByTestId('host-wait-duration')).toBeInTheDocument()
  })

  it('triggers cancelRoom when Cancel button is clicked', () => {
    const cancelRoom = vi.fn()
    renderComponent({ cancelRoom })

    const cancelBtn = screen.getByRole('button', { name: /cancel game & return to lobby/i })
    fireEvent.click(cancelBtn)

    expect(cancelRoom).toHaveBeenCalled()
  })

  it('disables cancel button when isCancelling is true', () => {
    renderComponent({ isCancelling: true })

    const cancelBtn = screen.getByRole('button', { name: /cancelling room/i })
    expect(cancelBtn).toBeDisabled()
  })
})
