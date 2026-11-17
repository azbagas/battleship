import { useEffect, useState } from 'react'

export interface UseCountdownOptions {
  /** Tick interval in milliseconds. */
  intervalMs?: number
  /** When false the countdown stops ticking and keeps its last value. */
  active?: boolean
}

/**
 * Whole seconds remaining until `deadline` (ISO timestamp), clamped at 0.
 * With no deadline (or while inactive) it keeps the last value, starting at `initial`,
 * so a UI never flashes 0 between phases.
 */
export function useCountdown(
  deadline: string | null | undefined,
  initial: number,
  { intervalMs = 250, active = true }: UseCountdownOptions = {},
): number {
  const [secondsRemaining, setSecondsRemaining] = useState<number>(initial)

  useEffect(() => {
    if (!deadline || !active) return

    const targetTime = new Date(deadline).getTime()
    const update = () => {
      setSecondsRemaining(Math.max(0, Math.ceil((targetTime - Date.now()) / 1000)))
    }

    update()
    const interval = setInterval(update, intervalMs)
    return () => clearInterval(interval)
  }, [deadline, active, intervalMs])

  return secondsRemaining
}
