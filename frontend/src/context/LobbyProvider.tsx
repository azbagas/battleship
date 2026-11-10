import React, { useState, useEffect, useCallback, useMemo } from 'react'
import {
  ErrorCode,
  ErrorPayload,
  HelloOKPayload,
  LobbySnapshotPayload,
  LobbyUpdatePayload,
  RoomStatePayload,
  WaitingRoomView,
} from '../types/protocol'
import { wsClient, WebSocketClient } from '../services/wsClient'
import { useSession } from './SessionContext'
import { LobbyContext } from './LobbyContext'

export interface LobbyProviderProps {
  children: React.ReactNode
  client?: WebSocketClient
}

function sortRoomsOldestFirst(rooms: WaitingRoomView[]): WaitingRoomView[] {
  return [...rooms].sort(
    (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
  )
}

export const LobbyProvider: React.FC<LobbyProviderProps> = ({
  children,
  client: propClient,
}) => {
  const session = useSession()
  const client = propClient || session.client || wsClient

  const [rooms, setRooms] = useState<WaitingRoomView[]>([])
  const [totalRooms, setTotalRooms] = useState<number>(0)
  const [maxRooms, setMaxRooms] = useState<number>(10)
  const [activeRoom, setActiveRoom] = useState<RoomStatePayload | null>(null)
  const [isCreating, setIsCreating] = useState<boolean>(false)
  const [joiningRoomId, setJoiningRoomId] = useState<string | null>(null)
  const [isCancelling, setIsCancelling] = useState<boolean>(false)
  const [lobbyError, setLobbyError] = useState<string | null>(null)

  // Reset lobby state if session logs out or resets
  useEffect(() => {
    if (!session.player) {
      setRooms([])
      setTotalRooms(0)
      setActiveRoom(null)
      setIsCreating(false)
      setJoiningRoomId(null)
      setIsCancelling(false)
      setLobbyError(null)
    }
  }, [session.player])

  // Wire WebSocket message listeners
  useEffect(() => {
    const unsubHelloOK = client.onHelloOK((payload: HelloOKPayload) => {
      if (payload.restoredState && typeof payload.restoredState === 'object') {
        const state = payload.restoredState as Partial<RoomStatePayload>
        if (state.roomId && state.phase) {
          setActiveRoom(payload.restoredState as RoomStatePayload)
        }
      }
    })

    const unsubSnapshot = client.onMessage<LobbySnapshotPayload>('lobby.snapshot', (payload) => {
      if (!payload || !Array.isArray(payload.rooms)) return
      setRooms(sortRoomsOldestFirst(payload.rooms))
      setTotalRooms(payload.totalRooms ?? payload.rooms.length)
      if (typeof payload.maxRooms === 'number') {
        setMaxRooms(payload.maxRooms)
      }

      // If host was waiting and snapshot arrives (e.g. after cancel), reset activeRoom
      setActiveRoom((prev) => (prev?.phase === 'waiting' ? null : prev))
      setIsCancelling(false)
    })

    const unsubUpdate = client.onMessage<LobbyUpdatePayload>('lobby.update', (payload) => {
      if (!payload) return

      if (payload.action === 'upsert' && payload.room) {
        setRooms((prev) => {
          const idx = prev.findIndex((r) => r.id === payload.room!.id)
          const next = idx >= 0 ? [...prev] : [...prev, payload.room!]
          if (idx >= 0) {
            next[idx] = payload.room!
          }
          return sortRoomsOldestFirst(next)
        })
      } else if (payload.action === 'remove' && payload.roomId) {
        setRooms((prev) => prev.filter((r) => r.id !== payload.roomId))
      }

      if (typeof payload.totalRooms === 'number') {
        setTotalRooms(payload.totalRooms)
      }
      if (typeof payload.maxRooms === 'number') {
        setMaxRooms(payload.maxRooms)
      }
    })

    const unsubRoomState = client.onMessage<RoomStatePayload>('room.state', (payload) => {
      if (!payload || !payload.roomId) return
      setActiveRoom(payload)
      setIsCreating(false)
      setJoiningRoomId(null)
      setLobbyError(null)
    })

    const unsubError = client.onError((err: ErrorPayload) => {
      setIsCreating(false)
      setJoiningRoomId(null)
      setIsCancelling(false)

      switch (err.code) {
        case ErrorCode.RoomNotAvailable:
          setLobbyError('Game is no longer available. Another commander joined or the room was cancelled.')
          break
        case ErrorCode.RoomCapReached:
          setLobbyError('Maximum room capacity reached (10 rooms). Please wait for an open slot or join a waiting game.')
          break
        case ErrorCode.RoomNotFound:
          setLobbyError('Room not found or no longer available.')
          break
        case ErrorCode.CannotJoinOwnRoom:
          setLobbyError('You cannot join your own room.')
          break
        case ErrorCode.AlreadyInRoom:
          setLobbyError('You already have an active room or match.')
          break
        case ErrorCode.NotInWaitingRoom:
          setLobbyError('You are not currently hosting a waiting room.')
          break
        default:
          // Other errors might be session-level, keep lobbyError null unless relevant
          break
      }
    })

    return () => {
      unsubHelloOK()
      unsubSnapshot()
      unsubUpdate()
      unsubRoomState()
      unsubError()
    }
  }, [client])

  const isRoomCapReached = totalRooms >= maxRooms

  const createRoom = useCallback(() => {
    if (isCreating || isRoomCapReached || activeRoom) return
    setIsCreating(true)
    setLobbyError(null)
    client.send('lobby.create', {})
  }, [client, isCreating, isRoomCapReached, activeRoom])

  const joinRoom = useCallback(
    (roomId: string) => {
      if (joiningRoomId || activeRoom || !roomId) return
      setJoiningRoomId(roomId)
      setLobbyError(null)
      client.send('lobby.join', { roomId })
    },
    [client, joiningRoomId, activeRoom],
  )

  const cancelRoom = useCallback(() => {
    if (isCancelling) return
    setIsCancelling(true)
    client.send('lobby.cancel', {})
  }, [client, isCancelling])

  const clearLobbyError = useCallback(() => {
    setLobbyError(null)
  }, [])

  const value = useMemo(
    () => ({
      rooms,
      totalRooms,
      maxRooms,
      activeRoom,
      isCreating,
      joiningRoomId,
      isCancelling,
      lobbyError,
      isRoomCapReached,
      createRoom,
      joinRoom,
      cancelRoom,
      clearLobbyError,
    }),
    [
      rooms,
      totalRooms,
      maxRooms,
      activeRoom,
      isCreating,
      joiningRoomId,
      isCancelling,
      lobbyError,
      isRoomCapReached,
      createRoom,
      joinRoom,
      cancelRoom,
      clearLobbyError,
    ],
  )

  return <LobbyContext.Provider value={value}>{children}</LobbyContext.Provider>
}

