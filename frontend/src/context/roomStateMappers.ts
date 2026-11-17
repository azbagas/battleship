import {
  BattleShotResultPayload,
  ShotRecordPayload,
  ShotResult,
  ShotView,
} from '../types/protocol'

/** Index shots by `"x,y"` for O(1) cell lookup on the boards. */
export function shotsToMap(shots: ShotView[]): Map<string, ShotResult> {
  const map = new Map<string, ShotResult>()
  shots.forEach((s) => {
    map.set(`${s.cell.x},${s.cell.y}`, s.result)
  })
  return map
}

/** Newest-first combat log built from the server's shot history, capped at `limit` entries. */
export function shotLogToEvents(log: ShotRecordPayload[], limit = 20): BattleShotResultPayload[] {
  return log
    .slice(-limit)
    .reverse()
    .map((rec) => ({
      by: rec.shooterId,
      cell: rec.cell,
      result: rec.result,
      sunkShip: rec.sunkShip,
      auto: rec.auto,
    }))
}
