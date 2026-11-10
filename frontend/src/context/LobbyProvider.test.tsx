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
})

