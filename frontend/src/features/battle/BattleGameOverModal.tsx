import React from 'react'

export interface BattleGameOverModalProps {
  isWinner: boolean
  winnerName: string
  reason: string
  ownSunkCount: number
  opponentSunkCount: number
  isLeavingRoom: boolean
  onReturnToLobby: () => void
}

const REASON_LABELS: Record<string, string> = {
  all_sunk: 'All Ships Sunk',
  forfeit: 'Enemy Forfeited',
  disconnect_timeout: 'Connection Lost',
}

export const BattleGameOverModal: React.FC<BattleGameOverModalProps> = ({
  isWinner,
  winnerName,
  reason,
  ownSunkCount,
  opponentSunkCount,
  isLeavingRoom,
  onReturnToLobby,
}) => (
  <div
    role="dialog"
    aria-modal="true"
    aria-labelledby="gameover-title"
    className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/85 backdrop-blur-md animate-fade-in"
  >
    <div className="w-full max-w-lg p-6 sm:p-8 rounded-2xl bg-slate-900 border-2 shadow-2xl text-center space-y-6 relative overflow-hidden">
      <div
        className={`p-4 rounded-xl border ${
          isWinner
            ? 'bg-emerald-950/60 border-emerald-500/50 text-emerald-300'
            : 'bg-rose-950/60 border-rose-500/50 text-rose-300'
        }`}
      >
        <div className="text-4xl sm:text-5xl mb-2">{isWinner ? '🏆' : '💥'}</div>
        <h2
          id="gameover-title"
          className={`text-2xl sm:text-3xl font-mono font-black tracking-wider uppercase ${
            isWinner ? 'text-emerald-400' : 'text-rose-400'
          }`}
        >
          {isWinner ? 'VICTORY ACHIEVED' : 'DEFEAT — FLEET LOST'}
        </h2>
        <p className="text-xs sm:text-sm font-mono mt-1 text-slate-300">
          {isWinner
            ? 'Hostile fleet successfully eliminated.'
            : 'All friendly naval forces have been destroyed.'}
        </p>
      </div>

      <div className="grid grid-cols-2 gap-3 text-left font-mono text-xs">
        <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
          <span className="text-slate-400 block text-[10px] uppercase">Victor</span>
          <span className="text-white font-bold text-sm truncate block mt-0.5">{winnerName}</span>
        </div>
        <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
          <span className="text-slate-400 block text-[10px] uppercase">Outcome Reason</span>
          <span className="text-cyan-300 font-bold text-sm block mt-0.5 uppercase">
            {REASON_LABELS[reason] ?? reason}
          </span>
        </div>
        <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
          <span className="text-slate-400 block text-[10px] uppercase">Friendly Losses</span>
          <span className="text-rose-400 font-bold text-sm block mt-0.5">{ownSunkCount}/5 Sunk</span>
        </div>
        <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
          <span className="text-slate-400 block text-[10px] uppercase">Hostile Vessels Sunk</span>
          <span className="text-emerald-400 font-bold text-sm block mt-0.5">
            {opponentSunkCount}/5 Sunk
          </span>
        </div>
      </div>

      <div className="pt-2">
        <button
          type="button"
          onClick={onReturnToLobby}
          disabled={isLeavingRoom}
          className="w-full py-3.5 px-6 rounded-xl bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 text-white font-mono font-bold tracking-wider uppercase text-sm sm:text-base shadow-lg shadow-cyan-900/40 transition-all duration-150 disabled:opacity-50"
        >
          {isLeavingRoom ? 'Returning to Lobby...' : 'Return to Lobby'}
        </button>
      </div>
    </div>
  </div>
)
