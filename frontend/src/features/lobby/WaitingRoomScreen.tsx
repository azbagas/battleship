import React from 'react'
import { useLobby } from '../../context/LobbyContext'
import { useSession } from '../../context/SessionContext'
import { useElapsedTime } from '../../hooks/useElapsedTime'
import { ConnectionBadge } from '../../components/ConnectionBadge'
import { BrandLogo } from '../../components/BrandLogo'

export const WaitingRoomScreen: React.FC = () => {
  const { activeRoom, cancelRoom, isCancelling } = useLobby()
  const { connectionStatus, isSessionConflict } = useSession()

  const hostName = activeRoom?.host?.name || 'Commander'
  const roomId = activeRoom?.roomId || ''
  const { formatted: waitDuration } = useElapsedTime(activeRoom?.createdAt)

  return (
    <div className="w-full max-w-lg mx-auto">
      <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-6 md:p-8 shadow-2xl border border-slate-800 text-slate-100">
        {/* Header Bar */}
        <div className="flex items-center justify-between pb-6 border-b border-slate-800/80 mb-6">
          <div className="flex items-center gap-3">
            <BrandLogo size="sm" />
            <div>
              <h1 className="text-xl font-bold tracking-tight text-white">Battleship</h1>
              <p className="text-xs text-slate-400">Host Waiting Room</p>
            </div>
          </div>
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        </div>

        {/* Sonar / Radar Centerpiece */}
        <div className="flex flex-col items-center justify-center py-6">
          <div className="relative w-44 h-44 rounded-full border border-cyan-500/30 bg-slate-950/80 flex items-center justify-center overflow-hidden shadow-inner shadow-cyan-950/50">
            {/* Concentric rings */}
            <div className="absolute inset-4 rounded-full border border-cyan-500/20 pointer-events-none" />
            <div className="absolute inset-10 rounded-full border border-cyan-500/15 pointer-events-none" />
            <div className="absolute inset-16 rounded-full border border-cyan-500/10 pointer-events-none" />

            {/* Crosshairs */}
            <div className="absolute inset-x-0 top-1/2 h-px bg-cyan-500/20 pointer-events-none" />
            <div className="absolute inset-y-0 left-1/2 w-px bg-cyan-500/20 pointer-events-none" />

            {/* Rotating sonar sweep */}
            <div
              className="absolute inset-0 rounded-full animate-spin pointer-events-none"
              style={{
                animationDuration: '4s',
                background:
                  'conic-gradient(from 0deg at 50% 50%, rgba(6, 182, 212, 0) 0deg, rgba(6, 182, 212, 0.25) 60deg, rgba(6, 182, 212, 0) 65deg)',
              }}
              aria-hidden="true"
            />

            {/* Central radar blip */}
            <div className="relative z-10 flex flex-col items-center">
              <span className="relative flex h-3.5 w-3.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-3.5 w-3.5 bg-cyan-500 shadow-sm shadow-cyan-400" />
              </span>
            </div>
          </div>

          {/* Radar Status Text */}
          <div className="mt-5 text-center space-y-1">
            <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-cyan-950/50 border border-cyan-800/40 text-[11px] text-cyan-300 font-mono tracking-wider uppercase">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              Sector Open • Transmitting
            </div>
            <h2 className="text-lg font-bold text-white pt-1">Waiting for an opponent</h2>
            <p className="text-xs text-slate-400 max-w-xs mx-auto">
              Your room is live in the public lobby. Battle commences as soon as an opponent accepts your challenge.
            </p>
          </div>
        </div>

        {/* Room & Host Telemetry */}
        <div className="p-4 rounded-xl bg-slate-950/60 border border-slate-800/80 mb-6 space-y-3">
          <div className="flex items-center justify-between text-xs">
            <span className="text-slate-400">Host Commander:</span>
            <span className="font-semibold text-white">{hostName}</span>
          </div>
          <div className="flex items-center justify-between text-xs">
            <span className="text-slate-400">Wait Duration:</span>
            <span
              data-testid="host-wait-duration"
              className="font-mono text-cyan-400 font-semibold text-sm"
            >
              {waitDuration}
            </span>
          </div>
          {roomId && (
            <div className="flex items-center justify-between text-xs">
              <span className="text-slate-400">Sector ID:</span>
              <span className="font-mono text-slate-400 text-[11px]">{roomId}</span>
            </div>
          )}
        </div>

        {/* Cancel Action */}
        <div className="space-y-3">
          <button
            type="button"
            onClick={cancelRoom}
            disabled={isCancelling}
            className="w-full py-3 px-4 rounded-xl bg-slate-800 hover:bg-rose-950/40 hover:text-rose-200 hover:border-rose-800/50 border border-slate-700 text-slate-300 text-sm font-medium transition cursor-pointer disabled:opacity-50 flex items-center justify-center gap-2"
          >
            {isCancelling ? (
              <>
                <svg
                  className="animate-spin -ml-1 mr-2 h-4 w-4 text-slate-300"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
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
                Cancelling Room...
              </>
            ) : (
              'Cancel Game & Return to Lobby'
            )}
          </button>
        </div>
      </div>
    </div>
  )
}

