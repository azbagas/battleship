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
} as const

export type ErrorCodeType = (typeof ErrorCode)[keyof typeof ErrorCode] | string

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error'

export interface PlayerSession {
  playerId: string
  sessionToken: string
  displayName: string
  restoredState: unknown
}
