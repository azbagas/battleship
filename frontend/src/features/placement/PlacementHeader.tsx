import React from 'react'
import { BrandLogo } from '../../components/BrandLogo'
import { ConnectionBadge } from '../../components/ConnectionBadge'
import { ConnectionStatus } from '../../types/protocol'

export interface PlacementHeaderProps {
  sectorId: string
  role: 'host' | 'guest'
  playerName: string
  opponentName: string
  isOpponentConfirmed: boolean
  secondsRemaining: number
  connectionStatus: ConnectionStatus
  isSessionConflict: boolean
  lobbyError: string | null
  onDismissError: () => void
  onLeave: () => void
}

/** Top card: sector, connection, 60s deployment timer, opponent readiness and errors. */
export const PlacementHeader: React.FC<PlacementHeaderProps> = ({
  sectorId,
  role,
  playerName,
  opponentName,
  isOpponentConfirmed,
  secondsRemaining,
  connectionStatus,
  isSessionConflict,
  lobbyError,
  onDismissError,
  onLeave,
}) => (
  <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-4 sm:p-6 shadow-2xl border border-slate-800 text-slate-100">
    <div className="flex flex-wrap items-center justify-between gap-4 pb-4 border-b border-slate-800/80">
      <div className="flex items-center gap-3">
        <BrandLogo size="md" />
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-bold tracking-tight text-white">Battleship Combat</h1>
            <span className="px-2 py-0.5 text-[10px] font-mono uppercase rounded bg-slate-800 text-slate-300 border border-slate-700">
              Sector {sectorId}
            </span>
          </div>
          <p className="text-xs text-slate-400">Tactical Deployment Phase</p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        <button
          type="button"
          onClick={onLeave}
          className="text-xs font-mono text-rose-400 hover:text-rose-300 underline underline-offset-4 transition-colors"
        >
          Leave Game
        </button>
      </div>
    </div>

    <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mt-4 items-center">
      <div className="flex items-center gap-3 p-3 rounded-xl bg-slate-950/60 border border-slate-800/80">
        <div className="w-8 h-8 rounded-lg bg-cyan-950/80 border border-cyan-700/40 flex items-center justify-center text-cyan-400 font-bold text-xs uppercase">
          {role === 'host' ? 'H' : 'G'}
        </div>
        <div className="min-w-0">
          <span className="text-[10px] font-mono uppercase text-slate-400">Commander</span>
          <p className="text-sm font-semibold text-white truncate">{playerName}</p>
        </div>
      </div>

      <div className="p-3 rounded-xl bg-slate-950/60 border border-slate-800/80 text-center relative overflow-hidden">
        <div className="flex items-center justify-between text-xs font-mono mb-1">
          <span className="text-slate-400 uppercase text-[10px]">Launch Deadline</span>
          <span
            className={`font-bold transition-colors ${
              secondsRemaining <= 10
                ? 'text-rose-400 animate-pulse'
                : secondsRemaining <= 20
                  ? 'text-amber-400'
                  : 'text-cyan-400'
            }`}
          >
            {secondsRemaining}s
          </span>
        </div>
        <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all duration-500 ease-linear ${
              secondsRemaining <= 10
                ? 'bg-rose-500'
                : secondsRemaining <= 20
                  ? 'bg-amber-500'
                  : 'bg-cyan-500'
            }`}
            style={{ width: `${Math.min(100, (secondsRemaining / 60) * 100)}%` }}
          />
        </div>
      </div>

      <div className="flex items-center gap-3 p-3 rounded-xl bg-slate-950/60 border border-slate-800/80">
        <div
          className={`w-8 h-8 rounded-lg border flex items-center justify-center font-bold text-xs uppercase ${
            isOpponentConfirmed
              ? 'bg-emerald-950/80 border-emerald-500/50 text-emerald-400'
              : 'bg-slate-900 border-slate-700 text-slate-400'
          }`}
        >
          {isOpponentConfirmed ? '✓' : '...'}
        </div>
        <div className="min-w-0">
          <span className="text-[10px] font-mono uppercase text-slate-400 truncate block">
            Target: {opponentName}
          </span>
          <p
            className={`text-xs font-semibold truncate ${
              isOpponentConfirmed ? 'text-emerald-400' : 'text-slate-400'
            }`}
          >
            {isOpponentConfirmed ? 'Fleet Confirmed' : 'Positioning Fleet...'}
          </p>
        </div>
      </div>
    </div>

    {lobbyError && (
      <div className="mt-4 p-3 rounded-xl bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs flex items-center justify-between">
        <span>{lobbyError}</span>
        <button
          onClick={onDismissError}
          className="text-rose-400 hover:text-white px-2 py-0.5 rounded text-[11px]"
        >
          Dismiss
        </button>
      </div>
    )}
  </div>
)
