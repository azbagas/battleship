import React from 'react'

export interface TurnStatusBannerProps {
  isMatchOver: boolean
  isMyTurn: boolean
  opponentName: string
  secondsRemaining: number
  isTimerUrgent: boolean
  timerPercentage: number
}

/** Whose-turn banner with the 30s salvo countdown and progress bar. */
export const TurnStatusBanner: React.FC<TurnStatusBannerProps> = ({
  isMatchOver,
  isMyTurn,
  opponentName,
  secondsRemaining,
  isTimerUrgent,
  timerPercentage,
}) => (
  <section
    role="status"
    aria-live="polite"
    className={`p-4 rounded-xl border transition-all duration-300 relative overflow-hidden shadow-lg ${
      isMatchOver
        ? 'bg-slate-900/90 border-slate-700'
        : isMyTurn
        ? 'bg-gradient-to-r from-cyan-950/80 via-emerald-950/50 to-slate-900/80 border-emerald-500/50 shadow-emerald-900/20'
        : 'bg-gradient-to-r from-slate-900/90 via-amber-950/40 to-slate-900/90 border-amber-500/40 shadow-amber-950/20'
    }`}
  >
    <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 relative z-10">
      <div className="flex items-center gap-3">
        <div
          className={`w-10 h-10 rounded-lg flex items-center justify-center font-mono font-bold text-lg border ${
            isMatchOver
              ? 'bg-slate-800 border-slate-700 text-slate-300'
              : isMyTurn
              ? 'bg-emerald-500/20 border-emerald-400 text-emerald-300 shadow-sm shadow-emerald-500/50 animate-pulse'
              : 'bg-amber-500/10 border-amber-500/40 text-amber-400'
          }`}
        >
          {isMatchOver ? '🏁' : isMyTurn ? '🎯' : '🛡️'}
        </div>
        <div>
          <div className="text-xs uppercase font-mono tracking-widest text-slate-400">
            {isMatchOver ? 'Engagement Concluded' : isMyTurn ? 'Command Priority' : 'Hostile Maneuver'}
          </div>
          <h2 className="text-base sm:text-lg font-mono font-black uppercase text-white">
            {isMatchOver
              ? 'Match Finished'
              : isMyTurn
              ? 'YOUR TURN — SELECT TARGET CELL'
              : `${opponentName.toUpperCase()}'S TURN — BRACE FOR IMPACT`}
          </h2>
        </div>
      </div>

      {!isMatchOver && (
        <div className="flex items-center gap-3 self-end sm:self-center">
          <div className="text-right">
            <div className="text-[10px] uppercase font-mono tracking-wider text-slate-400">
              {isTimerUrgent ? 'Auto-Shot Imminent' : 'Salvo Deadline'}
            </div>
            <div
              className={`text-xl sm:text-2xl font-mono font-black ${
                isTimerUrgent
                  ? 'text-rose-400 animate-pulse'
                  : isMyTurn
                  ? 'text-emerald-300'
                  : 'text-amber-300'
              }`}
            >
              {secondsRemaining}s
            </div>
          </div>
        </div>
      )}
    </div>

    {!isMatchOver && (
      <div className="w-full h-1.5 bg-slate-800/80 rounded-full mt-3 overflow-hidden">
        <div
          className={`h-full transition-all duration-300 ease-linear rounded-full ${
            isTimerUrgent ? 'bg-rose-500' : isMyTurn ? 'bg-emerald-400' : 'bg-amber-400'
          }`}
          style={{ width: `${timerPercentage}%` }}
        />
      </div>
    )}
  </section>
)
