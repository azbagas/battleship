import React from 'react'
import { useLobby } from '../context/LobbyContext'
import { useCountdown } from '../hooks/useCountdown'

export interface OpponentDisconnectBannerProps {
  deadline?: string | null
  opponentName?: string
  description?: string
  countdownLabel?: string
}

export const OpponentDisconnectBanner: React.FC<OpponentDisconnectBannerProps> = ({
  deadline: propDeadline,
  opponentName = 'Opponent',
  description,
  countdownLabel,
}) => {
  const { opponentDisconnectDeadline, activeRoom } = useLobby()
  const activeDeadline = propDeadline !== undefined ? propDeadline : opponentDisconnectDeadline

  const isPreGame = activeRoom?.phase === 'ready-up'
  const isHost = activeRoom?.role === 'host'

  let defaultDescription =
    'Awaiting reconnection. If commander does not return within the 60s grace window, victory is awarded to you.'
  let defaultCountdownLabel = 'Forfeit in:'

  if (isPreGame) {
    defaultCountdownLabel = 'Timeout in:'
    defaultDescription = isHost
      ? 'Awaiting reconnection. If commander does not return within the 60s grace window, this room will return to the public waiting lobby.'
      : 'Awaiting reconnection. If commander does not return within the 60s grace window, the room will close and you will return to the lobby.'
  }

  const finalDescription = description || defaultDescription
  const finalCountdownLabel = countdownLabel || defaultCountdownLabel

  const secondsRemaining = useCountdown(activeDeadline, 60)

  if (!activeDeadline) {
    return null
  }

  const isUrgent = secondsRemaining <= 10

  return (
    <div
      role="alert"
      aria-live="assertive"
      className="w-full p-3.5 sm:p-4 rounded-xl bg-gradient-to-r from-amber-950/90 via-rose-950/80 to-slate-900/90 border border-amber-500/50 shadow-lg shadow-amber-950/30 text-white animate-fade-in"
    >
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-amber-500/20 border border-amber-400 flex items-center justify-center text-amber-300 font-bold text-base shrink-0 animate-pulse">
            ⚠️
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-mono uppercase tracking-widest px-2 py-0.5 rounded bg-amber-500/20 text-amber-300 font-bold border border-amber-500/40">
                CONNECTION DROPPED
              </span>
              <span className="text-xs text-slate-300 font-mono font-bold">
                {opponentName} is offline
              </span>
            </div>
            <p className="text-xs text-slate-300 font-mono mt-0.5">
              {finalDescription}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 self-end sm:self-center bg-slate-950/70 px-3 py-1.5 rounded-lg border border-slate-800 shrink-0">
          <span className="text-[10px] font-mono uppercase tracking-wider text-slate-400">
            {finalCountdownLabel}
          </span>
          <span
            data-testid="disconnect-countdown"
            className={`font-mono text-lg font-black ${
              isUrgent ? 'text-rose-400 animate-pulse' : 'text-amber-300'
            }`}
          >
            {secondsRemaining}s
          </span>
        </div>
      </div>
    </div>
  )
}

