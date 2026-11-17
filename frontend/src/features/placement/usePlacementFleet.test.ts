import { describe, it, expect, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { usePlacementFleet } from './usePlacementFleet'
import { PlacedShip } from '../../types/protocol'

const setup = (overrides: Partial<Parameters<typeof usePlacementFleet>[0]> = {}) => {
  const submitFleet = vi.fn()
  const syncDraftFleet = vi.fn()
  const serverFleet: PlacedShip[] = []
  const hook = renderHook(() =>
    usePlacementFleet({
      isFleetConfirmed: false,
      serverFleet,
      submitFleet,
      syncDraftFleet,
      keyboardCoord: null,
      ...overrides,
    }),
  )
  return { ...hook, submitFleet, syncDraftFleet }
}

describe('usePlacementFleet', () => {
  it('places a selected ship on cell click and syncs a draft', () => {
    const { result, syncDraftFleet } = setup()

    act(() => result.current.handleDockClick('carrier'))
    expect(result.current.selectedShipType).toBe('carrier')

    act(() => result.current.handleCellClick(0, 0))
    expect(result.current.ships).toEqual([
      { type: 'carrier', origin: { x: 0, y: 0 }, orientation: 'horizontal' },
    ])
    expect(result.current.selectedShipType).toBeNull()
    expect(syncDraftFleet).toHaveBeenCalledTimes(1)
  })

  it('rejects out-of-bounds placement without syncing', () => {
    const { result, syncDraftFleet } = setup()

    act(() => result.current.handleDockClick('carrier'))
    act(() => result.current.handleCellClick(8, 0))
    expect(result.current.ships).toHaveLength(0)
    expect(syncDraftFleet).not.toHaveBeenCalled()
  })

  it('rotates a selected placed ship in place when valid', () => {
    const { result } = setup()

    act(() => result.current.handleDockClick('destroyer'))
    act(() => result.current.handleCellClick(2, 2))
    act(() => result.current.handleCellClick(2, 2)) // select the placed ship
    expect(result.current.selectedShipType).toBe('destroyer')

    act(() => result.current.handleRotate())
    expect(result.current.ships[0].orientation).toBe('vertical')
    expect(result.current.currentOrientation).toBe('vertical')
  })

  it('randomizes a complete fleet and resets to an empty dock', () => {
    const { result, syncDraftFleet } = setup()

    act(() => result.current.handleRandomize())
    expect(result.current.ships).toHaveLength(5)
    expect(result.current.isFleetComplete).toBe(true)

    act(() => result.current.handleReset())
    expect(result.current.ships).toHaveLength(0)
    expect(syncDraftFleet).toHaveBeenLastCalledWith([])
  })

  it('submits only a complete fleet', () => {
    const { result, submitFleet } = setup()

    act(() => result.current.handleConfirm())
    expect(submitFleet).not.toHaveBeenCalled()

    act(() => result.current.handleRandomize())
    act(() => result.current.handleConfirm())
    expect(submitFleet).toHaveBeenCalledWith(result.current.ships)
  })

  it('ignores every action once the fleet is confirmed', () => {
    const { result, syncDraftFleet } = setup({ isFleetConfirmed: true })

    act(() => result.current.handleRandomize())
    act(() => result.current.handleDockClick('carrier'))
    expect(result.current.ships).toHaveLength(0)
    expect(result.current.selectedShipType).toBeNull()
    expect(syncDraftFleet).not.toHaveBeenCalled()
  })

  it('adopts the server fleet when provided', () => {
    const serverFleet: PlacedShip[] = [
      { type: 'carrier', origin: { x: 0, y: 0 }, orientation: 'horizontal' },
    ]
    const { result } = setup({ serverFleet })
    expect(result.current.ships).toEqual(serverFleet)
  })
})
