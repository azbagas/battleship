import { useState, useEffect } from 'react'

export function formatDuration(totalSeconds: number): string {
  const clamped = Math.max(0, Math.floor(totalSeconds))
  const minutes = Math.floor(clamped / 60)
  const seconds = clamped % 60
  const mm = String(minutes).padStart(2, '0')
  const ss = String(seconds).padStart(2, '0')
  return `${mm}:${ss}`
}

export function calculateElapsedSeconds(createdAt?: string | null, nowMs: number = Date.now()): number {
  if (!createdAt) return 0
  const createdMs = new Date(createdAt).getTime()
  if (Number.isNaN(createdMs)) return 0
  const diffSec = Math.floor((nowMs - createdMs) / 1000)
  return Math.max(0, diffSec)
}

export interface UseElapsedTimeResult {
  elapsedSeconds: number
  formatted: string
}

export function useElapsedTime(createdAt?: string | null): UseElapsedTimeResult {
  const [elapsedSeconds, setElapsedSeconds] = useState<number>(() => calculateElapsedSeconds(createdAt))

  useEffect(() => {
    // Sync immediately on createdAt change
    setElapsedSeconds(calculateElapsedSeconds(createdAt))

    if (!createdAt) return

    const intervalId = setInterval(() => {
      setElapsedSeconds(calculateElapsedSeconds(createdAt))
    }, 1000)

    return () => clearInterval(intervalId)
  }, [createdAt])

  return {
    elapsedSeconds,
    formatted: formatDuration(elapsedSeconds),
  }
}

