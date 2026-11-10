import React from 'react'
import { useLobby } from '../context/LobbyContext'
import { useSession } from '../context/SessionContext'
import { ConnectionBadge } from './ConnectionBadge'

export const PlacementPlaceholder: React.FC = () => {
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
            <div className="w-9 h-9 rounded-lg bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
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
                <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
                <line x1="3" y1="9" x2="21" y2="9" />
                <line x1="3" y1="15" x2="21" y2="15" />
                <line x1="9" y1="3" x2="9" y2="21" />
                <line x1="15" y1="3" x2="15" y2="21" />
              </svg>
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight text-white">Battleship</h1>
              <p className="text-xs text-slate-400">Phase: Fleet Placement</p>
            </div>
          </div>
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        </div>

        {/* Phase Announcement */}
        <div className="text-center py-4 space-y-2">
          <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-cyan-950/60 border border-cyan-700/50 text-xs text-cyan-300 font-mono">
            <span className="w-2 h-2 rounded-full bg-cyan-400 animate-ping" />
            BOTH COMMANDERS READY
          </div>
          <h2 className="text-xl font-bold text-white">Fleet Dock Initializing</h2>
          <p className="text-xs text-slate-400">
            Combat sector matched. Playing as <strong>{role}</strong> against{' '}
            <strong>{role === 'Host' ? guestName : hostName}</strong>.
          </p>
        </div>

        {/* Roster Summary */}
        <div className="grid grid-cols-2 gap-3 my-6">
          <div className="p-4 rounded-xl bg-slate-950/70 border border-slate-800 flex flex-col items-center text-center">
            <span className="text-[11px] font-mono uppercase text-cyan-400 mb-1">Host</span>
            <span className="font-bold text-white text-base truncate max-w-full" title={hostName}>
              {hostName}
            </span>
          </div>
          <div className="p-4 rounded-xl bg-slate-950/70 border border-slate-800 flex flex-col items-center text-center">
            <span className="text-[11px] font-mono uppercase text-amber-400 mb-1">Guest</span>
            <span className="font-bold text-white text-base truncate max-w-full" title={guestName}>
              {guestName}
            </span>
          </div>
        </div>

        {/* Next Task Notice */}
        <div className="p-4 rounded-xl bg-cyan-950/30 border border-cyan-800/40 text-center text-xs text-cyan-200/90 leading-relaxed">
          10×10 Fleet Placement grid, drag-and-drop dock, and 60-second timer will activate in{' '}
          <strong>Task T-10</strong>.
        </div>
      </div>
    </div>
  )
}
