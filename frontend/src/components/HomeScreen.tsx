import React, { useState, useEffect, useRef } from 'react'
import { useSession } from '../context/SessionContext'
import { sanitizeDisplayName } from '../utils/sanitize'
import { ConnectionBadge } from './ConnectionBadge'
import { BrandLogo } from './BrandLogo'

export const HomeScreen: React.FC = () => {
  const {
    connectionStatus,
    player,
    savedDisplayName,
    errorMessage,
    isSessionConflict,
    isRestored,
    login,
    logout,
    clearConflictAndReset,
    retry,
  } = useSession()

  const [nameInput, setNameInput] = useState(() => savedDisplayName || '')
  const [localError, setLocalError] = useState<string | null>(null)
  const prevSavedNameRef = useRef(savedDisplayName)

  // Only sync input if savedDisplayName changes from outside (e.g. storage change or fresh login)
  useEffect(() => {
    if (savedDisplayName !== prevSavedNameRef.current) {
      prevSavedNameRef.current = savedDisplayName
      if (savedDisplayName) {
        setNameInput(savedDisplayName)
      }
    }
  }, [savedDisplayName])

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setNameInput(e.target.value)
    if (localError) {
      setLocalError(null)
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const validation = sanitizeDisplayName(nameInput)
    if (!validation.isValid) {
      setLocalError(validation.error || 'Please enter a valid display name')
      return
    }

    setLocalError(null)
    login(nameInput)
  }

  const currentRuneCount = Array.from(nameInput.trim()).length
  const isConnecting = connectionStatus === 'connecting'

  return (
    <div className="w-full max-w-md mx-auto">
      {/* Tactical Card */}
      <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-6 md:p-8 shadow-2xl border border-slate-800 text-slate-100">
        {/* Top Header & Status */}
        <div className="flex items-center justify-between pb-6 border-b border-slate-800/80 mb-6">
          <div className="flex items-center gap-3">
            <BrandLogo size="sm" />
            <div>
              <h1 className="text-xl font-bold tracking-tight text-white">Battleship</h1>
              <p className="text-xs text-slate-400">Naval Combat Command</p>
            </div>
          </div>
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        </div>

        {/* Tab Conflict Notice */}
        {isSessionConflict && (
          <div
            role="alert"
            className="mb-6 p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-sm"
          >
            <div className="font-semibold text-rose-200 mb-1 flex items-center gap-2">
              <svg
                className="w-4 h-4 text-rose-400"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
              </svg>
              Session Active in Another Tab
            </div>
            <p className="text-xs text-rose-300/90 mb-3 leading-relaxed">
              This session is already open in another browser window or tab. Only one active connection is permitted per session to prevent split-brain state.
            </p>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={clearConflictAndReset}
                className="px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium transition cursor-pointer"
              >
                Start Fresh Session
              </button>
              <button
                type="button"
                onClick={retry}
                className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition cursor-pointer"
              >
                Retry
              </button>
            </div>
          </div>
        )}

        {/* General Error Alert */}
        {!isSessionConflict && (errorMessage || localError) && (
          <div
            role="alert"
            className="mb-6 p-3 rounded-xl bg-amber-500/10 border border-amber-500/25 text-amber-300 text-xs flex items-start gap-2"
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
            <div className="flex-1">
              <span>{localError || errorMessage}</span>
              {connectionStatus === 'error' && (
                <button
                  type="button"
                  onClick={retry}
                  className="ml-2 underline hover:text-amber-200 cursor-pointer"
                >
                  Retry connection
                </button>
              )}
            </div>
          </div>
        )}

        {/* Content Section: Connected Session Card vs. Login Form */}
        {player && connectionStatus === 'connected' ? (
          <div className="space-y-6">
            {/* Identity Card */}
            <div className="p-4 rounded-xl bg-slate-800/60 border border-slate-700/60 space-y-3">
              <div className="flex items-center justify-between text-xs text-slate-400">
                <span>Commander Identity</span>
                <span className="font-mono text-cyan-400 text-[11px]">
                  ID: {player.playerId.slice(0, 8)}...
                </span>
              </div>
              <div className="text-2xl font-bold text-white tracking-wide break-words">
                {player.displayName}
              </div>

              {isRestored && (
                <div className="flex items-center gap-1.5 text-xs text-emerald-400/90 pt-1">
                  <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                    <path
                      fillRule="evenodd"
                      d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z"
                      clipRule="evenodd"
                    />
                  </svg>
                  <span>Session restored across refresh</span>
                </div>
              )}
            </div>

            {/* Actions */}
            <div className="space-y-3 pt-2">
              <div className="p-3 rounded-lg bg-cyan-950/30 border border-cyan-800/40 text-center text-xs text-cyan-300">
                Lobby matchmaking unlocks in <strong>Task T-04</strong>. Ready for fleet combat.
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={logout}
                  className="w-full py-2.5 px-4 rounded-xl bg-slate-800 hover:bg-slate-700/80 border border-slate-700 text-slate-300 text-sm font-medium transition cursor-pointer"
                >
                  Change Name / Disconnect
                </button>
              </div>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <div className="flex items-center justify-between mb-2">
                <label htmlFor="displayName" className="text-xs font-medium text-slate-300">
                  Callsign / Display Name
                </label>
                <span
                  className={`text-[11px] font-mono ${
                    currentRuneCount > 20
                      ? 'text-rose-400 font-bold'
                      : currentRuneCount > 0
                        ? 'text-cyan-400'
                        : 'text-slate-500'
                  }`}
                >
                  {currentRuneCount} / 20
                </span>
              </div>
              <div className="relative">
                <input
                  id="displayName"
                  type="text"
                  maxLength={25}
                  value={nameInput}
                  onChange={handleInputChange}
                  placeholder="e.g. Admiral Nimitz"
                  autoComplete="off"
                  disabled={isConnecting}
                  className="w-full px-4 py-3 rounded-xl bg-slate-950/70 border border-slate-700 focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500/20 text-white placeholder-slate-500 text-sm outline-none transition disabled:opacity-50"
                />
              </div>
              <p className="mt-1.5 text-[11px] text-slate-400">
                1–20 characters. Persists across browser refreshes.
              </p>
            </div>

            <button
              type="submit"
              disabled={isConnecting || currentRuneCount === 0 || currentRuneCount > 20}
              className="w-full py-3 px-4 rounded-xl bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 disabled:opacity-50 disabled:pointer-events-none text-white font-medium text-sm transition shadow-lg shadow-cyan-900/30 flex items-center justify-center gap-2 cursor-pointer"
            >
              {isConnecting ? (
                <>
                  <svg
                    className="animate-spin -ml-1 mr-2 h-4 w-4 text-white"
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
                    ></circle>
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  Establishing Link...
                </>
              ) : (
                'Play Battleship'
              )}
            </button>
          </form>
        )}
      </div>

      {/* Footer Tagline */}
      <footer className="mt-6 text-center text-xs text-slate-500">
        Server-authoritative naval combat over WebSockets
      </footer>
    </div>
  )
}

