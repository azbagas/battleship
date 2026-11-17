import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { defenseCellAppearance, targetCellAppearance } from './cellAppearance'

const base = {
  notation: 'A1',
  shotResult: undefined,
  sunkShipType: undefined,
  isMyTurn: true,
  isMatchOver: false,
  isFocused: false,
}

describe('targetCellAppearance', () => {
  it('marks untargeted cells ready to fire on my turn', () => {
    const a = targetCellAppearance(base)
    expect(a.ariaLabel).toBe('Target cell A1, ready to fire')
    expect(a.cellStyle).toContain('cursor-crosshair')
    expect(a.isSunk).toBe(false)
  })

  it('adds the keyboard hint when focused', () => {
    expect(targetCellAppearance({ ...base, isFocused: true }).ariaLabel).toBe(
      'Target cell A1, ready to fire (press Enter or Space to fire)',
    )
  })

  it('stands by when it is not my turn or the match is over', () => {
    expect(targetCellAppearance({ ...base, isMyTurn: false }).ariaLabel).toBe('Target cell A1, stand by')
    expect(targetCellAppearance({ ...base, isMatchOver: true }).cellStyle).toContain('cursor-not-allowed')
  })

  it('describes miss, hit and sunk cells', () => {
    expect(targetCellAppearance({ ...base, shotResult: 'miss' }).ariaLabel).toBe(
      'Target cell A1, MISS (water splash)',
    )
    expect(targetCellAppearance({ ...base, shotResult: 'hit' }).ariaLabel).toBe(
      'Target cell A1, HIT (vessel damaged)',
    )
    const sunk = targetCellAppearance({ ...base, shotResult: 'sunk', sunkShipType: 'destroyer' })
    expect(sunk.isSunk).toBe(true)
    expect(sunk.ariaLabel).toBe('Target cell A1, SUNK destroyer (diagonal striped)')
  })

  it('treats a cell inside a sunk ship as sunk even without a sunk shot result', () => {
    const a = targetCellAppearance({ ...base, shotResult: 'hit', sunkShipType: 'cruiser' })
    expect(a.isSunk).toBe(true)
  })
})

describe('defenseCellAppearance', () => {
  const ship = { type: 'cruiser', origin: { x: 0, y: 0 }, orientation: 'horizontal' } as never
  const occupied = (index: number) => ({ ship, index })

  it('labels empty water, enemy misses and untouched ships', () => {
    expect(defenseCellAppearance({ notation: 'B2', occupied: undefined, shotResult: undefined, isSunk: false }).ariaLabel).toBe('Defensive cell B2')
    expect(defenseCellAppearance({ notation: 'B2', occupied: undefined, shotResult: 'miss', isSunk: false }).ariaLabel).toBe('Defensive cell B2, enemy miss')
    expect(defenseCellAppearance({ notation: 'B2', occupied: occupied(1), shotResult: undefined, isSunk: false }).ariaLabel).toBe('Defensive cell B2, cruiser')
  })

  it('labels hit and sunk ship cells', () => {
    expect(defenseCellAppearance({ notation: 'B2', occupied: occupied(0), shotResult: 'hit', isSunk: false }).ariaLabel).toBe('Defensive cell B2, HIT on cruiser')
    expect(defenseCellAppearance({ notation: 'B2', occupied: occupied(0), shotResult: 'hit', isSunk: true }).ariaLabel).toBe('Defensive cell B2, SUNK cruiser (diagonal striped)')
  })

  it('exposes the ship type to screen readers only on its first cell', () => {
    const first = render(<>{defenseCellAppearance({ notation: 'A1', occupied: occupied(0), shotResult: undefined, isSunk: false }).cellContent}</>)
    expect(first.container.querySelector('.sr-only')?.textContent).toBe('cruiser')
    const second = defenseCellAppearance({ notation: 'A2', occupied: occupied(1), shotResult: undefined, isSunk: false })
    expect(second.cellContent).toBeNull()
  })
})
