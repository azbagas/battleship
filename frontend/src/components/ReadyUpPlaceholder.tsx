import React from 'react'
import { useLobby } from '../context/LobbyContext'
import { useSession } from '../context/SessionContext'
import { ConnectionBadge } from './ConnectionBadge'

export const ReadyUpPlaceholder: React.FC = () => {
  const { activeRoom } = useLobby()
  const { connectionStatus, isSessionConflict } = useSession()

  const hostName = activeRoom?.host?.name || 'Host'
  const guestName = activeRoom?.guest?.name || activeRoom?.opponentName || 'Guest'
  const role = activeRoom?.role === 'host' ? 'Host' : 'Guest'

  return (
    <div className="w-full max-w-lg mx-auto">
      <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-6 md:p-8 shadow-2xl border border-slate-800 text-slate-100">
        {/* Header Bar */}
        <div className="flex items-center justify-between pb-6 border-b border-slate-800/80 mb-6">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <svg
                className="w-5 h-5"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
                <circle cx="9" cy="7" r="4" />
                <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
                <path d="M16 3.13a4 4 0 0 1 0 7.75" />
              </svg>
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight text-white">Battleship</h1>
              <p className="text-xs text-slate-400">Combat Sector Matched</p>
            </div>
          </div>
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        </div>

        {/* Match Announcement */}
        <div className="text-center py-4 space-y-2">
          <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-emerald-950/60 border border-emerald-700/50 text-xs text-emerald-300 font-mono">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-ping" />
            OPPONENT CONNECTED
          </div>
          <h2 className="text-xl font-bold text-white">Sector Engagement Locked</h2>
          <p className="text-xs text-slate-400">
            Both commanders have joined. You are participating as <strong>{role}</strong>.
          </p>
        </div>

        {/* Combatants Roster */}
        <div className="grid grid-cols-2 gap-3 my-6">
          <div className="p-4 rounded-xl bg-slate-950/70 border border-slate-800 flex flex-col items-center text-center">
            <span className="text-[11px] font-mono uppercase text-cyan-400 mb-1">Host Commander</span>
            <span className="font-bold text-white text-base truncate max-w-full" title={hostName}>
              {hostName}
            </span>
          </div>
          <div className="p-4 rounded-xl bg-slate-950/70 border border-slate-800 flex flex-col items-center text-center">
            <span className="text-[11px] font-mono uppercase text-amber-400 mb-1">Guest Commander</span>
            <span className="font-bold text-white text-base truncate max-w-full" title={guestName}>
              {guestName}
            </span>
          </div>
        </div>

        {/* Phase transition note */}
        <div className="p-4 rounded-xl bg-cyan-950/30 border border-cyan-800/40 text-center text-xs text-cyan-200/90 leading-relaxed">
          Pre-Game Ready-Up controls will activate in <strong>Task T-07</strong>. Stand by for fleet readiness handshake.
        </div>
      </div>
    </div>
  )
}

