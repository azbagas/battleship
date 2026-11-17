import { describe, it, expect } from 'vitest'
import { getRoomParticipants } from './roomParticipants'
import { RoomStatePayload } from '../types/protocol'

const room = (overrides: Partial<RoomStatePayload>) => overrides as RoomStatePayload

describe('getRoomParticipants', () => {
  it('falls back to default names and host role without a room', () => {
    expect(getRoomParticipants(null)).toEqual({
      role: 'host',
      hostName: 'Host Commander',
      guestName: 'Guest Commander',
      playerName: 'Host Commander',
      opponentName: 'Guest Commander',
    })
  })

  it('resolves player and opponent for the host', () => {
    const p = getRoomParticipants(
      room({ role: 'host', host: { name: 'Ann' } as never, guest: { name: 'Bob' } as never }),
    )
    expect(p.playerName).toBe('Ann')
    expect(p.opponentName).toBe('Bob')
  })

  it('resolves player and opponent for the guest, using opponentName as fallback', () => {
    const p = getRoomParticipants(
      room({ role: 'guest', host: { name: 'Ann' } as never, opponentName: 'Ann' }),
    )
    expect(p.role).toBe('guest')
    expect(p.playerName).toBe('Ann')
    expect(p.opponentName).toBe('Ann')
  })
})
