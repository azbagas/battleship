// Domain-action message types matching backend protocol
export type MessageType =
  | 'hello'
  | 'hello.ok'
  | 'error'
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

export const ErrorCode = {
  InvalidHandshake: 'INVALID_HANDSHAKE',
  InvalidName: 'INVALID_NAME',
  SessionAlreadyActive: 'SESSION_ALREADY_ACTIVE',
  BadRequest: 'BAD_REQUEST',
} as const

export type ErrorCodeType = (typeof ErrorCode)[keyof typeof ErrorCode] | string

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error'

export interface PlayerSession {
  playerId: string
  sessionToken: string
  displayName: string
  restoredState: unknown
}

