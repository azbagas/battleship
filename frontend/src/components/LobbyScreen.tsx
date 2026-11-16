import React from 'react'
import { useLobby } from '../context/LobbyContext'
import { useSession } from '../context/SessionContext'
import { useElapsedTime } from '../hooks/useElapsedTime'
import { ConnectionBadge } from './ConnectionBadge'
import { BrandLogo } from './BrandLogo'
import { WaitingRoomView } from '../types/protocol'

interface RoomItemProps {
  room: WaitingRoomView
  onJoin: (roomId: string) => void
  isJoining: boolean
  isAnyJoining: boolean
}

const RoomItem: React.FC<RoomItemProps> = ({
  room,
  onJoin,
  isJoining,
  isAnyJoining,
}) => {
  const { formatted: waitDuration } = useElapsedTime(room.createdAt)

  return (
    <div
      data-testid={`room-item-${room.id}`}
      className="p-4 rounded-xl bg-slate-950/70 border border-slate-800/90 hover:border-slate-700 transition flex items-center justify-between gap-4"
    >
      <div className="flex items-center gap-3.5 min-w-0">
        <div className="w-10 h-10 rounded-lg bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400 shrink-0">
          {/* Ship / Commander Icon */}
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
            <path d="M2 21c.6.5 1.2 1 2.5 1 2.5 0 2.5-2 5-2 1.3 0 1.9.5 2.5 1 .6.5 1.2 1 2.5 1 2.5 0 2.5-2 5-2 1.3 0 1.9.5 2.5 1" />
            <path d="M19.38 20A11.6 11.6 0 0 0 21 14l-9-4-9 4c0 2.9.94 5.34 2.81 7.07" />
            <path d="M12 10V4" />
            <path d="M8 8h8" />
          </svg>
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className="font-bold text-white text-sm truncate"
              title={room.hostName}
            >
              {room.hostName}
            </span>
            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono bg-emerald-950/60 border border-emerald-700/50 text-emerald-300">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              OPEN
            </span>
          </div>
          <div className="flex items-center gap-2 mt-0.5 text-xs text-slate-400">
            <span>Wait time:</span>
            <span
              data-testid={`wait-duration-${room.id}`}
              className="font-mono text-cyan-400 font-medium"
            >
              {waitDuration}
            </span>
          </div>
        </div>
      </div>

      <button
        type="button"
        onClick={() => onJoin(room.id)}
        disabled={isAnyJoining}
        className="px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 disabled:opacity-50 text-white font-medium text-xs tracking-wide transition shadow-md shadow-cyan-950/40 cursor-pointer shrink-0 flex items-center gap-1.5"
      >
        {isJoining ? (
          <>
            <svg
              className="animate-spin -ml-0.5 mr-1 h-3.5 w-3.5 text-white"
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
            Joining...
          </>
        ) : (
          'Join'
        )}
      </button>
    </div>
  )
}

