import React from 'react'
import { BattleShotResultPayload } from '../../types/protocol'
import { coordToNotation } from '../../utils/placement'

export interface CombatLogProps {
  events: BattleShotResultPayload[]
  playerId: string | undefined
  playerName: string
  opponentName: string
  visibleOnMobile: boolean
}

function outcomeOf(evt: BattleShotResultPayload): { badgeColor: string; outcomeText: string } {
  switch (evt.result) {
    case 'sunk':
      return {
        badgeColor: 'text-rose-300 bg-rose-500/20 border-rose-500/50',
        outcomeText: `SUNK ${evt.sunkShip?.type.toUpperCase()}!`,
      }
    case 'hit':
      return {
        badgeColor: 'text-amber-300 bg-amber-500/20 border-amber-500/50',
        outcomeText: 'DIRECT HIT',
      }
    default:
      return {
        badgeColor: 'text-cyan-400 bg-cyan-500/10 border-cyan-500/30',
        outcomeText: 'MISS',
      }
  }
}

export const CombatLog: React.FC<CombatLogProps> = ({
  events,
  playerId,
  playerName,
  opponentName,
  visibleOnMobile,
}) => (
  <div
    className={`p-4 sm:p-5 rounded-2xl bg-slate-900/70 border border-slate-800 shadow-xl backdrop-blur-sm ${
      visibleOnMobile ? 'block' : 'hidden lg:block'
    }`}
  >
    <div className="flex items-center justify-between mb-2">
      <h3 className="text-sm font-mono font-bold uppercase tracking-wider text-white flex items-center gap-2">
        <span>📜</span> Tactical Combat Log
      </h3>
      <span className="text-[10px] font-mono text-slate-400">Latest Salvos</span>
    </div>

    <div
      tabIndex={0}
      aria-label="Combat event log"
      className="max-h-52 overflow-y-auto space-y-1.5 pr-1 font-mono text-xs focus:outline-none focus:ring-1 focus:ring-cyan-500/40 rounded"
    >
      {events.length === 0 ? (
        <div className="text-slate-500 italic py-4 text-center">
          Engagement in progress. Waiting for opening salvos...
        </div>
      ) : (
        events.map((evt, idx) => {
          const shooterName = evt.by === playerId ? playerName : opponentName
          const { badgeColor, outcomeText } = outcomeOf(evt)

          return (
            <div
              key={idx}
              className="flex items-center justify-between p-1.5 rounded bg-slate-950/60 border border-slate-800/80 gap-2"
            >
              <div className="flex items-center gap-1.5 truncate">
                <span className="text-slate-400 font-bold truncate">{shooterName}:</span>
                <span className="text-white font-bold">{coordToNotation(evt.cell)}</span>
                {evt.auto && (
                  <span className="text-[9px] px-1 py-0.2 rounded bg-purple-500/20 text-purple-300 border border-purple-500/40 font-mono">
                    AUTO
                  </span>
                )}
              </div>
              <span
                className={`text-[10px] px-1.5 py-0.5 rounded border font-bold uppercase whitespace-nowrap ${badgeColor}`}
              >
                {outcomeText}
              </span>
            </div>
          )
        })
      )}
    </div>
  </div>
)
