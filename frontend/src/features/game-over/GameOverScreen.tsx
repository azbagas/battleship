import React from 'react'
import { useLobby } from '../../context/LobbyContext'
import { useSession } from '../../context/SessionContext'
import { ConnectionBadge } from '../../components/ConnectionBadge'
import { getRoomParticipants } from '../../utils/roomParticipants'

export const GameOverScreen: React.FC = () => {
  const {
    activeRoom,
    gameOverResult,
    ownSunkShips,
    opponentSunkShips,
    rematchStatus,
    rematchCancelReason,
    acceptRematch,
    declineRematch,
    leaveRoom,
    isLeavingRoom,
  } = useLobby()
  const { player, connectionStatus, isSessionConflict } = useSession()

  const { playerName, opponentName } = getRoomParticipants(activeRoom)
  const sectorId = activeRoom?.roomId ? activeRoom.roomId.slice(-6).toUpperCase() : 'SECTOR-1'

  const winnerPlayerId = gameOverResult?.winnerPlayerId ?? activeRoom?.winnerPlayerId
  const gameOverReason = gameOverResult?.reason || activeRoom?.endReason || 'all_sunk'

  const isAbandoned = gameOverReason === 'abandoned' || !winnerPlayerId
  const isWinner = !isAbandoned && winnerPlayerId === player?.playerId
  const winnerName = isAbandoned ? 'None (Abandoned)' : isWinner ? playerName : opponentName
  const totalShots = gameOverResult?.totalShots ?? 0

  const isNormalEnd = gameOverReason === 'all_sunk'

  return (
    <div
      role="main"
      aria-label="Match debrief and game over summary"
      className="w-full max-w-2xl mx-auto flex flex-col items-center justify-center p-3 sm:p-6"
    >
      <div className="w-full rounded-2xl bg-slate-900/90 border-2 border-slate-800/90 shadow-2xl backdrop-blur-md p-5 sm:p-8 space-y-6 text-center">
        {/* Sector and Connectivity Bar */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-3 text-xs font-mono">
          <div className="flex items-center gap-2 text-slate-400">
            <span className="text-cyan-400 font-bold">GRID SECTOR:</span>
            <span className="text-white font-bold">{sectorId}</span>
          </div>
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        </div>

        {/* Hero Outcome Banner */}
        <div
          className={`p-5 rounded-xl border ${
            isAbandoned
              ? 'bg-amber-950/40 border-amber-500/40 text-amber-300'
              : isWinner
              ? 'bg-emerald-950/50 border-emerald-500/50 text-emerald-300'
              : 'bg-rose-950/50 border-rose-500/50 text-rose-300'
          }`}
        >
          <div className="text-4xl sm:text-5xl mb-2">
            {isAbandoned ? '⚠️' : isWinner ? '🏆' : '💥'}
          </div>
          <h1
            id="gameover-title"
            className={`text-2xl sm:text-3xl font-mono font-black tracking-wider uppercase ${
              isAbandoned
                ? 'text-amber-400'
                : isWinner
                ? 'text-emerald-400'
                : 'text-rose-400'
            }`}
          >
            {isAbandoned
              ? 'ENGAGEMENT ABANDONED'
              : isWinner
              ? 'VICTORY ACHIEVED'
              : 'DEFEAT — FLEET LOST'}
          </h1>
          <p className="text-xs sm:text-sm font-mono mt-1 text-slate-300">
            {isAbandoned
              ? 'Both naval commanders disconnected from the sector before victory.'
              : isWinner
              ? 'Hostile fleet successfully eliminated.'
              : 'All friendly naval forces have been destroyed.'}
          </p>
        </div>

        {/* Tactical Debrief Statistics Grid */}
        <div className="grid grid-cols-2 gap-3 text-left font-mono text-xs">
          <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
            <span className="text-slate-400 block text-[10px] uppercase">Victor</span>
            <span className="text-white font-bold text-sm truncate block mt-0.5">
              {winnerName}
            </span>
          </div>
          <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
            <span className="text-slate-400 block text-[10px] uppercase">Outcome Reason</span>
            <span className="text-cyan-300 font-bold text-sm block mt-0.5 uppercase">
              {gameOverReason === 'all_sunk'
                ? 'All Ships Sunk'
                : gameOverReason === 'forfeit'
                ? isWinner
                  ? 'Enemy Forfeited'
                  : 'Surrendered'
                : gameOverReason === 'disconnect_timeout'
                ? isWinner
                  ? 'Opponent Disconnected'
                  : 'Connection Lost'
                : gameOverReason === 'abandoned'
                ? 'Both Disconnected'
                : gameOverReason}
            </span>
          </div>
          <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
            <span className="text-slate-400 block text-[10px] uppercase">Friendly Losses</span>
            <span className="text-rose-400 font-bold text-sm block mt-0.5">
              {ownSunkShips.length}/5 Sunk
            </span>
          </div>
          <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800">
            <span className="text-slate-400 block text-[10px] uppercase">Hostile Losses</span>
            <span className="text-emerald-400 font-bold text-sm block mt-0.5">
              {opponentSunkShips.length}/5 Sunk
            </span>
          </div>
          {totalShots > 0 && (
            <div className="col-span-2 p-2.5 rounded-lg bg-slate-950/70 border border-slate-800 flex justify-between items-center">
              <span className="text-slate-400 text-[10px] uppercase">Total Salvos Fired</span>
              <span className="text-cyan-400 font-bold">{totalShots}</span>
            </div>
          )}
        </div>

        {/* Rematch Section (Strictly available after normal match end) */}
        {isNormalEnd && (
          <div
            aria-live="polite"
            className="p-4 rounded-xl bg-slate-950/80 border border-slate-800/80 space-y-3"
          >
            <div className="flex items-center justify-between text-xs font-mono">
              <span className="text-slate-400 uppercase tracking-wider font-bold">
                Combat Rematch
              </span>
              <span className="text-[10px] text-cyan-400">Mutual Acceptance Required</span>
            </div>

            {rematchStatus === 'idle' && (
              <button
                type="button"
                onClick={acceptRematch}
                className="w-full py-3 px-4 rounded-xl bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 text-white font-mono font-bold uppercase tracking-wider text-sm shadow-lg shadow-cyan-950/40 transition-all focus:outline-none focus:ring-2 focus:ring-cyan-400"
              >
                Request Rematch
              </button>
            )}

            {rematchStatus === 'offered_by_me' && (
              <div className="p-3 rounded-lg bg-cyan-950/40 border border-cyan-500/40 flex items-center justify-center gap-2.5 text-cyan-300 font-mono text-xs">
                <span className="inline-block w-2.5 h-2.5 rounded-full bg-cyan-400 animate-ping" />
                <span>Rematch request sent. Awaiting opponent response...</span>
              </div>
            )}

            {rematchStatus === 'offered_by_opponent' && (
              <div className="space-y-2.5">
                <p className="text-xs font-mono text-amber-300 font-bold">
                  ⚔️ {opponentName} has requested a rematch!
                </p>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={acceptRematch}
                    className="flex-1 py-2.5 px-3 rounded-lg bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white font-mono font-bold text-xs uppercase transition-all focus:outline-none focus:ring-2 focus:ring-emerald-400"
                  >
                    Accept Rematch
                  </button>
                  <button
                    type="button"
                    onClick={declineRematch}
                    className="py-2.5 px-4 rounded-lg bg-slate-800 hover:bg-slate-700 active:bg-slate-900 text-slate-300 font-mono font-bold text-xs uppercase transition-all focus:outline-none focus:ring-2 focus:ring-slate-400"
                  >
                    Decline
                  </button>
                </div>
              </div>
            )}

            {rematchStatus === 'cancelled' && (
              <div className="p-2.5 rounded-lg bg-slate-900 border border-slate-700 text-slate-400 font-mono text-xs">
                {rematchCancelReason === 'declined'
                  ? 'Rematch offer was declined.'
                  : rematchCancelReason === 'disconnected'
                  ? 'Opponent disconnected. Rematch cancelled.'
                  : 'Opponent departed the room. Rematch cancelled.'}
              </div>
            )}
          </div>
        )}

        {/* Back to Lobby Action */}
        <div className="pt-2">
          <button
            type="button"
            onClick={leaveRoom}
            disabled={isLeavingRoom}
            className="w-full py-3.5 px-6 rounded-xl bg-slate-800 hover:bg-slate-700 active:bg-slate-900 text-white font-mono font-bold tracking-wider uppercase text-sm sm:text-base border border-slate-700 shadow-md transition-all duration-150 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-slate-400"
          >
            {isLeavingRoom ? 'Returning to Lobby...' : 'Return to Lobby'}
          </button>
        </div>
      </div>
    </div>
  )
}
export default GameOverScreen

