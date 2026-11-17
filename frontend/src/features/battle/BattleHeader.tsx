import React from 'react'
import { ConnectionBadge } from '../../components/ConnectionBadge'
import { ConnectionStatus } from '../../types/protocol'

export interface BattleHeaderProps {
  sectorId: string
  role: string
  playerName: string
  opponentName: string
  connectionStatus: ConnectionStatus
  isSessionConflict: boolean
  isMatchOver: boolean
  onLeave: () => void
}

export const BattleHeader: React.FC<BattleHeaderProps> = ({
  sectorId,
  role,
  playerName,
  opponentName,
  connectionStatus,
  isSessionConflict,
  isMatchOver,
  onLeave,
}) => (
  <header className="flex flex-wrap items-center justify-between gap-4 p-4 rounded-xl bg-slate-900/80 border border-slate-800 shadow-xl backdrop-blur-sm">
    <div className="flex items-center gap-3">
      <div className="w-3 h-3 rounded-full bg-cyan-400 animate-ping" />
      <div>
        <h1 className="text-xl sm:text-2xl font-mono font-black tracking-wider uppercase text-white flex items-center gap-2">
          Battleship Combat
          <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/30 text-cyan-300 font-mono">
            SEC-{sectorId}
          </span>
        </h1>
        <p className="text-xs text-slate-400 font-mono">
          Role: <strong className="text-cyan-300 uppercase">{role}</strong> | Commander{' '}
          <strong className="text-white">{playerName}</strong> vs{' '}
          <strong className="text-slate-300">{opponentName}</strong>
        </p>
      </div>
    </div>

    <div className="flex items-center gap-3">
      <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
      {!isMatchOver && (
        <button
          type="button"
          onClick={onLeave}
          className="text-xs font-mono text-rose-400 hover:text-rose-300 underline underline-offset-4 transition-colors"
        >
          Leave Game
        </button>
      )}
    </div>
  </header>
)
