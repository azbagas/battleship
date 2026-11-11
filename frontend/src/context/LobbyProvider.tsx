import React, { useState, useEffect, useCallback, useMemo } from 'react'
import {
  BattleShotResultPayload,
  BattleStartPayload,
  BattleTurnPayload,
  Coord,
  ErrorCode,
  ErrorPayload,
  GameOverPayload,
  HelloOKPayload,
  LobbySnapshotPayload,
  LobbyUpdatePayload,
  OpponentDisconnectedPayload,
  PlacedShip,
  PlacementConfirmedPayload,
  PlacementOpponentReadyPayload,
  RematchCancelledPayload,
  RematchOfferedPayload,
  RoomReadyStatePayload,
  RoomStatePayload,
  ShotResult,
  SunkShipInfo,
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

  // Placement phase states
  const [isFleetConfirmed, setIsFleetConfirmed] = useState<boolean>(false)
  const [isOpponentConfirmed, setIsOpponentConfirmed] = useState<boolean>(false)
  const [ownFleet, setOwnFleet] = useState<PlacedShip[]>([])
  const [placementDeadline, setPlacementDeadline] = useState<string | null>(null)

  // Battle phase states
  const [turnPlayerId, setTurnPlayerId] = useState<string | null>(null)
  const [turnDeadline, setTurnDeadline] = useState<string | null>(null)
  const [targetShots, setTargetShots] = useState<Map<string, ShotResult>>(new Map())
  const [ownShots, setOwnShots] = useState<Map<string, ShotResult>>(new Map())
  const [opponentSunkShips, setOpponentSunkShips] = useState<SunkShipInfo[]>([])
  const [ownSunkShips, setOwnSunkShips] = useState<SunkShipInfo[]>([])
  const [recentEvents, setRecentEvents] = useState<BattleShotResultPayload[]>([])
  const [gameOverResult, setGameOverResult] = useState<GameOverPayload | null>(null)

  // Disconnect & recovery states
  const [opponentDisconnectDeadline, setOpponentDisconnectDeadline] = useState<string | null>(null)
  const [serverRestartNotice, setServerRestartNotice] = useState<string | null>(null)

  // Rematch states
  const [rematchStatus, setRematchStatus] = useState<
    'idle' | 'offered_by_me' | 'offered_by_opponent' | 'cancelled'
  >('idle')
  const [rematchCancelReason, setRematchCancelReason] = useState<string | null>(null)

  const resetRematchState = useCallback(() => {
    setRematchStatus('idle')
    setRematchCancelReason(null)
  }, [])

  const isMyTurn = Boolean(
    session.player &&
      turnPlayerId &&
      session.player.playerId === turnPlayerId &&
      !gameOverResult,
  )

  const resetBattleState = useCallback(() => {
    setTurnPlayerId(null)
    setTurnDeadline(null)
    setTargetShots(new Map())
    setOwnShots(new Map())
    setOpponentSunkShips([])
    setOwnSunkShips([])
    setRecentEvents([])
    setGameOverResult(null)
  }, [])

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
      setIsFleetConfirmed(false)
      setIsOpponentConfirmed(false)
      setOwnFleet([])
      setPlacementDeadline(null)
      resetBattleState()
      resetRematchState()
    }
  }, [session.player, resetBattleState, resetRematchState])

  // Wire WebSocket message listeners
  useEffect(() => {
    const unsubHelloOK = client.onHelloOK((payload: HelloOKPayload) => {
      if (payload.restoredState && typeof payload.restoredState === 'object') {
        const state = payload.restoredState as Partial<RoomStatePayload>
        if (state.roomId && state.phase) {
          setActiveRoom(payload.restoredState as RoomStatePayload)
          if (state.opponentDisconnected && state.opponentGraceDeadline) {
            setOpponentDisconnectDeadline(state.opponentGraceDeadline)
          } else {
            setOpponentDisconnectDeadline(null)
          }
          if (typeof state.hostReady === 'boolean' && typeof state.guestReady === 'boolean') {
            setReadyState({ hostReady: state.hostReady, guestReady: state.guestReady })
          }
          if (state.placementDeadline) {
            setPlacementDeadline(state.placementDeadline)
          }
          if (typeof state.fleetConfirmed === 'boolean') {
            setIsFleetConfirmed(state.fleetConfirmed)
          }
          if (typeof state.opponentConfirmed === 'boolean') {
            setIsOpponentConfirmed(state.opponentConfirmed)
          }
          if (Array.isArray(state.ownFleet)) {
            setOwnFleet(state.ownFleet)
          }
          if (state.turnPlayerId) {
            setTurnPlayerId(state.turnPlayerId)
          }
          if (state.turnDeadline) {
            setTurnDeadline(state.turnDeadline)
          }
          if (state.winnerPlayerId) {
            setGameOverResult({
              winnerPlayerId: state.winnerPlayerId,
              reason: state.endReason || 'all_sunk',
              totalShots: 0,
            })
          }
          if (state.battleView) {
            if (Array.isArray(state.battleView.targetBoard?.shots)) {
              const tMap = new Map<string, ShotResult>()
              state.battleView.targetBoard.shots.forEach((s) => {
                tMap.set(`${s.cell.x},${s.cell.y}`, s.result)
              })
              setTargetShots(tMap)
            }
            if (Array.isArray(state.battleView.ownBoard?.shots)) {
              const oMap = new Map<string, ShotResult>()
              state.battleView.ownBoard.shots.forEach((s) => {
                oMap.set(`${s.cell.x},${s.cell.y}`, s.result)
              })
              setOwnShots(oMap)
            }
            if (Array.isArray(state.battleView.targetBoard?.sunkShips)) {
              setOpponentSunkShips(state.battleView.targetBoard.sunkShips)
            }
            if (Array.isArray(state.battleView.ownBoard?.sunkShips)) {
              setOwnSunkShips(state.battleView.ownBoard.sunkShips)
            }
          }
        }
      } else {
        // If the player previously had an active room and restoredState is null/empty,
        // the server restarted and cleared live matches.
        setActiveRoom((prev) => {
          if (prev && prev.roomId) {
            setServerRestartNotice('Your in-progress game was lost because the server restarted.')
          }
          return null
        })
        setReadyState(null)
        setIsFleetConfirmed(false)
        setIsOpponentConfirmed(false)
        setOwnFleet([])
        setPlacementDeadline(null)
        setOpponentDisconnectDeadline(null)
        resetBattleState()
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
      setIsFleetConfirmed(false)
      setIsOpponentConfirmed(false)
      setOwnFleet([])
      setPlacementDeadline(null)
      setOpponentDisconnectDeadline(null)
      resetBattleState()
      resetRematchState()
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
      if (payload.phase === 'placement') {
        setGameOverResult(null)
        resetBattleState()
        resetRematchState()
      }
      if (payload.opponentDisconnected && payload.opponentGraceDeadline) {
        setOpponentDisconnectDeadline(payload.opponentGraceDeadline)
      } else {
        setOpponentDisconnectDeadline(null)
      }
      if (typeof payload.hostReady === 'boolean' && typeof payload.guestReady === 'boolean') {
        setReadyState({ hostReady: payload.hostReady, guestReady: payload.guestReady })
      }
      if (payload.placementDeadline) {
        setPlacementDeadline(payload.placementDeadline)
      }
      if (typeof payload.fleetConfirmed === 'boolean') {
        setIsFleetConfirmed(payload.fleetConfirmed)
      }
      if (typeof payload.opponentConfirmed === 'boolean') {
        setIsOpponentConfirmed(payload.opponentConfirmed)
      }
      if (Array.isArray(payload.ownFleet)) {
        setOwnFleet(payload.ownFleet)
      }
      if (payload.turnPlayerId) {
        setTurnPlayerId(payload.turnPlayerId)
      }
      if (payload.turnDeadline) {
        setTurnDeadline(payload.turnDeadline)
      }
      if (payload.winnerPlayerId) {
        setGameOverResult({
          winnerPlayerId: payload.winnerPlayerId,
          reason: payload.endReason || 'all_sunk',
          totalShots: 0,
        })
      }
      if (payload.battleView) {
        if (Array.isArray(payload.battleView.targetBoard?.shots)) {
          const tMap = new Map<string, ShotResult>()
          payload.battleView.targetBoard.shots.forEach((s) => {
            tMap.set(`${s.cell.x},${s.cell.y}`, s.result)
          })
          setTargetShots(tMap)
        }
        if (Array.isArray(payload.battleView.ownBoard?.shots)) {
          const oMap = new Map<string, ShotResult>()
          payload.battleView.ownBoard.shots.forEach((s) => {
            oMap.set(`${s.cell.x},${s.cell.y}`, s.result)
          })
          setOwnShots(oMap)
        }
        if (Array.isArray(payload.battleView.targetBoard?.sunkShips)) {
          setOpponentSunkShips(payload.battleView.targetBoard.sunkShips)
        }
        if (Array.isArray(payload.battleView.ownBoard?.sunkShips)) {
          setOwnSunkShips(payload.battleView.ownBoard.sunkShips)
        }
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

    const unsubPlacementConfirmed = client.onMessage<PlacementConfirmedPayload>(
      'placement.confirmed',
      (payload) => {
        if (!payload) return
        setIsFleetConfirmed(true)
        if (Array.isArray(payload.ships)) {
          setOwnFleet(payload.ships)
        }
      },
    )

    const unsubOpponentReady = client.onMessage<PlacementOpponentReadyPayload>(
      'placement.opponentReady',
      () => {
        setIsOpponentConfirmed(true)
      },
    )

    const unsubBattleStart = client.onMessage<BattleStartPayload>(
      'battle.start',
      (payload) => {
        if (!payload) return
        setTurnPlayerId(payload.firstPlayerId)
      },
    )

    const unsubBattleTurn = client.onMessage<BattleTurnPayload>(
      'battle.turn',
      (payload) => {
        if (!payload) return
        setTurnPlayerId(payload.turnPlayerId)
        setTurnDeadline(payload.deadline)
      },
    )

    const unsubBattleShotResult = client.onMessage<BattleShotResultPayload>(
      'battle.shotResult',
      (payload) => {
        if (!payload) return

        const isMyShot = session.player?.playerId === payload.by
        const key = `${payload.cell.x},${payload.cell.y}`

        if (isMyShot) {
          setTargetShots((prev) => {
            const next = new Map(prev)
            next.set(key, payload.result)
            return next
          })
          if (payload.sunkShip) {
            setOpponentSunkShips((prev) => [...prev, payload.sunkShip!])
          }
        } else {
          setOwnShots((prev) => {
            const next = new Map(prev)
            next.set(key, payload.result)
            return next
          })
          if (payload.sunkShip) {
            setOwnSunkShips((prev) => [...prev, payload.sunkShip!])
          }
        }

        setRecentEvents((prev) => [payload, ...prev.slice(0, 19)])
      },
    )

    const unsubGameOver = client.onMessage<GameOverPayload>(
      'game.over',
      (payload) => {
        if (!payload) return
        setGameOverResult(payload)
        setOpponentDisconnectDeadline(null)
      },
    )

    const unsubOpponentDisconnected = client.onMessage<OpponentDisconnectedPayload>(
      'opponent.disconnected',
      (payload) => {
        if (payload?.graceDeadline) {
          setOpponentDisconnectDeadline(payload.graceDeadline)
        }
      },
    )

    const unsubOpponentReconnected = client.onMessage(
      'opponent.reconnected',
      () => {
        setOpponentDisconnectDeadline(null)
      },
    )

    const unsubRematchOffered = client.onMessage<RematchOfferedPayload>(
      'rematch.offered',
      (payload) => {
        if (!payload) return
        if (payload.by === session.player?.playerId) {
          setRematchStatus('offered_by_me')
        } else {
          setRematchStatus('offered_by_opponent')
        }
      },
    )

    const unsubRematchCancelled = client.onMessage<RematchCancelledPayload>(
      'rematch.cancelled',
      (payload) => {
        setRematchStatus('cancelled')
        setRematchCancelReason(payload?.reason || 'cancelled')
      },
    )

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
          setLobbyError('Room is no longer in ready-up phase.')
          break
        case ErrorCode.NotInPlacement:
          setLobbyError('Fleet placement has concluded.')
          break
        case ErrorCode.FleetAlreadyLocked:
          setLobbyError('Fleet is already confirmed and locked.')
          break
        case ErrorCode.InvalidPlacement:
          setLobbyError('Invalid ship positions. Check for overlaps or out-of-grid bounds.')
          break
        case ErrorCode.NotYourTurn:
          setLobbyError('Stand by, Commander. It is not your turn to fire.')
          break
        case ErrorCode.CellAlreadyTargeted:
          setLobbyError('Coordinate has already been targeted. Select an uncharted grid cell.')
          break
        case ErrorCode.GameOver:
          setLobbyError('The engagement has already concluded.')
          break
        case ErrorCode.RematchNotAllowed:
          setLobbyError('Rematch is only available following a normal completed game.')
          break
        default:
          setLobbyError(err.message || 'An unexpected error occurred.')
      }
    })

    return () => {
      unsubHelloOK()
      unsubSnapshot()
      unsubUpdate()
      unsubRoomState()
      unsubReadyState()
      unsubPlacementConfirmed()
      unsubOpponentReady()
      unsubBattleStart()
      unsubBattleTurn()
      unsubBattleShotResult()
      unsubGameOver()
      unsubOpponentDisconnected()
      unsubOpponentReconnected()
      unsubRematchOffered()
      unsubRematchCancelled()
      unsubError()
    }
  }, [client, session.player, resetBattleState, resetRematchState])

  const isRoomCapReached = totalRooms >= maxRooms

  const createRoom = useCallback(() => {
    setIsCreating(true)
    setLobbyError(null)
    client.send('lobby.create', {})
  }, [client])

  const joinRoom = useCallback(
    (roomId: string) => {
      setJoiningRoomId(roomId)
      setLobbyError(null)
      client.send('lobby.join', { roomId })
    },
    [client],
  )

  const cancelRoom = useCallback(() => {
    setIsCancelling(true)
    setLobbyError(null)
    client.send('lobby.cancel', {})
  }, [client])

  const clearLobbyError = useCallback(() => {
    setLobbyError(null)
  }, [])

  const toggleReady = useCallback(() => {
    if (!activeRoom || activeRoom.phase !== 'ready-up') return
    setIsTogglingReady(true)
    const amHost = activeRoom.role === 'host'
    const currentlyReady = amHost ? readyState?.hostReady : readyState?.guestReady

    if (currentlyReady) {
      client.send('room.unready', {})
    } else {
      client.send('room.ready', {})
    }
  }, [activeRoom, readyState, client])

  const clearServerRestartNotice = useCallback(() => {
    setServerRestartNotice(null)
  }, [])

  const forfeitGame = useCallback(() => {
    setIsLeavingRoom(true)
    setLobbyError(null)
    client.send('game.leave', {})
  }, [client])

  const leaveRoom = useCallback(() => {
    setIsLeavingRoom(true)
    setLobbyError(null)
    if (activeRoom && (activeRoom.phase === 'placement' || activeRoom.phase === 'battle')) {
      client.send('game.leave', {})
    } else {
      client.send('room.leave', {})
    }
  }, [activeRoom, client])

  const submitFleet = useCallback(
    (ships: PlacedShip[]) => {
      if (!activeRoom || activeRoom.phase !== 'placement') return
      client.send('placement.submit', { ships })
    },
    [activeRoom, client],
  )

  const fireShot = useCallback(
    (cell: Coord) => {
      if (!isMyTurn || gameOverResult) return
      const key = `${cell.x},${cell.y}`
      if (targetShots.has(key)) return
      client.send('battle.fire', { cell })
    },
    [isMyTurn, gameOverResult, targetShots, client],
  )

  const acceptRematch = useCallback(() => {
    client.send('rematch.accept', {})
  }, [client])

  const declineRematch = useCallback(() => {
    client.send('rematch.decline', {})
    setRematchStatus('cancelled')
    setRematchCancelReason('declined')
  }, [client])

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
      isFleetConfirmed,
      isOpponentConfirmed,
      ownFleet,
      placementDeadline,
      submitFleet,
      turnPlayerId,
      turnDeadline,
      isMyTurn,
      targetShots,
      ownShots,
      opponentSunkShips,
      ownSunkShips,
      recentEvents,
      gameOverResult,
      fireShot,
      opponentDisconnectDeadline,
      serverRestartNotice,
      clearServerRestartNotice,
      forfeitGame,
      rematchStatus,
      rematchCancelReason,
      acceptRematch,
      declineRematch,
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
      isFleetConfirmed,
      isOpponentConfirmed,
      ownFleet,
      placementDeadline,
      submitFleet,
      turnPlayerId,
      turnDeadline,
      isMyTurn,
      targetShots,
      ownShots,
      opponentSunkShips,
      ownSunkShips,
      recentEvents,
      gameOverResult,
      fireShot,
      opponentDisconnectDeadline,
      serverRestartNotice,
      clearServerRestartNotice,
      forfeitGame,
      rematchStatus,
      rematchCancelReason,
      acceptRematch,
      declineRematch,
    ],
  )

  return <LobbyContext.Provider value={value}>{children}</LobbyContext.Provider>
}
