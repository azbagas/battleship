import { Coord, Orientation, PlacedShip, ShipType, SunkShipInfo } from '../types/protocol'

export const GRID_SIZE = 10
export const COLUMN_LABELS = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J']
export const ROW_LABELS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '10']

export interface ShipDefinition {
  type: ShipType
  name: string
  size: number
}

export const FLEET_SHIPS: readonly ShipDefinition[] = [
  { type: 'carrier', name: 'Carrier', size: 5 },
  { type: 'battleship', name: 'Battleship', size: 4 },
  { type: 'cruiser', name: 'Cruiser', size: 3 },
  { type: 'submarine', name: 'Submarine', size: 3 },
  { type: 'destroyer', name: 'Destroyer', size: 2 },
] as const

export const SHIP_SIZES: Record<ShipType, number> = {
  carrier: 5,
  battleship: 4,
  cruiser: 3,
  submarine: 3,
  destroyer: 2,
}

// getShipCells returns all grid coordinates occupied by a ship.
export function getShipCells(origin: Coord, size: number, orientation: Orientation): Coord[] {
  const cells: Coord[] = []
  for (let i = 0; i < size; i++) {
    if (orientation === 'horizontal') {
      cells.push({ x: origin.x + i, y: origin.y })
    } else {
      cells.push({ x: origin.x, y: origin.y + i })
    }
  }
  return cells
}

// isShipInBounds returns true if all cells of the ship fall inside the 10×10 grid.
export function isShipInBounds(origin: Coord, size: number, orientation: Orientation): boolean {
  if (orientation === 'horizontal') {
    return origin.x >= 0 && origin.x + size <= GRID_SIZE && origin.y >= 0 && origin.y < GRID_SIZE
  }
  return origin.x >= 0 && origin.x < GRID_SIZE && origin.y >= 0 && origin.y + size <= GRID_SIZE
}

// cellsOverlap returns true if any coordinate is present in both sets.
export function cellsOverlap(cellsA: Coord[], cellsB: Coord[]): boolean {
  return cellsA.some((a) => cellsB.some((b) => a.x === b.x && a.y === b.y))
}

// isValidPlacement checks whether a candidate ship fits within the grid and does not overlap other ships.
export function isValidPlacement(candidate: PlacedShip, existingShips: PlacedShip[]): boolean {
  const size = SHIP_SIZES[candidate.type]
  if (!size || !isShipInBounds(candidate.origin, size, candidate.orientation)) {
    return false
  }

  const candidateCells = getShipCells(candidate.origin, size, candidate.orientation)
  for (const other of existingShips) {
    if (other.type === candidate.type) continue // ignore self when moving
    const otherSize = SHIP_SIZES[other.type]
    const otherCells = getShipCells(other.origin, otherSize, other.orientation)
    if (cellsOverlap(candidateCells, otherCells)) {
      return false
    }
  }

  return true
}

// validateCompleteFleet checks that exactly 5 ships are legally placed without overlaps.
export function validateCompleteFleet(ships: PlacedShip[]): { valid: boolean; error?: string } {
  if (ships.length !== FLEET_SHIPS.length) {
    return { valid: false, error: `Fleet must have 5 ships (currently has ${ships.length})` }
  }

  const seenTypes = new Set<ShipType>()
  const occupied = new Set<string>()

  for (const ship of ships) {
    if (seenTypes.has(ship.type)) {
      return { valid: false, error: `Duplicate ship type: ${ship.type}` }
    }
    seenTypes.add(ship.type)

    const size = SHIP_SIZES[ship.type]
    if (!size || !isShipInBounds(ship.origin, size, ship.orientation)) {
      return { valid: false, error: `${ship.type} is out of bounds` }
    }

    const cells = getShipCells(ship.origin, size, ship.orientation)
    for (const cell of cells) {
      const key = `${cell.x},${cell.y}`
      if (occupied.has(key)) {
        return { valid: false, error: `Overlap detected at cell ${coordToNotation(cell)}` }
      }
      occupied.add(key)
    }
  }

  for (const def of FLEET_SHIPS) {
    if (!seenTypes.has(def.type)) {
      return { valid: false, error: `Missing ${def.name}` }
    }
  }

  return { valid: true }
}

// generateRandomFleet generates a valid random 5-ship fleet on the client.
export function generateRandomFleet(): PlacedShip[] {
  const orientations: Orientation[] = ['horizontal', 'vertical']
  const result: PlacedShip[] = []
  const occupied = new Set<string>()

  for (const def of FLEET_SHIPS) {
    let placed = false
    let attempts = 0
    while (!placed && attempts < 1000) {
      attempts++
      const orientation = orientations[Math.floor(Math.random() * orientations.length)]
      const maxX = orientation === 'horizontal' ? GRID_SIZE - def.size : GRID_SIZE - 1
      const maxY = orientation === 'vertical' ? GRID_SIZE - def.size : GRID_SIZE - 1

      const origin: Coord = {
        x: Math.floor(Math.random() * (maxX + 1)),
        y: Math.floor(Math.random() * (maxY + 1)),
      }

      const cells = getShipCells(origin, def.size, orientation)
      const hasOverlap = cells.some((c) => occupied.has(`${c.x},${c.y}`))

      if (!hasOverlap) {
        cells.forEach((c) => occupied.add(`${c.x},${c.y}`))
        result.push({
          type: def.type,
          origin,
          orientation,
        })
        placed = true
      }
    }

    if (!placed) {
      // If random collision occurred, restart fresh
      return generateRandomFleet()
    }
  }

  return result
}

// coordToNotation converts (x, y) to algebraic grid notation (e.g. 0,0 -> "A1").
export function coordToNotation(c: Coord): string {
  if (c.x < 0 || c.x >= GRID_SIZE || c.y < 0 || c.y >= GRID_SIZE) {
    return `?(${c.x},${c.y})`
  }
  return `${COLUMN_LABELS[c.x]}${ROW_LABELS[c.y]}`
}

// sunkShipToPlacedShip reconstructs a PlacedShip from a SunkShipInfo payload on the Target Radar.
export function sunkShipToPlacedShip(sunk: SunkShipInfo): PlacedShip | null {
  if (!sunk.cells || sunk.cells.length === 0) {
    return null
  }
  const size = SHIP_SIZES[sunk.type] || sunk.cells.length
  const xs = sunk.cells.map((c) => c.x)
  const ys = sunk.cells.map((c) => c.y)
  const minX = Math.min(...xs)
  const maxX = Math.max(...xs)
  const minY = Math.min(...ys)
  const maxY = Math.max(...ys)

  const orientation: Orientation = maxY > minY && maxX === minX ? 'vertical' : 'horizontal'
  const clampedX =
    orientation === 'horizontal' ? Math.max(0, Math.min(minX, 10 - size)) : Math.max(0, Math.min(minX, 9))
  const clampedY =
    orientation === 'vertical' ? Math.max(0, Math.min(minY, 10 - size)) : Math.max(0, Math.min(minY, 9))

  return {
    type: sunk.type,
    origin: { x: clampedX, y: clampedY },
    orientation,
  }
}


