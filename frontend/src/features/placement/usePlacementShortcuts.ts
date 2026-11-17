import { useEffect } from 'react'
import { Coord, ShipType } from '../../types/protocol'
import { FLEET_SHIPS } from '../../utils/placement'
import { isTypingTarget } from '../../hooks/useGridKeyboardNav'

export interface PlacementShortcutsOptions {
  isFleetConfirmed: boolean
  isFleetComplete: boolean
  focusedCoord: Coord
  onRotate: () => void
  onCellActivate: (x: number, y: number) => void
  onConfirm: () => void
  onSelectShip: (type: ShipType | null) => void
  /** Called when a shortcut starts keyboard-driven interaction (1-5). */
  onKeyboardUse: () => void
}

/** Placement hotkeys besides the arrows: 1-5 select, R rotate, Enter/Space place, Esc clear, C confirm. */
export function usePlacementShortcuts({
  isFleetConfirmed,
  isFleetComplete,
  focusedCoord,
  onRotate,
  onCellActivate,
  onConfirm,
  onSelectShip,
  onKeyboardUse,
}: PlacementShortcutsOptions) {
  useEffect(() => {
    if (isFleetConfirmed) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (isTypingTarget(e.target)) return

      if (e.key === 'r' || e.key === 'R') {
        e.preventDefault()
        onRotate()
        return
      }

      if (['1', '2', '3', '4', '5'].includes(e.key)) {
        const ship = FLEET_SHIPS[parseInt(e.key, 10) - 1]
        if (ship) {
          e.preventDefault()
          onKeyboardUse()
          onSelectShip(ship.type)
        }
        return
      }

      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault()
        onCellActivate(focusedCoord.x, focusedCoord.y)
        return
      }

      if (e.key === 'Escape') {
        e.preventDefault()
        onSelectShip(null)
        return
      }

      if ((e.key === 'c' || e.key === 'C') && isFleetComplete) {
        e.preventDefault()
        onConfirm()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [
    isFleetConfirmed,
    isFleetComplete,
    focusedCoord,
    onRotate,
    onCellActivate,
    onConfirm,
    onSelectShip,
    onKeyboardUse,
  ])
}
