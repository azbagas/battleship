import React, { useState, useEffect, useCallback, useMemo } from 'react'
import {
  ErrorCode,
  ErrorPayload,
  HelloOKPayload,
  LobbySnapshotPayload,
  LobbyUpdatePayload,
  RoomReadyStatePayload,
  RoomStatePayload,
  WaitingRoomView,
} from '../types/protocol'
import { wsClient, WebSocketClient } from '../services/wsClient'
import { useSession } from './SessionContext'
import { LobbyContext, ReadyState } from './LobbyContext'

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

  // Ready-up states
  const [readyState, setReadyState] = useState<ReadyState | null>(null)
  const [isTogglingReady, setIsTogglingReady] = useState<boolean>(false)
  const [isLeavingRoom, setIsLeavingRoom] = useState<boolean>(false)

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
      setReadyState(null)
      setIsTogglingReady(false)
      setIsLeavingRoom(false)
    }
  }, [session.player])

  // Wire WebSocket message listeners
  useEffect(() => {
    const unsubHelloOK = client.onHelloOK((payload: HelloOKPayload) => {
      if (payload.restoredState && typeof payload.restoredState === 'object') {
        const state = payload.restoredState as Partial<RoomStatePayload>
        if (state.roomId && state.phase) {
          setActiveRoom(payload.restoredState as RoomStatePayload)
          if (typeof state.hostReady === 'boolean' && typeof state.guestReady === 'boolean') {
            setReadyState({ hostReady: state.hostReady, guestReady: state.guestReady })
          }
        }
      }
    })

    const unsubSnapshot = client.onMessage<LobbySnapshotPayload>('lobby.snapshot', (payload) => {
      if (!payload) return
      const rawRooms = Array.isArray(payload.rooms) ? payload.rooms : []
      setRooms(sortRoomsOldestFirst(rawRooms))
      setTotalRooms(payload.totalRooms ?? rawRooms.length)
      if (typeof payload.maxRooms === 'number') {
        setMaxRooms(payload.maxRooms)
      }

      // If in lobby snapshot, player is browsing the public lobby
      setActiveRoom(null)
      setReadyState(null)
      setIsCancelling(false)
      setIsLeavingRoom(false)
      setIsTogglingReady(false)
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
      if (typeof payload.hostReady === 'boolean' && typeof payload.guestReady === 'boolean') {
        setReadyState({ hostReady: payload.hostReady, guestReady: payload.guestReady })
      }
      setIsCreating(false)
      setJoiningRoomId(null)
      setIsLeavingRoom(false)
      setLobbyError(null)
    })

    const unsubReadyState = client.onMessage<RoomReadyStatePayload>('room.readyState', (payload) => {
      if (!payload || !payload.roomId) return
      setReadyState({ hostReady: payload.hostReady, guestReady: payload.guestReady })
      setIsTogglingReady(false)
    })

    const unsubError = client.onError((err: ErrorPayload) => {
      setIsCreating(false)
      setJoiningRoomId(null)
      setIsCancelling(false)
      setIsTogglingReady(false)
      setIsLeavingRoom(false)

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
        case ErrorCode.NotInReadyUp:
          setLobbyError('Room is not currently in ready-up phase.')
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
      unsubReadyState()
      unsubError()
    }
  }, [client])

  const isRoomCapReached = totalRooms >= maxRooms

  const createRoom = useCallback(() => {
    if (isCreating || isRoomCapReached || activeRoom) return
    setIsCreating(true)
    setLobbyError(null)
    const sent = client.send('lobby.create', {})
    if (!sent) {
      setIsCreating(false)
    }
  }, [client, isCreating, isRoomCapReached, activeRoom])

  const joinRoom = useCallback(
    (roomId: string) => {
      if (joiningRoomId || activeRoom || !roomId) return
      setJoiningRoomId(roomId)
      setLobbyError(null)
      const sent = client.send('lobby.join', { roomId })
      if (!sent) {
        setJoiningRoomId(null)
      }
    },
    [client, joiningRoomId, activeRoom],
  )

  const cancelRoom = useCallback(() => {
    if (isCancelling) return
    setIsCancelling(true)
    const sent = client.send('lobby.cancel', {})
    if (!sent) {
      setIsCancelling(false)
    }
  }, [client, isCancelling])

  const toggleReady = useCallback(() => {
    if (!activeRoom || activeRoom.phase !== 'ready-up' || isTogglingReady) return

    const isSelfReady =
      activeRoom.role === 'host'
        ? !!readyState?.hostReady
        : !!readyState?.guestReady

    setIsTogglingReady(true)
    const sent = isSelfReady
      ? client.send('room.unready', {})
      : client.send('room.ready', {})
    if (!sent) {
      setIsTogglingReady(false)
    }
  }, [client, activeRoom, readyState, isTogglingReady])

  const leaveRoom = useCallback(() => {
    if (isLeavingRoom) return
    setIsLeavingRoom(true)
    const sent = client.send('room.leave', {})
    if (!sent) {
      setIsLeavingRoom(false)
    }
  }, [client, isLeavingRoom])

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
      readyState,
      isTogglingReady,
      isLeavingRoom,
      toggleReady,
      leaveRoom,
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
      readyState,
      isTogglingReady,
      isLeavingRoom,
      toggleReady,
      leaveRoom,
    ],
  )

  return <LobbyContext.Provider value={value}>{children}</LobbyContext.Provider>
}

