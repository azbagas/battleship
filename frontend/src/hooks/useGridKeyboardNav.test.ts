import { describe, it, expect, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { moveCoord, isTypingTarget, useGridKeyboardNav } from './useGridKeyboardNav'

describe('moveCoord', () => {
  it('moves one cell per arrow key', () => {
    expect(moveCoord({ x: 3, y: 3 }, 'ArrowUp')).toEqual({ x: 3, y: 2 })
    expect(moveCoord({ x: 3, y: 3 }, 'ArrowDown')).toEqual({ x: 3, y: 4 })
    expect(moveCoord({ x: 3, y: 3 }, 'ArrowLeft')).toEqual({ x: 2, y: 3 })
    expect(moveCoord({ x: 3, y: 3 }, 'ArrowRight')).toEqual({ x: 4, y: 3 })
  })

  it('clamps at grid edges', () => {
    expect(moveCoord({ x: 0, y: 0 }, 'ArrowUp')).toEqual({ x: 0, y: 0 })
    expect(moveCoord({ x: 0, y: 0 }, 'ArrowLeft')).toEqual({ x: 0, y: 0 })
    expect(moveCoord({ x: 9, y: 9 }, 'ArrowDown')).toEqual({ x: 9, y: 9 })
    expect(moveCoord({ x: 9, y: 9 }, 'ArrowRight')).toEqual({ x: 9, y: 9 })
  })

  it('returns null for non-arrow keys', () => {
    expect(moveCoord({ x: 1, y: 1 }, 'Enter')).toBeNull()
  })
})

describe('isTypingTarget', () => {
  it('detects input and textarea elements only', () => {
    expect(isTypingTarget(document.createElement('input'))).toBe(true)
    expect(isTypingTarget(document.createElement('textarea'))).toBe(true)
    expect(isTypingTarget(document.createElement('button'))).toBe(false)
    expect(isTypingTarget(null)).toBe(false)
  })
})

describe('useGridKeyboardNav', () => {
  it('updates focus on arrow keys and calls onArrow', () => {
    const onArrow = vi.fn()
    const { result } = renderHook(() => useGridKeyboardNav({ enabled: true, onArrow }))

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }))
    })
    expect(result.current[0]).toEqual({ x: 1, y: 0 })
    expect(onArrow).toHaveBeenCalledTimes(1)
  })

  it('ignores keys when disabled', () => {
    const { result } = renderHook(() => useGridKeyboardNav({ enabled: false }))
    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }))
    })
    expect(result.current[0]).toEqual({ x: 0, y: 0 })
  })
})
