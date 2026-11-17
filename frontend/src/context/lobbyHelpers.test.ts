import { describe, it, expect } from 'vitest'
import { shotLogToEvents, shotsToMap } from './roomStateMappers'
import { lobbyErrorMessage } from './lobbyErrorMessages'
import { ErrorCode, ShotRecordPayload } from '../types/protocol'

describe('shotsToMap', () => {
  it('indexes shots by "x,y"', () => {
    const map = shotsToMap([
      { cell: { x: 1, y: 2 }, result: 'hit' },
      { cell: { x: 3, y: 4 }, result: 'miss' },
    ])
    expect(map.get('1,2')).toBe('hit')
    expect(map.get('3,4')).toBe('miss')
    expect(map.size).toBe(2)
  })
})

describe('shotLogToEvents', () => {
  const rec = (i: number): ShotRecordPayload => ({
    shooterId: `p${i}`,
    cell: { x: i % 10, y: 0 },
    result: 'miss',
    auto: i === 0,
  })

  it('returns newest first and maps shooterId to by', () => {
    const events = shotLogToEvents([rec(0), rec(1), rec(2)])
    expect(events.map((e) => e.by)).toEqual(['p2', 'p1', 'p0'])
    expect(events[2].auto).toBe(true)
  })

  it('keeps only the latest 20 entries', () => {
    const log = Array.from({ length: 25 }, (_, i) => rec(i))
    const events = shotLogToEvents(log)
    expect(events).toHaveLength(20)
    expect(events[0].by).toBe('p24')
    expect(events[19].by).toBe('p5')
  })
})

describe('lobbyErrorMessage', () => {
  it('maps known codes to friendly text', () => {
    expect(lobbyErrorMessage({ code: ErrorCode.NotYourTurn, message: 'raw' })).toBe(
      'Stand by, Commander. It is not your turn to fire.',
    )
    expect(lobbyErrorMessage({ code: ErrorCode.CannotJoinOwnRoom, message: 'raw' })).toBe(
      'You cannot join your own room.',
    )
    expect(lobbyErrorMessage({ code: ErrorCode.OpponentNotPresent, message: 'raw' })).toBe(
      'Opponent has already left the room. Rematch is unavailable.',
    )
  })

  it('falls back to the server message, then a generic one', () => {
    expect(lobbyErrorMessage({ code: 'SOMETHING_NEW', message: 'Server says no' })).toBe('Server says no')
    expect(lobbyErrorMessage({ code: 'SOMETHING_NEW', message: '' })).toBe('An unexpected error occurred.')
  })
})
