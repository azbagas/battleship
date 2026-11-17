import { RoomStatePayload } from '../types/protocol'

export interface RoomParticipants {
  role: 'host' | 'guest'
  hostName: string
  guestName: string
  playerName: string
  opponentName: string
}

/** Derives display names for both commanders from the active room, with stable fallbacks. */
export function getRoomParticipants(room: RoomStatePayload | null | undefined): RoomParticipants {
  const hostName = room?.host?.name || 'Host Commander'
  const guestName = room?.guest?.name || room?.opponentName || 'Guest Commander'
  const role = room?.role || 'host'
  return {
    role,
    hostName,
    guestName,
    playerName: role === 'host' ? hostName : guestName,
    opponentName: role === 'host' ? guestName : hostName,
  }
}
