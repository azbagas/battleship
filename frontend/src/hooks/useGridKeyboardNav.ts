import { useEffect, useState } from 'react'
import { Coord } from '../types/protocol'

const MAX_INDEX = 9

/** Returns the next coordinate for an arrow key, or null if the key is not an arrow. */
export function moveCoord(coord: Coord, key: string): Coord | null {
  switch (key) {
    case 'ArrowUp':
      return { ...coord, y: Math.max(0, coord.y - 1) }
    case 'ArrowDown':
      return { ...coord, y: Math.min(MAX_INDEX, coord.y + 1) }
    case 'ArrowLeft':
      return { ...coord, x: Math.max(0, coord.x - 1) }
    case 'ArrowRight':
      return { ...coord, x: Math.min(MAX_INDEX, coord.x + 1) }
    default:
      return null
  }
}

/** True when a key event originates from a text field and must not trigger game shortcuts. */
export function isTypingTarget(target: EventTarget | null): boolean {
  const tag = (target as HTMLElement | null)?.tagName?.toLowerCase()
  return tag === 'input' || tag === 'textarea'
}

export interface UseGridKeyboardNavOptions {
  /** When false, arrow keys are ignored. */
  enabled: boolean
  /** Called when an arrow key moves the focus (e.g. to flag keyboard navigation). */
  onArrow?: () => void
}

/** Tracks a focused cell on a 10x10 grid driven by the arrow keys. */
export function useGridKeyboardNav({ enabled, onArrow }: UseGridKeyboardNavOptions) {
  const [focused, setFocused] = useState<Coord>({ x: 0, y: 0 })

  useEffect(() => {
    if (!enabled) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (isTypingTarget(e.target)) return
      const next = moveCoord(focused, e.key)
      if (!next) return
      e.preventDefault()
      onArrow?.()
      setFocused(next)
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [enabled, focused, onArrow])

  return [focused, setFocused] as const
}
