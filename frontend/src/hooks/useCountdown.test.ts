import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useCountdown } from './useCountdown'

describe('useCountdown', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000_000_000_000)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns the initial value when there is no deadline', () => {
    const { result } = renderHook(() => useCountdown(null, 30))
    expect(result.current).toBe(30)
  })

  it('computes and ticks down seconds remaining', () => {
    const deadline = new Date(Date.now() + 10_000).toISOString()
    const { result } = renderHook(() => useCountdown(deadline, 60))
    expect(result.current).toBe(10)

    act(() => {
      vi.advanceTimersByTime(4_000)
    })
    expect(result.current).toBe(6)
  })

  it('clamps at 0 once the deadline has passed', () => {
    const deadline = new Date(Date.now() - 5_000).toISOString()
    const { result } = renderHook(() => useCountdown(deadline, 60))
    expect(result.current).toBe(0)
  })

  it('stops ticking while inactive and keeps the last value', () => {
    const deadline = new Date(Date.now() + 10_000).toISOString()
    const { result, rerender } = renderHook(
      ({ active }) => useCountdown(deadline, 60, { active }),
      { initialProps: { active: true } },
    )
    expect(result.current).toBe(10)

    rerender({ active: false })
    act(() => {
      vi.advanceTimersByTime(5_000)
    })
    expect(result.current).toBe(10)
  })

  it('clears its interval on unmount', () => {
    const spy = vi.spyOn(window, 'clearInterval')
    const deadline = new Date(Date.now() + 10_000).toISOString()
    const { unmount } = renderHook(() => useCountdown(deadline, 60))
    unmount()
    expect(spy).toHaveBeenCalled()
  })
})
