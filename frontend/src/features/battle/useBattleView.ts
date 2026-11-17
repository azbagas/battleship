import { useMemo } from 'react'
import {
  GameOverPayload,
  PlacedShip,
  RoomStatePayload,
  ShipType,
  SunkShipInfo,
} from '../../types/protocol'
import { SHIP_SIZES, getShipCells, sunkShipToPlacedShip } from '../../utils/placement'
import { getRoomParticipants } from '../../utils/roomParticipants'

export interface OccupiedCell {
  ship: PlacedShip
  index: number
}

function sunkCellMap(ships: SunkShipInfo[]): Map<string, ShipType> {
  const map = new Map<string, ShipType>()
  ships.forEach((sunk) => {
    sunk.cells.forEach((c) => map.set(`${c.x},${c.y}`, sunk.type))
  })
  return map
}

export interface BattleViewInput {
  activeRoom: RoomStatePayload | null
  ownFleet: PlacedShip[]
  opponentSunkShips: SunkShipInfo[]
  ownSunkShips: SunkShipInfo[]
  gameOverResult: GameOverPayload | null
  playerId: string | undefined
}

/** Derives names, lookups and match outcome for the battle screen from lobby state. */
export function useBattleView({
  activeRoom,
  ownFleet,
  opponentSunkShips,
  ownSunkShips,
  gameOverResult,
  playerId,
}: BattleViewInput) {
  const { role, playerName, opponentName } = getRoomParticipants(activeRoom)
  const sectorId = activeRoom?.roomId ? activeRoom.roomId.slice(-6).toUpperCase() : 'SECTOR-1'

  const ownOccupiedMap = useMemo(() => {
    const map = new Map<string, OccupiedCell>()
    ownFleet.forEach((s) => {
      getShipCells(s.origin, SHIP_SIZES[s.type], s.orientation).forEach((c, idx) => {
        map.set(`${c.x},${c.y}`, { ship: s, index: idx })
      })
    })
    return map
  }, [ownFleet])

  const opponentSunkCellMap = useMemo(() => sunkCellMap(opponentSunkShips), [opponentSunkShips])
  const ownSunkCellMap = useMemo(() => sunkCellMap(ownSunkShips), [ownSunkShips])
  const opponentSunkShipTypes = useMemo(
    () => new Set(opponentSunkShips.map((s) => s.type)),
    [opponentSunkShips],
  )
  const ownSunkShipTypes = useMemo(() => new Set(ownSunkShips.map((s) => s.type)), [ownSunkShips])

  // Sunk opponent vessels are revealed as ship art on the target radar.
  const revealedOpponentShips = useMemo(
    () =>
      opponentSunkShips
        .map((sunk) => sunkShipToPlacedShip(sunk))
        .filter((s): s is PlacedShip => s !== null),
    [opponentSunkShips],
  )

  const isMatchOver = Boolean(gameOverResult || activeRoom?.phase === 'finished')
  const isWinner = Boolean(
    gameOverResult
      ? gameOverResult.winnerPlayerId === playerId
      : activeRoom?.winnerPlayerId === playerId,
  )
  const gameOverReason = gameOverResult?.reason || activeRoom?.endReason || 'all_sunk'
  const winnerName = isWinner ? playerName : opponentName

  return {
    role,
    playerName,
    opponentName,
    sectorId,
    ownOccupiedMap,
    opponentSunkCellMap,
    ownSunkCellMap,
    opponentSunkShipTypes,
    ownSunkShipTypes,
    revealedOpponentShips,
    isMatchOver,
    isWinner,
    gameOverReason,
    winnerName,
  }
}
