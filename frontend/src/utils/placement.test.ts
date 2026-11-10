import { describe, it, expect } from 'vitest'
import {
  getShipCells,
  isShipInBounds,
  isValidPlacement,
  validateCompleteFleet,
  generateRandomFleet,
  coordToNotation,
  FLEET_SHIPS,
} from './placement'
import { PlacedShip } from '../types/protocol'

describe('placement utilities', () => {
  it('calculates occupied cells accurately for horizontal and vertical orientations', () => {
    const horizontalCells = getShipCells({ x: 2, y: 3 }, 3, 'horizontal')
    expect(horizontalCells).toEqual([
      { x: 2, y: 3 },
      { x: 3, y: 3 },
      { x: 4, y: 3 },
    ])

    const verticalCells = getShipCells({ x: 5, y: 1 }, 4, 'vertical')
    expect(verticalCells).toEqual([
      { x: 5, y: 1 },
      { x: 5, y: 2 },
      { x: 5, y: 3 },
      { x: 5, y: 4 },
    ])
  })

  it('detects in-bounds and out-of-bounds positions', () => {
    expect(isShipInBounds({ x: 0, y: 0 }, 5, 'horizontal')).toBe(true)
    expect(isShipInBounds({ x: 5, y: 0 }, 5, 'horizontal')).toBe(true)
    expect(isShipInBounds({ x: 6, y: 0 }, 5, 'horizontal')).toBe(false) // 6+5=11 > 10

    expect(isShipInBounds({ x: 0, y: 6 }, 4, 'vertical')).toBe(true)
    expect(isShipInBounds({ x: 0, y: 7 }, 4, 'vertical')).toBe(false) // 7+4=11 > 10
    expect(isShipInBounds({ x: -1, y: 0 }, 2, 'horizontal')).toBe(false)
  })

  it('validates candidate ship placement against existing ships', () => {
    const existing: PlacedShip[] = [
      { type: 'carrier', origin: { x: 0, y: 0 }, orientation: 'horizontal' }, // (0..4, 0)
    ]

    // Overlapping candidate
    const overlapping: PlacedShip = {
      type: 'battleship',
      origin: { x: 2, y: 0 },
      orientation: 'vertical',
    }
    expect(isValidPlacement(overlapping, existing)).toBe(false)

    // Non-overlapping (adjacent) candidate
    const adjacent: PlacedShip = {
      type: 'battleship',
      origin: { x: 0, y: 1 },
      orientation: 'horizontal',
    }
    expect(isValidPlacement(adjacent, existing)).toBe(true)

    // Moving existing ship to same place is allowed (skips self)
    expect(isValidPlacement(existing[0], existing)).toBe(true)
  })

  it('validates a complete fleet and detects errors', () => {
    const validFleet: PlacedShip[] = [
      { type: 'carrier', origin: { x: 0, y: 0 }, orientation: 'horizontal' },
      { type: 'battleship', origin: { x: 0, y: 1 }, orientation: 'horizontal' },
      { type: 'cruiser', origin: { x: 0, y: 2 }, orientation: 'horizontal' },
      { type: 'submarine', origin: { x: 0, y: 3 }, orientation: 'horizontal' },
      { type: 'destroyer', origin: { x: 0, y: 4 }, orientation: 'horizontal' },
    ]

    expect(validateCompleteFleet(validFleet).valid).toBe(true)

    // Missing ship
    expect(validateCompleteFleet(validFleet.slice(0, 4)).valid).toBe(false)

    // Duplicate ship type
    const duplicateFleet: PlacedShip[] = [
      ...validFleet.slice(0, 4),
      { type: 'carrier', origin: { x: 0, y: 5 }, orientation: 'horizontal' },
    ]
    expect(validateCompleteFleet(duplicateFleet).valid).toBe(false)
  })

  it('generates 100 valid random fleets with 0 overlaps', () => {
    for (let i = 0; i < 100; i++) {
      const fleet = generateRandomFleet()
      expect(fleet).toHaveLength(FLEET_SHIPS.length)
      const res = validateCompleteFleet(fleet)
      expect(res.valid).toBe(true)
    }
  })

  it('formats coordinate to algebraic notation', () => {
    expect(coordToNotation({ x: 0, y: 0 })).toBe('A1')
    expect(coordToNotation({ x: 9, y: 9 })).toBe('J10')
    expect(coordToNotation({ x: 4, y: 6 })).toBe('E7')
  })
})

