// Domain-action message types matching backend protocol
export type MessageType =
  | 'hello'
  | 'hello.ok'
  | 'error'
  | 'lobby.create'
  | 'lobby.cancel'
  | 'lobby.join'
  | 'lobby.snapshot'
  | 'lobby.update'
  | 'room.state'
  | 'room.ready'
  | 'room.unready'
  | 'room.leave'
  | 'game.leave'
  | 'room.readyState'
  | 'placement.submit'
  | 'placement.ready'
  | 'placement.opponentReady'
  | 'placement.confirmed'
  | 'battle.fire'
  | 'battle.start'
  | 'battle.shotResult'
  | 'battle.turn'
  | 'game.over'
  | 'opponent.disconnected'
  | 'opponent.reconnected'
  | 'rematch.accept'
  | 'rematch.decline'
  | 'rematch.offered'
  | 'rematch.cancelled'
  | string

export interface InboundMessage<T = unknown> {
  type: MessageType
  payload: T
}

export interface OutboundMessage<T = unknown> {
  type: MessageType
  payload: T
}

export interface HelloPayload {
  name: string
  sessionToken?: string
}

export interface HelloOKPayload {
  playerId: string
  sessionToken: string
  displayName: string
  restoredState: unknown
}

export interface ErrorPayload {
  code: string
  message: string
}

export interface WaitingRoomView {
  id: string
  hostName: string
  createdAt: string
}

export interface LobbySnapshotPayload {
  rooms: WaitingRoomView[]
  totalRooms: number
  maxRooms: number
}

export interface LobbyUpdatePayload {
  action: 'upsert' | 'remove'
  room?: WaitingRoomView
  roomId?: string
  totalRooms: number
  maxRooms: number
}

export interface LobbyJoinPayload {
  roomId: string
}

export interface RoomPlayerView {
  id: string
  name: string
}

export type RoomPhase = 'waiting' | 'ready-up' | 'placement' | 'battle' | 'finished'

export type ShipType = 'carrier' | 'battleship' | 'cruiser' | 'submarine' | 'destroyer'
export type Orientation = 'horizontal' | 'vertical'

export interface Coord {
  x: number // 0..9 (A-J)
  y: number // 0..9 (1-10)
}

export interface PlacedShip {
  type: ShipType
  origin: Coord
  orientation: Orientation
}

export interface PlacementSubmitPayload {
  ships: PlacedShip[]
}

export interface PlacementConfirmedPayload {
  confirmed: boolean
  ships?: PlacedShip[]
  auto?: boolean
}

export interface PlacementOpponentReadyPayload {
  roomId: string
}

export type ShotResult = 'miss' | 'hit' | 'sunk'

export interface SunkShipInfo {
  type: ShipType
  cells: Coord[]
}

export interface ShotView {
  cell: Coord
  result: ShotResult
}

export interface PlayerBattleView {
  ownBoard: {
    ships: PlacedShip[]
    shots: ShotView[]
    sunkShips: SunkShipInfo[]
    remainingShips: number
    allSunk: boolean
  }
  targetBoard: {
    shots: ShotView[]
    sunkShips: SunkShipInfo[]
    remainingShips: number
    allSunk: boolean
  }
}

export interface BattleShotResultPayload {
  by: string
  cell: Coord
  result: ShotResult
  sunkShip?: SunkShipInfo
  auto?: boolean
}

export interface BattleTurnPayload {
  turnPlayerId: string
  deadline: string
}

export interface BattleStartPayload {
  firstPlayerId: string
}

export interface GameOverPayload {
  winnerPlayerId: string
  reason: 'all_sunk' | 'forfeit' | 'disconnect_timeout' | 'abandoned' | string
  totalShots: number
}

export interface OpponentDisconnectedPayload {
  graceDeadline: string
}

export type OpponentReconnectedPayload = Record<string, never>

export interface RematchOfferedPayload {
  by: string
}

export interface RematchCancelledPayload {
  reason: 'declined' | 'left' | 'disconnected' | string
}

export interface RoomStatePayload {
  roomId: string
  phase: RoomPhase
  role: 'host' | 'guest'
  opponentName?: string
  host: RoomPlayerView
  guest?: RoomPlayerView
  hostReady?: boolean
  guestReady?: boolean
  createdAt: string
  placementDeadline?: string
  fleetConfirmed?: boolean
  opponentConfirmed?: boolean
  ownFleet?: PlacedShip[]
  turnPlayerId?: string
  turnDeadline?: string
  battleView?: PlayerBattleView
  winnerPlayerId?: string
  endReason?: string
  opponentDisconnected?: boolean
  opponentGraceDeadline?: string
}

export interface RoomReadyStatePayload {
  roomId: string
  hostReady: boolean
  guestReady: boolean
}

export const ErrorCode = {
  InvalidHandshake: 'INVALID_HANDSHAKE',
  InvalidName: 'INVALID_NAME',
  SessionAlreadyActive: 'SESSION_ALREADY_ACTIVE',
  BadRequest: 'BAD_REQUEST',
  RoomCapReached: 'ROOM_CAP_REACHED',
  AlreadyInRoom: 'ALREADY_IN_ROOM',
  RoomNotFound: 'ROOM_NOT_FOUND',
  RoomNotAvailable: 'ROOM_NOT_AVAILABLE',
  NotInWaitingRoom: 'NOT_IN_WAITING_ROOM',
  CannotJoinOwnRoom: 'CANNOT_JOIN_OWN_ROOM',
  NotInReadyUp: 'NOT_IN_READY_UP',
  NotInRoom: 'NOT_IN_ROOM',
  NotInPlacement: 'NOT_IN_PLACEMENT',
  FleetAlreadyLocked: 'FLEET_ALREADY_LOCKED',
  InvalidPlacement: 'INVALID_PLACEMENT',
  InvalidState: 'INVALID_STATE',
  NotInBattle: 'NOT_IN_BATTLE',
  NotYourTurn: 'NOT_YOUR_TURN',
  CellAlreadyTargeted: 'CELL_ALREADY_TARGETED',
  InvalidCoordinate: 'INVALID_COORDINATE',
  GameOver: 'GAME_OVER',
  RematchNotAllowed: 'REMATCH_NOT_ALLOWED',
  NotInFinished: 'NOT_IN_FINISHED',
} as const

export type ErrorCodeType = (typeof ErrorCode)[keyof typeof ErrorCode] | string

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error'

export interface PlayerSession {
  playerId: string
  sessionToken: string
  displayName: string
  restoredState: unknown
}
