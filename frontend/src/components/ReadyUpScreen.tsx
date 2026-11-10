import React from 'react'
import { useLobby } from '../context/LobbyContext'
import { useSession } from '../context/SessionContext'
import { ConnectionBadge } from './ConnectionBadge'

export const ReadyUpScreen: React.FC = () => {
  const { activeRoom, readyState, isTogglingReady, isLeavingRoom, toggleReady, leaveRoom } = useLobby()
  const { connectionStatus, isSessionConflict } = useSession()

  const hostName = activeRoom?.host?.name || 'Host Commander'
  const guestName = activeRoom?.guest?.name || activeRoom?.opponentName || 'Guest Commander'
  const role = activeRoom?.role || 'host'

  const isHostReady = !!readyState?.hostReady
  const isGuestReady = !!readyState?.guestReady
  const isSelfReady = role === 'host' ? isHostReady : isGuestReady
  const isOpponentReady = role === 'host' ? isGuestReady : isHostReady
  const opponentName = role === 'host' ? guestName : hostName

  // Contextual guidance text
  let guidanceText = 'Both commanders must confirm fleet readiness before deployment commences.'
  let statusBadgeLabel = 'AWAITING READINESS'
  let statusBadgeColor = 'border-cyan-800/50 bg-cyan-950/40 text-cyan-300'

  if (isHostReady && isGuestReady) {
    guidanceText = 'Both commanders ready! Transitioning to fleet placement...'
    statusBadgeLabel = 'DEPLOYMENT ENGAGED'
    statusBadgeColor = 'border-emerald-700/60 bg-emerald-950/60 text-emerald-300'
  } else if (isSelfReady && !isOpponentReady) {
    guidanceText = `Readiness locked. Standing by for ${opponentName} to confirm...`
    statusBadgeLabel = 'WAITING FOR OPPONENT'
    statusBadgeColor = 'border-amber-700/50 bg-amber-950/40 text-amber-300'
  } else if (!isSelfReady && isOpponentReady) {
    guidanceText = `${opponentName} is ready! Confirm your readiness to initiate battle deployment.`
    statusBadgeLabel = 'OPPONENT READY'
    statusBadgeColor = 'border-emerald-700/50 bg-emerald-950/40 text-emerald-300'
  }

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
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                <path d="m9 12 2 2 4-4" />
              </svg>
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight text-white">Battleship</h1>
              <p className="text-xs text-slate-400">Pre-Game Readiness Check</p>
            </div>
          </div>
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        </div>

        {/* Status Announcement Banner */}
        <div className="text-center py-2 space-y-2 mb-6">
          <div
            className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full border text-xs font-mono uppercase tracking-wider ${statusBadgeColor}`}
          >
            <span
              className={`w-2 h-2 rounded-full ${
                isSelfReady && isOpponentReady
                  ? 'bg-emerald-400 animate-ping'
                  : isSelfReady
                    ? 'bg-amber-400 animate-pulse'
                    : 'bg-cyan-400 animate-pulse'
              }`}
            />
            {statusBadgeLabel}
          </div>
          <h2 className="text-xl font-bold text-white tracking-tight">Sector Engagement Locked</h2>
          <p
            className="text-xs text-slate-300 max-w-md mx-auto leading-relaxed"
            aria-live="polite"
          >
            {guidanceText}
          </p>
        </div>

        {/* Combatants Roster */}
        <div className="grid grid-cols-2 gap-3.5 mb-6" role="region" aria-label="Combatants Roster">
          {/* Host Commander Card */}
          <div
            className={`p-4 rounded-xl border transition-all duration-200 flex flex-col items-center text-center ${
              role === 'host'
                ? 'bg-slate-950/80 border-cyan-500/40 ring-1 ring-cyan-500/20'
                : 'bg-slate-950/60 border-slate-800'
            }`}
          >
            <div className="flex items-center gap-1.5 mb-1.5">
              <span className="text-[11px] font-mono uppercase text-cyan-400 font-semibold">
                Host
              </span>
              {role === 'host' && (
                <span className="px-1.5 py-0.2 rounded text-[10px] font-mono bg-cyan-950 border border-cyan-700/50 text-cyan-300">
                  YOU
                </span>
              )}
            </div>
            <span
              className="font-bold text-white text-base truncate max-w-full mb-3"
              title={hostName}
            >
              {hostName}
            </span>
            <div
              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono font-medium ${
                isHostReady
                  ? 'bg-emerald-950/70 border border-emerald-500/50 text-emerald-300 shadow-sm shadow-emerald-900/40'
                  : 'bg-slate-900 border border-slate-700/60 text-slate-400'
              }`}
            >
              {isHostReady ? (
                <>
                  <svg
                    className="w-3.5 h-3.5 text-emerald-400"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    aria-hidden="true"
                  >
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  <span>READY</span>
                </>
              ) : (
                <>
                  <span className="w-1.5 h-1.5 rounded-full bg-slate-500" />
                  <span>STANDBY</span>
                </>
              )}
            </div>
          </div>

          {/* Guest Commander Card */}
          <div
            className={`p-4 rounded-xl border transition-all duration-200 flex flex-col items-center text-center ${
              role === 'guest'
                ? 'bg-slate-950/80 border-cyan-500/40 ring-1 ring-cyan-500/20'
                : 'bg-slate-950/60 border-slate-800'
            }`}
          >
            <div className="flex items-center gap-1.5 mb-1.5">
              <span className="text-[11px] font-mono uppercase text-amber-400 font-semibold">
                Guest
              </span>
              {role === 'guest' && (
                <span className="px-1.5 py-0.2 rounded text-[10px] font-mono bg-cyan-950 border border-cyan-700/50 text-cyan-300">
                  YOU
                </span>
              )}
            </div>
            <span
              className="font-bold text-white text-base truncate max-w-full mb-3"
              title={guestName}
            >
              {guestName}
            </span>
            <div
              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono font-medium ${
                isGuestReady
                  ? 'bg-emerald-950/70 border border-emerald-500/50 text-emerald-300 shadow-sm shadow-emerald-900/40'
                  : 'bg-slate-900 border border-slate-700/60 text-slate-400'
              }`}
            >
              {isGuestReady ? (
                <>
                  <svg
                    className="w-3.5 h-3.5 text-emerald-400"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    aria-hidden="true"
                  >
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  <span>READY</span>
                </>
              ) : (
                <>
                  <span className="w-1.5 h-1.5 rounded-full bg-slate-500" />
                  <span>STANDBY</span>
                </>
              )}
            </div>
          </div>
        </div>

        {/* Action Controls */}
        <div className="space-y-3 pt-2">
          {/* Ready / Un-Ready Toggle Button */}
          <button
            type="button"
            onClick={toggleReady}
            disabled={isTogglingReady || isLeavingRoom}
            className={`w-full py-3.5 px-4 rounded-xl font-bold text-sm tracking-wide transition-all shadow-lg flex items-center justify-center gap-2 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-slate-900 disabled:opacity-50 disabled:cursor-not-allowed ${
              isSelfReady
                ? 'bg-amber-600/90 hover:bg-amber-500 text-white shadow-amber-950/40 focus:ring-amber-500 border border-amber-500/50'
                : 'bg-gradient-to-r from-emerald-600 to-cyan-600 hover:from-emerald-500 hover:to-cyan-500 text-white shadow-emerald-950/40 focus:ring-emerald-500 border border-emerald-400/30'
            }`}
          >
            {isTogglingReady ? (
              <>
                <svg
                  className="animate-spin -ml-1 mr-2 h-4 w-4 text-white"
                  fill="none"
                  viewBox="0 0 24 24"
                  aria-hidden="true"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
                <span>Synchronizing...</span>
              </>
            ) : isSelfReady ? (
              <>
                <svg
                  className="w-4 h-4 text-white"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
                <span>Cancel Ready</span>
              </>
            ) : (
              <>
                <svg
                  className="w-4 h-4 text-white"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
                <span>Ready for Battle</span>
              </>
            )}
          </button>

          {/* Leave Action Button */}
          <button
            type="button"
            onClick={leaveRoom}
            disabled={isLeavingRoom || isTogglingReady}
            className="w-full py-2.5 px-4 rounded-xl text-xs font-semibold text-slate-400 hover:text-rose-300 hover:bg-rose-950/20 border border-slate-800 hover:border-rose-900/40 transition-colors flex items-center justify-center gap-1.5 focus:outline-none focus:ring-1 focus:ring-rose-500/50 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isLeavingRoom ? (
              <span>Leaving sector...</span>
            ) : (
              <>
                <svg
                  className="w-3.5 h-3.5"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                  <polyline points="16 17 21 12 16 7" />
                  <line x1="21" y1="12" x2="9" y2="12" />
                </svg>
                <span>Leave Room</span>
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  )
}