export const LobbyScreen: React.FC = () => {
  const {
    rooms,
    totalRooms,
    maxRooms,
    isRoomCapReached,
    createRoom,
    joinRoom,
    isCreating,
    joiningRoomId,
    lobbyError,
    clearLobbyError,
    serverRestartNotice,
    clearServerRestartNotice,
  } = useLobby()

  const { player, connectionStatus, isSessionConflict, logout } = useSession()

  return (
    <div className="w-full max-w-2xl mx-auto">
      <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-6 md:p-8 shadow-2xl border border-slate-800 text-slate-100">
        {/* Top Header & Session Bar */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-6 border-b border-slate-800/80 mb-6 gap-4">
          <div className="flex items-center gap-3">
            <BrandLogo size="md" />
            <div>
              <h1 className="text-xl font-bold tracking-tight text-white">Battleship</h1>
              <p className="text-xs text-slate-400">Public Battle Lobby</p>
            </div>
          </div>

          <div className="flex items-center justify-between sm:justify-end gap-3">
            <div className="flex flex-col sm:items-end text-left sm:text-right">
              <span className="text-xs font-semibold text-white truncate max-w-[150px]">
                {player?.displayName || 'Commander'}
              </span>
              <button
                type="button"
                onClick={logout}
                className="text-[11px] text-slate-400 hover:text-cyan-300 transition underline cursor-pointer"
              >
                Change Name
              </button>
            </div>
            <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
          </div>
        </div>

        {/* Server Restart / Lost Game Notice */}
        {serverRestartNotice && (
          <div
            role="alert"
            data-testid="server-restart-notice"
            className="mb-6 p-4 rounded-xl bg-amber-500/10 border border-amber-500/40 text-amber-200 text-xs flex items-start justify-between gap-3 shadow-lg shadow-amber-950/20"
          >
            <div className="flex items-start gap-2.5">
              <span className="text-amber-400 font-bold text-sm shrink-0 mt-0.5">⚠️</span>
              <div className="space-y-1">
                <span className="font-semibold text-amber-300 block">Previous Match Terminated</span>
                <span>{serverRestartNotice}</span>
              </div>
            </div>
            <button
              type="button"
              onClick={clearServerRestartNotice}
              aria-label="Dismiss server restart notification"
              className="text-amber-400 hover:text-amber-200 p-1 text-sm shrink-0 transition cursor-pointer"
            >
              ✕
            </button>
          </div>
        )}

        {/* Room Capacity Status */}
        <div className="mb-6 p-4 rounded-xl bg-slate-950/60 border border-slate-800/80">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs text-slate-400">Global Sector Capacity:</span>
            <span
              data-testid="capacity-count"
              className={`text-xs font-mono font-semibold ${
                isRoomCapReached ? 'text-amber-400' : 'text-cyan-400'
              }`}
            >
              {totalRooms} / {maxRooms} active rooms
            </span>
          </div>

          {/* Capacity Progress Bar */}
          <div className="w-full h-1.5 rounded-full bg-slate-800 overflow-hidden">
            <div
              className={`h-full transition-all duration-300 ${
                isRoomCapReached ? 'bg-amber-400' : 'bg-cyan-500'
              }`}
              style={{
                width: `${Math.min(100, Math.max(0, (totalRooms / maxRooms) * 100))}%`,
              }}
            />
          </div>

          {/* 10-Room Cap Notice */}
          {isRoomCapReached && (
            <div
              role="alert"
              className="mt-3 p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs flex items-start gap-2"
            >
              <svg
                className="w-4 h-4 text-amber-400 mt-0.5 shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
              <span>
                Maximum capacity reached (10 rooms). New rooms cannot be created until an active game completes. You may still join any available waiting room below.
              </span>
            </div>
          )}
        </div>

        {/* In-Lobby Error Alert */}
        {lobbyError && (
          <div
            role="alert"
            className="mb-6 p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center justify-between gap-3"
          >
            <div className="flex items-center gap-2">
              <svg
                className="w-4 h-4 text-rose-400 shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
              <span>{lobbyError}</span>
            </div>
            <button
              type="button"
              onClick={clearLobbyError}
              className="text-rose-400 hover:text-rose-200 text-xs font-semibold px-2 py-0.5 rounded cursor-pointer"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Section Header & Primary Action */}
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-semibold text-white tracking-wide uppercase">
              Waiting Rooms
            </h2>
            <span className="px-2 py-0.5 rounded-full bg-slate-800 text-[11px] font-mono text-slate-300">
              {rooms.length}
            </span>
          </div>

          <button
            type="button"
            onClick={createRoom}
            disabled={isCreating || isRoomCapReached}
            title={
              isRoomCapReached
                ? 'Maximum room capacity reached (10 rooms)'
                : 'Host a new public room'
            }
            className="px-4 py-2.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 disabled:opacity-50 disabled:hover:bg-cyan-600 disabled:cursor-not-allowed text-white font-medium text-xs transition shadow-lg shadow-cyan-950/40 flex items-center gap-2 cursor-pointer"
          >
            {isCreating ? (
              <>
                <svg
                  className="animate-spin -ml-1 mr-1 h-3.5 w-3.5 text-white"
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
                Creating Sector...
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
                >
                  <line x1="12" y1="5" x2="12" y2="19" />
                  <line x1="5" y1="12" x2="19" y2="12" />
                </svg>
                Create Game
              </>
            )}
          </button>
        </div>

        {/* Room List or Empty State */}
        {rooms.length === 0 ? (
          <div
            data-testid="lobby-empty-state"
            className="p-8 rounded-2xl bg-slate-950/40 border border-dashed border-slate-800 text-center flex flex-col items-center justify-center my-2"
          >
            <div className="w-14 h-14 rounded-2xl bg-slate-900 border border-slate-800 flex items-center justify-center text-slate-500 mb-3">
              <svg
                className="w-7 h-7 text-slate-500"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="22" y1="12" x2="18" y2="12" />
                <line x1="6" y1="12" x2="2" y2="12" />
                <line x1="12" y1="6" x2="12" y2="2" />
                <line x1="12" y1="22" x2="12" y2="18" />
              </svg>
            </div>
            <h3 className="text-base font-semibold text-white mb-1">
              No Active Waiting Rooms
            </h3>
            <p className="text-xs text-slate-400 max-w-sm mb-4 leading-relaxed">
              All sectors are currently quiet. Create a waiting room to host a match and invite an opponent to join.
            </p>
            <button
              type="button"
              onClick={createRoom}
              disabled={isCreating || isRoomCapReached}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-cyan-400 border border-slate-700 text-xs font-medium transition cursor-pointer disabled:opacity-50"
            >
              Create Waiting Room
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            {rooms.map((room) => (
              <RoomItem
                key={room.id}
                room={room}
                onJoin={joinRoom}
                isJoining={joiningRoomId === room.id}
                isAnyJoining={joiningRoomId !== null}
              />
            ))}
          </div>
        )}
      </div>

      {/* Footer Tagline */}
      <footer className="mt-6 text-center text-xs text-slate-500">
        Live matchmaking over WebSockets • 10-room global capacity
      </footer>
    </div>
  )
}

