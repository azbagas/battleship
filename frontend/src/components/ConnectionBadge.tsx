import React from 'react'
import { ConnectionStatus } from '../types/protocol'

interface ConnectionBadgeProps {
  status: ConnectionStatus
  isConflict?: boolean
}

export const ConnectionBadge: React.FC<ConnectionBadgeProps> = ({ status, isConflict }) => {
  if (isConflict) {
    return (
      <div
        role="status"
        className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-rose-500/10 border border-rose-500/25 text-rose-400 text-xs font-medium"
      >
        <span className="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
        Tab Conflict
      </div>
    )
  }

  switch (status) {
    case 'connected':
      return (
        <div
          role="status"
          className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/25 text-emerald-400 text-xs font-medium"
        >
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
          Connected
        </div>
      )
    case 'connecting':
      return (
        <div
          role="status"
          className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-amber-500/10 border border-amber-500/25 text-amber-400 text-xs font-medium"
        >
          <span className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-ping"></span>
          Connecting
        </div>
      )
    case 'error':
      return (
        <div
          role="status"
          className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-rose-500/10 border border-rose-500/25 text-rose-400 text-xs font-medium"
        >
          <span className="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
          Error
        </div>
      )
    case 'disconnected':
    default:
      return (
        <div
          role="status"
          className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-slate-700/40 border border-slate-600/30 text-slate-400 text-xs font-medium"
        >
          <span className="w-1.5 h-1.5 rounded-full bg-slate-500"></span>
          Offline
        </div>
      )
  }
}

