import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useElapsedTime, formatDuration, calculateElapsedSeconds } from './useElapsedTime'

describe('formatDuration helper', () => {
  it('formats seconds into mm:ss', () => {
    expect(formatDuration(0)).toBe('00:00')
    expect(formatDuration(9)).toBe('00:09')
    expect(formatDuration(65)).toBe('01:05')
    expect(formatDuration(360)).toBe('06:00')
    expect(formatDuration(3665)).toBe('61:05')
  })

  it('clamps negative numbers to 00:00', () => {
    expect(formatDuration(-15)).toBe('00:00')
  })
})

describe('calculateElapsedSeconds helper', () => {
  it('calculates elapsed seconds accurately', () => {
    const now = 1000000000000
    const created = new Date(now - 45000).toISOString()
    expect(calculateElapsedSeconds(created, now)).toBe(45)
  })

  it('returns 0 for empty or invalid timestamps', () => {
    expect(calculateElapsedSeconds(null)).toBe(0)
    expect(calculateElapsedSeconds(undefined)).toBe(0)
    expect(calculateElapsedSeconds('invalid-date')).toBe(0)
  })

  it('clamps future timestamps to 0', () => {
    const now = 1000000000000
    const future = new Date(now + 5000).toISOString()
    expect(calculateElapsedSeconds(future, now)).toBe(0)
  })
})

describe('useElapsedTime hook', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns 00:00 when createdAt is not provided', () => {
    const { result } = renderHook(() => useElapsedTime(null))
    expect(result.current.elapsedSeconds).toBe(0)
    expect(result.current.formatted).toBe('00:00')
  })

  it('ticks every second', () => {
    const initialTime = Date.now()
    vi.setSystemTime(initialTime)

    const createdAt = new Date(initialTime - 10000).toISOString() // 10s ago
    const { result } = renderHook(() => useElapsedTime(createdAt))

    expect(result.current.elapsedSeconds).toBe(10)
    expect(result.current.formatted).toBe('00:10')

    // Advance 5 seconds
    act(() => {
      vi.advanceTimersByTime(5000)
    })

    expect(result.current.elapsedSeconds).toBe(15)
    expect(result.current.formatted).toBe('00:15')
  })

  it('cleans up interval on unmount', () => {
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval')
    const createdAt = new Date().toISOString()
    const { unmount } = renderHook(() => useElapsedTime(createdAt))

    unmount()
    expect(clearIntervalSpy).toHaveBeenCalled()
  })
})

