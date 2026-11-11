import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import React from 'react'
import { LobbyProvider } from './LobbyProvider'
import { useLobby } from './LobbyContext'
import { SessionContext, SessionContextValue } from './SessionContext'
import { WebSocketClient } from '../services/wsClient'
import {
  ErrorCode,
  LobbySnapshotPayload,
  LobbyUpdatePayload,
  RoomStatePayload,
  WaitingRoomView,
} from '../types/protocol'

describe('LobbyProvider', () => {
  let mockClient: WebSocketClient
  let messageHandlers: Map<string, (payload: unknown) => void>
  let errorHandlers: ((err: unknown) => void)[]
  let helloOKHandlers: ((payload: unknown) => void)[]

  const mockSession: SessionContextValue = {
    connectionStatus: 'connected',
    player: {
      playerId: 'p1',
      sessionToken: 'tok1',
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

  beforeEach(() => {
    messageHandlers = new Map()
    errorHandlers = []
    helloOKHandlers = []

    mockClient = {
      send: vi.fn().mockReturnValue(true),
      onMessage: vi.fn((type: string, handler: (payload: unknown) => void) => {
        messageHandlers.set(type, handler)
        return () => messageHandlers.delete(type)
      }),
      onError: vi.fn((handler: (err: unknown) => void) => {
        errorHandlers.push(handler)
        return () => {
          errorHandlers = errorHandlers.filter((h) => h !== handler)
        }
      }),
      onHelloOK: vi.fn((handler: (payload: unknown) => void) => {
        helloOKHandlers.push(handler)
        return () => {
          helloOKHandlers = helloOKHandlers.filter((h) => h !== handler)
        }
      }),
    } as unknown as WebSocketClient

    mockSession.client = mockClient
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <SessionContext.Provider value={mockSession}>
      <LobbyProvider client={mockClient}>{children}</LobbyProvider>
    </SessionContext.Provider>
  )

  it('initializes with empty rooms and default limits', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })
    expect(result.current.rooms).toEqual([])
    expect(result.current.totalRooms).toBe(0)
    expect(result.current.maxRooms).toBe(10)
    expect(result.current.isRoomCapReached).toBe(false)
    expect(result.current.activeRoom).toBeNull()
  })

  it('processes lobby.snapshot and sorts rooms oldest first', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    const snapshotHandler = messageHandlers.get('lobby.snapshot')
    expect(snapshotHandler).toBeDefined()

    const roomNewer: WaitingRoomView = {
      id: 'r_2',
      hostName: 'Captain Kirk',
      createdAt: '2026-11-09T10:05:00Z',
    }
    const roomOlder: WaitingRoomView = {
      id: 'r_1',
      hostName: 'Admiral Nimitz',
      createdAt: '2026-11-09T10:00:00Z',
    }

    const snapshot: LobbySnapshotPayload = {
      rooms: [roomNewer, roomOlder],
      totalRooms: 5,
      maxRooms: 10,
    }

    act(() => {
      snapshotHandler!(snapshot)
    })

    expect(result.current.rooms).toEqual([roomOlder, roomNewer])
    expect(result.current.totalRooms).toBe(5)
    expect(result.current.maxRooms).toBe(10)
    expect(result.current.isRoomCapReached).toBe(false)
  })

  it('updates live with lobby.update upsert and remove', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    const updateHandler = messageHandlers.get('lobby.update')
    expect(updateHandler).toBeDefined()

    const newRoom: WaitingRoomView = {
      id: 'r_3',
      hostName: 'Spock',
      createdAt: '2026-11-09T10:10:00Z',
    }

    // Upsert
    act(() => {
      updateHandler!({
        action: 'upsert',
        room: newRoom,
        totalRooms: 1,
        maxRooms: 10,
      } as LobbyUpdatePayload)
    })

    expect(result.current.rooms).toEqual([newRoom])
    expect(result.current.totalRooms).toBe(1)

    // Remove
    act(() => {
      updateHandler!({
        action: 'remove',
        roomId: 'r_3',
        totalRooms: 0,
        maxRooms: 10,
      } as LobbyUpdatePayload)
    })

    expect(result.current.rooms).toEqual([])
    expect(result.current.totalRooms).toBe(0)
  })

  it('indicates when room cap is reached', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })
    const snapshotHandler = messageHandlers.get('lobby.snapshot')

    act(() => {
      snapshotHandler!({
        rooms: [],
        totalRooms: 10,
        maxRooms: 10,
      })
    })

    expect(result.current.isRoomCapReached).toBe(true)
  })

  it('sends lobby.create, lobby.join, and lobby.cancel messages', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    act(() => {
      result.current.createRoom()
    })
    expect(mockClient.send).toHaveBeenCalledWith('lobby.create', {})

    act(() => {
      result.current.joinRoom('r_target')
    })
    expect(mockClient.send).toHaveBeenCalledWith('lobby.join', { roomId: 'r_target' })

    act(() => {
      result.current.cancelRoom()
    })
    expect(mockClient.send).toHaveBeenCalledWith('lobby.cancel', {})
  })

  it('updates activeRoom when room.state is received', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })
    const roomStateHandler = messageHandlers.get('room.state')

    const roomState: RoomStatePayload = {
      roomId: 'r_100',
      phase: 'waiting',
      role: 'host',
      host: { id: 'p1', name: 'Admiral Nimitz' },
      createdAt: '2026-11-09T10:00:00Z',
    }

    act(() => {
      roomStateHandler!(roomState)
    })

    expect(result.current.activeRoom).toEqual(roomState)
  })

  it('formats human-readable error on ROOM_NOT_AVAILABLE', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    act(() => {
      for (const handler of errorHandlers) {
        handler({
          code: ErrorCode.RoomNotAvailable,
          message: 'game is no longer available',
        })
      }
    })

    expect(result.current.lobbyError).toBe(
      'Game is no longer available. Another commander joined or the room was cancelled.',
    )

    act(() => {
      result.current.clearLobbyError()
    })
    expect(result.current.lobbyError).toBeNull()
  })

  it('updates readyState on room.readyState message and handles toggleReady', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })
    const roomStateHandler = messageHandlers.get('room.state')
    const readyStateHandler = messageHandlers.get('room.readyState')

    // Transition into ready-up room
    act(() => {
      roomStateHandler!({
        roomId: 'r_ready1',
        phase: 'ready-up',
        role: 'host',
        host: { id: 'p1', name: 'Admiral Nimitz' },
        createdAt: '2026-11-09T10:00:00Z',
      })
    })

    expect(result.current.readyState).toBeNull()

    // Host clicks toggleReady -> should send room.ready
    act(() => {
      result.current.toggleReady()
    })
    expect(mockClient.send).toHaveBeenCalledWith('room.ready', {})

    // Server broadcasts room.readyState with hostReady: true
    act(() => {
      readyStateHandler!({
        roomId: 'r_ready1',
        hostReady: true,
        guestReady: false,
      })
    })

    expect(result.current.readyState).toEqual({ hostReady: true, guestReady: false })

    // Host clicks toggleReady again -> should send room.unready
    act(() => {
      result.current.toggleReady()
    })
    expect(mockClient.send).toHaveBeenCalledWith('room.unready', {})
  })

  it('sends room.leave when leaveRoom is invoked', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    act(() => {
      result.current.leaveRoom()
    })

    expect(mockClient.send).toHaveBeenCalledWith('room.leave', {})
  })

  it('resets activeRoom and readyState when receiving lobby.snapshot', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })
    const roomStateHandler = messageHandlers.get('room.state')
    const snapshotHandler = messageHandlers.get('lobby.snapshot')

    act(() => {
      roomStateHandler!({
        roomId: 'r_ready2',
        phase: 'ready-up',
        role: 'guest',
        host: { id: 'p1', name: 'Admiral Nimitz' },
        guest: { id: 'p2', name: 'Captain Picard' },
        createdAt: '2026-11-09T10:00:00Z',
      })
    })
    expect(result.current.activeRoom).not.toBeNull()

    act(() => {
      snapshotHandler!({
        rooms: [],
        totalRooms: 0,
        maxRooms: 10,
      })
    })

    expect(result.current.activeRoom).toBeNull()
    expect(result.current.readyState).toBeNull()
  })

  it('resets activeRoom and isCancelling when lobby.snapshot arrives with null rooms', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })
    const roomStateHandler = messageHandlers.get('room.state')
    const snapshotHandler = messageHandlers.get('lobby.snapshot')

    act(() => {
      roomStateHandler!({
        roomId: 'r_waiting1',
        phase: 'waiting',
        role: 'host',
        host: { id: 'p1', name: 'Admiral Nimitz' },
        createdAt: '2026-11-09T10:00:00Z',
      })
    })

    act(() => {
      result.current.cancelRoom()
    })
    expect(result.current.isCancelling).toBe(true)

    // Backend sends rooms: null when empty
    act(() => {
      snapshotHandler!({
        rooms: null as unknown as WaitingRoomView[],
        totalRooms: 0,
        maxRooms: 10,
      })
    })

    expect(result.current.activeRoom).toBeNull()
    expect(result.current.isCancelling).toBe(false)
    expect(result.current.rooms).toEqual([])
  })

  it('handles placement phase orchestration messages and submitFleet', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })
    const roomStateHandler = messageHandlers.get('room.state')
    const placementConfirmedHandler = messageHandlers.get('placement.confirmed')
    const opponentReadyHandler = messageHandlers.get('placement.opponentReady')

    // Room enters placement
    act(() => {
      roomStateHandler!({
        roomId: 'r_place1',
        phase: 'placement',
        role: 'host',
        host: { id: 'p1', name: 'Admiral Nimitz' },
        guest: { id: 'p2', name: 'Commander Spock' },
        opponentName: 'Commander Spock',
        placementDeadline: '2026-11-10T12:01:00Z',
        fleetConfirmed: false,
        opponentConfirmed: false,
        createdAt: '2026-11-10T12:00:00Z',
      })
    })

    expect(result.current.activeRoom?.phase).toBe('placement')
    expect(result.current.placementDeadline).toBe('2026-11-10T12:01:00Z')
    expect(result.current.isFleetConfirmed).toBe(false)
    expect(result.current.isOpponentConfirmed).toBe(false)

    // Player submits fleet
    const testShips = [
      { type: 'destroyer' as const, origin: { x: 0, y: 0 }, orientation: 'horizontal' as const },
    ]
    act(() => {
      result.current.submitFleet(testShips)
    })
    expect(mockClient.send).toHaveBeenCalledWith('placement.submit', { ships: testShips })

    // Server sends placement.confirmed
    act(() => {
      placementConfirmedHandler!({
        confirmed: true,
        ships: testShips,
      })
    })
    expect(result.current.isFleetConfirmed).toBe(true)
    expect(result.current.ownFleet).toEqual(testShips)

    // Server sends placement.opponentReady
    act(() => {
      opponentReadyHandler!({
        roomId: 'r_place1',
      })
    })
    expect(result.current.isOpponentConfirmed).toBe(true)
  })

  it('handles opponent.disconnected and opponent.reconnected events', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    const roomStateHandler = messageHandlers.get('room.state')
    const oppDisconnectHandler = messageHandlers.get('opponent.disconnected')
    const oppReconnectHandler = messageHandlers.get('opponent.reconnected')

    expect(oppDisconnectHandler).toBeDefined()
    expect(oppReconnectHandler).toBeDefined()

    // Enter battle room
    act(() => {
      roomStateHandler!({
        roomId: 'r_battle1',
        phase: 'battle',
        role: 'host',
        host: { id: 'p1', name: 'Admiral Nimitz' },
        guest: { id: 'p2', name: 'Commander Spock' },
        opponentName: 'Commander Spock',
        turnPlayerId: 'p1',
        turnDeadline: '2026-11-10T12:00:30Z',
        createdAt: '2026-11-10T12:00:00Z',
      })
    })

    expect(result.current.opponentDisconnectDeadline).toBeNull()

    // Opponent disconnects
    const graceDeadline = '2026-11-10T12:01:00Z'
    act(() => {
      oppDisconnectHandler!({
        roomId: 'r_battle1',
        playerId: 'p2',
        graceDeadline,
      })
    })

    expect(result.current.opponentDisconnectDeadline).toBe(graceDeadline)

    // Opponent reconnects
    act(() => {
      oppReconnectHandler!({
        roomId: 'r_battle1',
        playerId: 'p2',
      })
    })

    expect(result.current.opponentDisconnectDeadline).toBeNull()
  })

  it('forfeitGame sends game.leave message and resets leaving state on error', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    act(() => {
      result.current.forfeitGame()
    })

    expect(mockClient.send).toHaveBeenCalledWith('game.leave', {})
    expect(result.current.isLeavingRoom).toBe(true)

    // On error, isLeavingRoom should reset
    act(() => {
      errorHandlers.forEach((h) =>
        h({
          code: ErrorCode.InvalidState,
          message: 'Cannot forfeit',
        }),
      )
    })

    expect(result.current.isLeavingRoom).toBe(false)
  })

  it('sets serverRestartNotice when reconnecting and server returns restoredState: null after active game', () => {
    const { result } = renderHook(() => useLobby(), { wrapper })

    const roomStateHandler = messageHandlers.get('room.state')

    // Player was in an active room
    act(() => {
      roomStateHandler!({
        roomId: 'r_active1',
        phase: 'battle',
        role: 'host',
        host: { id: 'p1', name: 'Admiral Nimitz' },
        guest: { id: 'p2', name: 'Commander Spock' },
        opponentName: 'Commander Spock',
        turnPlayerId: 'p1',
        createdAt: '2026-11-10T12:00:00Z',
      })
    })

    expect(result.current.activeRoom).not.toBeNull()

    // Server restarted: hello.ok arrives with restoredState: null
    act(() => {
      helloOKHandlers.forEach((h) =>
        h({
          playerId: 'p1',
          displayName: 'Admiral Nimitz',
          sessionToken: 'tok1',
          restoredState: null,
        }),
      )
    })

    expect(result.current.activeRoom).toBeNull()
    expect(result.current.serverRestartNotice).toContain(
      'Your in-progress game was lost because the server restarted.',
    )

    // User dismisses notification
    act(() => {
      result.current.clearServerRestartNotice()
    })

    expect(result.current.serverRestartNotice).toBeNull()
  })
})

