import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { BattleScreen } from './BattleScreen'
import { LobbyContext, LobbyContextValue } from '../context/LobbyContext'
import { SessionContext, SessionContextValue } from '../context/SessionContext'
import { WebSocketClient } from '../services/wsClient'
import { PlacedShip, RoomStatePayload, ShotResult, SunkShipInfo } from '../types/protocol'

describe('BattleScreen', () => {
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
    phase: 'battle',
    role: 'host',
    host: { id: 'p_host', name: 'Admiral Alpha' },
    guest: { id: 'p_guest', name: 'Captain Bravo' },
    opponentName: 'Captain Bravo',
    turnPlayerId: 'p_host',
    turnDeadline: new Date(Date.now() + 30000).toISOString(),
    createdAt: new Date().toISOString(),
  }

  const sampleFleet: PlacedShip[] = [
    { type: 'carrier', origin: { x: 0, y: 0 }, orientation: 'horizontal' },
    { type: 'battleship', origin: { x: 0, y: 1 }, orientation: 'horizontal' },
    { type: 'cruiser', origin: { x: 0, y: 2 }, orientation: 'horizontal' },
    { type: 'submarine', origin: { x: 0, y: 3 }, orientation: 'horizontal' },
    { type: 'destroyer', origin: { x: 0, y: 4 }, orientation: 'horizontal' },
  ]

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
    isFleetConfirmed: true,
    isOpponentConfirmed: true,
    ownFleet: sampleFleet,
    placementDeadline: null,
    submitFleet: vi.fn(),
    turnPlayerId: 'p_host',
    turnDeadline: baseRoom.turnDeadline || null,
    isMyTurn: true,
    targetShots: new Map<string, ShotResult>(),
    ownShots: new Map<string, ShotResult>(),
    opponentSunkShips: [] as SunkShipInfo[],
    ownSunkShips: [] as SunkShipInfo[],
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
          <BattleScreen />
        </LobbyContext.Provider>
      </SessionContext.Provider>,
    )
  }

  it('renders sector header, commanders roster, and connection status', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    expect(screen.getByText('Battleship Combat')).toBeDefined()
    expect(screen.getByText(/SEC-123456/i)).toBeDefined()
    expect(screen.getByText('Admiral Alpha')).toBeDefined()
    expect(screen.getByText('Captain Bravo')).toBeDefined()
    expect(screen.getByText('Connected')).toBeDefined()
  })

  it('renders "YOUR TURN" banner and countdown when player has turn priority', () => {
    const lobby = createMockLobby({ isMyTurn: true, turnPlayerId: 'p_host' })
    renderComponent(lobby)

    expect(screen.getByText(/YOUR TURN — SELECT TARGET CELL/i)).toBeDefined()
    expect(screen.getByText(/Command Priority/i)).toBeDefined()
    expect(screen.getByText(/Salvo Deadline/i)).toBeDefined()
  })

  it('renders "OPPONENT\'S TURN" banner when opponent has turn priority', () => {
    const lobby = createMockLobby({
      isMyTurn: false,
      turnPlayerId: 'p_guest',
    })
    renderComponent(lobby)

    expect(screen.getByText(/CAPTAIN BRAVO'S TURN — BRACE FOR IMPACT/i)).toBeDefined()
    expect(screen.getByText(/Hostile Maneuver/i)).toBeDefined()
  })

  it('renders all 5 ships in both friendly and hostile fleet rosters', () => {
    const lobby = createMockLobby()
    renderComponent(lobby)

    // Friendly roster
    expect(screen.getByText(/Friendly Fleet/i)).toBeDefined()
    // Hostile roster
    expect(screen.getByText(/Hostile Fleet/i)).toBeDefined()

    // 5 ships rendered for both friendly and hostile rosters
    expect(screen.getAllByText('Carrier').length).toBe(2)
    expect(screen.getAllByText('Battleship').length).toBe(2)
    expect(screen.getAllByText('Cruiser').length).toBe(2)
    expect(screen.getAllByText('Submarine').length).toBe(2)
    expect(screen.getAllByText('Destroyer').length).toBe(2)
  })

  it('allows clicking an untargeted cell to fire a shot when isMyTurn is true', () => {
    const fireShotMock = vi.fn()
    const lobby = createMockLobby({
      isMyTurn: true,
      fireShot: fireShotMock,
    })
    renderComponent(lobby)

    const cellB3 = screen.getByRole('button', { name: /Target cell B3, ready to fire/i })
    expect(cellB3).toBeDefined()
    expect(cellB3.hasAttribute('disabled')).toBe(false)

    fireEvent.click(cellB3)
    expect(fireShotMock).toHaveBeenCalledTimes(1)
    expect(fireShotMock).toHaveBeenCalledWith({ x: 1, y: 2 })
  })

  it('disables firing when it is not player turn', () => {
    const fireShotMock = vi.fn()
    const lobby = createMockLobby({
      isMyTurn: false,
      fireShot: fireShotMock,
    })
    renderComponent(lobby)

    const cellB3 = screen.getByRole('button', { name: /Target cell B3, stand by/i })
    expect(cellB3.hasAttribute('disabled')).toBe(true)

    fireEvent.click(cellB3)
    expect(fireShotMock).not.toHaveBeenCalled()
  })

  it('renders shot markers (miss, hit, sunk) on target radar correctly', () => {
    const targetShots = new Map<string, ShotResult>()
    targetShots.set('0,0', 'hit')
    targetShots.set('1,0', 'hit')
    targetShots.set('2,2', 'miss')
    targetShots.set('4,4', 'sunk')

    const opponentSunkShips: SunkShipInfo[] = [
      {
        type: 'destroyer',
        cells: [
          { x: 4, y: 4 },
          { x: 5, y: 4 },
        ],
      },
    ]

    const lobby = createMockLobby({
      targetShots,
      opponentSunkShips,
    })
    renderComponent(lobby)

    // Miss on C3
    const cellC3 = screen.getByRole('button', { name: /Target cell C3, MISS/i })
    expect(cellC3).toBeDefined()
    expect(cellC3.textContent).toContain('•')

    // Hit on A1
    const cellA1 = screen.getByRole('button', { name: /Target cell A1, HIT/i })
    expect(cellA1).toBeDefined()
    expect(cellA1.textContent).toContain('✕')

    // Sunk on E5
    const cellE5 = screen.getByRole('button', { name: /Target cell E5, SUNK destroyer/i })
    expect(cellE5).toBeDefined()
  })

  it('renders defensive grid with own placed ships and incoming shots', () => {
    const ownShots = new Map<string, ShotResult>()
    // Hit on Carrier at 0,0
    ownShots.set('0,0', 'hit')
    // Miss in open water at 9,9
    ownShots.set('9,9', 'miss')

    const lobby = createMockLobby({ ownShots })
    renderComponent(lobby)

    // Carrier hit at A1 on defensive grid
    expect(screen.getByLabelText(/Defensive cell A1, HIT on carrier/i)).toBeDefined()
    // Open water miss at J10
    expect(screen.getByLabelText(/Defensive cell J10, enemy miss/i)).toBeDefined()
    // Intact Destroyer at A5
    expect(screen.getByLabelText(/Defensive cell A5, destroyer/i)).toBeDefined()
  })

  it('renders combat event log with shots and auto-shot badges', () => {
    const recentEvents = [
      {
        by: 'p_host',
        cell: { x: 1, y: 3 }, // B4
        result: 'hit' as const,
      },
      {
        by: 'p_guest',
        cell: { x: 8, y: 8 }, // I9
        result: 'miss' as const,
        auto: true,
      },
      {
        by: 'p_host',
        cell: { x: 3, y: 3 }, // D4
        result: 'sunk' as const,
        sunkShip: { type: 'submarine' as const, cells: [{ x: 3, y: 3 }] },
      },
    ]

    const lobby = createMockLobby({ recentEvents })
    renderComponent(lobby)

    expect(screen.getByText('Tactical Combat Log')).toBeDefined()
    expect(screen.getByText('B4')).toBeDefined()
    expect(screen.getByText('DIRECT HIT')).toBeDefined()
    expect(screen.getByText('I9')).toBeDefined()
    expect(screen.getByText('MISS')).toBeDefined()
    expect(screen.getByText('AUTO')).toBeDefined()
    expect(screen.getByText('D4')).toBeDefined()
    expect(screen.getByText('SUNK SUBMARINE!')).toBeDefined()
  })

  it('renders Game Over modal on victory and triggers leaveRoom on Return to Lobby', () => {
    const leaveRoomMock = vi.fn()
    const lobby = createMockLobby({
      gameOverResult: {
        winnerPlayerId: 'p_host',
        reason: 'all_sunk',
        totalShots: 24,
      },
      leaveRoom: leaveRoomMock,
    })
    renderComponent(lobby)

    expect(screen.getByText('VICTORY ACHIEVED')).toBeDefined()
    expect(screen.getByText('Hostile fleet successfully eliminated.')).toBeDefined()
    expect(screen.getByText('All Ships Sunk')).toBeDefined()

    const returnBtn = screen.getByRole('button', { name: /Return to Lobby/i })
    expect(returnBtn).toBeDefined()

    fireEvent.click(returnBtn)
    expect(leaveRoomMock).toHaveBeenCalledTimes(1)
  })

  it('renders Game Over modal on defeat', () => {
    const lobby = createMockLobby({
      gameOverResult: {
        winnerPlayerId: 'p_guest',
        reason: 'all_sunk',
        totalShots: 28,
      },
    })
    renderComponent(lobby)

    expect(screen.getByText('DEFEAT — FLEET LOST')).toBeDefined()
    expect(screen.getByText('All friendly naval forces have been destroyed.')).toBeDefined()
  })

  it('allows commander to leave/forfeit via leave confirmation modal', () => {
    const forfeitGameMock = vi.fn()
    const lobby = createMockLobby({ forfeitGame: forfeitGameMock })
    renderComponent(lobby)

    const leaveLink = screen.getByRole('button', { name: /Leave Game/i })
    fireEvent.click(leaveLink)

    expect(screen.getByText(/Leave Game \/ Forfeit Match\?/i)).toBeDefined()
    expect(screen.getByText(/Leaving will forfeit the match immediately/i)).toBeDefined()

    const confirmBtn = screen.getByRole('button', { name: /Confirm Forfeit/i })
    fireEvent.click(confirmBtn)

    expect(forfeitGameMock).toHaveBeenCalledTimes(1)
  })

  it('renders opponent disconnect banner when opponentDisconnectDeadline is active', () => {
    const deadline = new Date(Date.now() + 45000).toISOString()
    const lobby = createMockLobby({ opponentDisconnectDeadline: deadline })
    renderComponent(lobby)

    expect(screen.getByText(/CONNECTION DROPPED/i)).toBeDefined()
    expect(screen.getByText(/Captain Bravo is offline/i)).toBeDefined()
  })
})

