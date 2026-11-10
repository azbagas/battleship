import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { PlacementScreen } from './PlacementScreen'
import { LobbyContext, LobbyContextValue } from '../context/LobbyContext'
import { SessionContext, SessionContextValue } from '../context/SessionContext'
import { WebSocketClient } from '../services/wsClient'
import { RoomStatePayload } from '../types/protocol'

describe('PlacementScreen', () => {
  const baseSession: SessionContextValue = {
    connectionStatus: 'connected',
    player: {
      playerId: 'p_host',
      sessionToken: 'tok_host',
      displayName: 'Admiral Alpha',
      restoredState: null,
    },
    savedDisplayName: 'Admiral Alpha',
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
    roomId: 'r_alpha_sector_123456',
    phase: 'placement',
    role: 'host',
    host: { id: 'p_host', name: 'Admiral Alpha' },
    guest: { id: 'p_guest', name: 'Captain Bravo' },
    opponentName: 'Captain Bravo',
    placementDeadline: new Date(Date.now() + 60000).toISOString(),
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
    readyState: { hostReady: true, guestReady: true },
    isTogglingReady: false,
    isLeavingRoom: false,
    toggleReady: vi.fn(),
    leaveRoom: vi.fn(),
    isFleetConfirmed: false,
    isOpponentConfirmed: false,
    ownFleet: [],
    placementDeadline: baseRoom.placementDeadline || null,
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
    ...overrides,
  })

  const renderComponent = (lobbyValue: LobbyContextValue, sessionValue = baseSession) => {
    return render(
      <SessionContext.Provider value={sessionValue}>
        <LobbyContext.Provider value={lobbyValue}>
          <PlacementScreen />
        </LobbyContext.Provider>
      </SessionContext.Provider>,
    )
  }

  it('renders tactical 10×10 grid with headers and fleet dock', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    // Verify grid title and sector
    expect(screen.getByText('Battleship Combat')).toBeDefined()
    expect(screen.getByText(/Sector 123456/i)).toBeDefined()

    // Verify column headers A through J
    ;['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'].forEach((col) => {
      expect(screen.getAllByText(col).length).toBeGreaterThan(0)
    })

    // Verify all 5 ships in the fleet dock
    expect(screen.getByText('Carrier')).toBeDefined()
    expect(screen.getByText('Battleship')).toBeDefined()
    expect(screen.getByText('Cruiser')).toBeDefined()
    expect(screen.getByText('Submarine')).toBeDefined()
    expect(screen.getByText('Destroyer')).toBeDefined()

    // Confirm fleet button is initially disabled
    const confirmBtn = screen.getByRole('button', { name: /Place All Ships/i })
    expect(confirmBtn).toBeDefined()
    expect(confirmBtn.hasAttribute('disabled')).toBe(true)
  })

  it('allows selecting a ship and placing it on the grid via click', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    // Click Destroyer (2 cells) in dock
    const destroyerItem = screen.getByText('Destroyer')
    fireEvent.click(destroyerItem)

    // Cell B2 is column B (x=1), row 2 (y=1) -> button aria-label="Cell B2"
    const cellB2 = screen.getByRole('button', { name: 'Cell B2' })
    fireEvent.click(cellB2)

    // Cell B2 and C2 should now show 'D'
    expect(screen.getByRole('button', { name: /Cell B2, destroyer/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /Cell C2, destroyer/i })).toBeDefined()

    // Ship count should update to 1/5
    expect(screen.getByText('1/5 Ships Deployed')).toBeDefined()
  })

  it('supports rotating orientation via button and R keyboard shortcut', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    const rotateBtn = screen.getByRole('button', { name: /Rotate/i })
    expect(rotateBtn.textContent).toContain('Rotate (H)')

    // Click rotate button -> toggles to Vertical (V)
    fireEvent.click(rotateBtn)
    expect(rotateBtn.textContent).toContain('Rotate (V)')

    // Press 'r' key -> toggles back to Horizontal (H)
    fireEvent.keyDown(window, { key: 'r' })
    expect(rotateBtn.textContent).toContain('Rotate (H)')
  })

  it('places all 5 ships on Randomize and enables Confirm Fleet', () => {
    const submitFleetMock = vi.fn()
    const lobby = createMockLobby({ submitFleet: submitFleetMock })
    renderComponent(lobby)

    // Click Randomize
    const randomizeBtn = screen.getByRole('button', { name: /Randomize/i })
    fireEvent.click(randomizeBtn)

    // All 5 ships placed
    expect(screen.getByText('5/5 Ships Deployed')).toBeDefined()

    // Confirm fleet button should now be enabled
    const confirmBtn = screen.getByRole('button', { name: /Confirm Fleet/i })
    expect(confirmBtn.hasAttribute('disabled')).toBe(false)

    // Click Confirm Fleet
    fireEvent.click(confirmBtn)
    expect(submitFleetMock).toHaveBeenCalledTimes(1)
    expect(submitFleetMock.mock.calls[0][0]).toHaveLength(5)
  })

  it('displays opponent confirmed status when opponent confirms', () => {
    const lobby = createMockLobby({ isOpponentConfirmed: true })
    renderComponent(lobby)

    expect(screen.getByText('Fleet Confirmed')).toBeDefined()
  })

  it('locks controls when self fleet is confirmed', () => {
    const lobby = createMockLobby({ isFleetConfirmed: true })
    renderComponent(lobby)

    // Confirm button shows confirmed state and is disabled
    const confirmedBadge = screen.getByRole('button', { name: /Fleet Confirmed/i })
    expect(confirmedBadge.hasAttribute('disabled')).toBe(true)

    // Rotate and randomize buttons are disabled
    const rotateBtn = screen.getByRole('button', { name: /Rotate/i })
    expect(rotateBtn.hasAttribute('disabled')).toBe(true)

    const randomizeBtn = screen.getByRole('button', { name: /Randomize/i })
    expect(randomizeBtn.hasAttribute('disabled')).toBe(true)
  })
})
