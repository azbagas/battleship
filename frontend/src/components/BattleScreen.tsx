import React, { useState, useEffect, useMemo, useCallback } from 'react'
import { useLobby } from '../context/LobbyContext'
import { useSession } from '../context/SessionContext'
import { ConnectionBadge } from './ConnectionBadge'
import { PlacedShip, ShipType } from '../types/protocol'
import {
  COLUMN_LABELS,
  ROW_LABELS,
  FLEET_SHIPS,
  SHIP_SIZES,
  getShipCells,
  coordToNotation,
} from '../utils/placement'

export const BattleScreen: React.FC = () => {
  const {
    activeRoom,
    ownFleet,
    turnDeadline,
    isMyTurn,
    targetShots,
    ownShots,
    opponentSunkShips,
    ownSunkShips,
    recentEvents,
    gameOverResult,
    fireShot,
    leaveRoom,
    isLeavingRoom,
    lobbyError,
    clearLobbyError,
  } = useLobby()
  const { player, connectionStatus, isSessionConflict } = useSession()

  // Mobile active tab: 'target' | 'defense' | 'log'
  const [activeMobileTab, setActiveMobileTab] = useState<'target' | 'defense' | 'log'>('target')
  // Turn timer countdown in seconds
  const [secondsRemaining, setSecondsRemaining] = useState<number>(30)
  // Surrender confirmation modal
  const [showSurrenderConfirm, setShowSurrenderConfirm] = useState<boolean>(false)

  // Countdown timer derived from turnDeadline
  useEffect(() => {
    if (!turnDeadline || gameOverResult || activeRoom?.phase === 'finished') {
      return
    }

    const targetTime = new Date(turnDeadline).getTime()

    const updateTimer = () => {
      const now = Date.now()
      const diffSec = Math.max(0, Math.ceil((targetTime - now) / 1000))
      setSecondsRemaining(diffSec)
    }

    updateTimer()
    const interval = setInterval(updateTimer, 250)
    return () => clearInterval(interval)
  }, [turnDeadline, gameOverResult, activeRoom?.phase])

  // Automatically switch mobile tab to 'target' when it becomes user's turn
  useEffect(() => {
    if (isMyTurn) {
      setActiveMobileTab('target')
    }
  }, [isMyTurn])

  // Player and opponent display details
  const hostName = activeRoom?.host?.name || 'Host Commander'
  const guestName = activeRoom?.guest?.name || activeRoom?.opponentName || 'Guest Commander'
  const role = activeRoom?.role || 'host'
  const playerName = role === 'host' ? hostName : guestName
  const opponentName = role === 'host' ? guestName : hostName
  const sectorId = activeRoom?.roomId ? activeRoom.roomId.slice(-6).toUpperCase() : 'SECTOR-1'

  // Map own fleet cells for defensive grid lookup
  const ownOccupiedMap = useMemo(() => {
    const map = new Map<string, { ship: PlacedShip; index: number }>()
    ownFleet.forEach((s) => {
      const size = SHIP_SIZES[s.type]
      const cells = getShipCells(s.origin, size, s.orientation)
      cells.forEach((c, idx) => {
        map.set(`${c.x},${c.y}`, { ship: s, index: idx })
      })
    })
    return map
  }, [ownFleet])

  // Lookup map for opponent sunk ship cells
  const opponentSunkCellMap = useMemo(() => {
    const map = new Map<string, ShipType>()
    opponentSunkShips.forEach((sunk) => {
      sunk.cells.forEach((c) => {
        map.set(`${c.x},${c.y}`, sunk.type)
      })
    })
    return map
  }, [opponentSunkShips])

  // Lookup map for own sunk ship cells
  const ownSunkCellMap = useMemo(() => {
    const map = new Map<string, ShipType>()
    ownSunkShips.forEach((sunk) => {
      sunk.cells.forEach((c) => {
        map.set(`${c.x},${c.y}`, sunk.type)
      })
    })
    return map
  }, [ownSunkShips])

  // Opponent sunk ship types set
  const opponentSunkShipTypes = useMemo(() => {
    return new Set(opponentSunkShips.map((s) => s.type))
  }, [opponentSunkShips])

  // Own sunk ship types set
  const ownSunkShipTypes = useMemo(() => {
    return new Set(ownSunkShips.map((s) => s.type))
  }, [ownSunkShips])

  // Handle cell click on target radar
  const handleTargetCellClick = useCallback(
    (x: number, y: number) => {
      if (!isMyTurn || gameOverResult || activeRoom?.phase === 'finished') {
        return
      }
      const key = `${x},${y}`
      if (targetShots.has(key)) {
        return
      }
      fireShot({ x, y })
    },
    [isMyTurn, gameOverResult, activeRoom?.phase, targetShots, fireShot],
  )

  // Determine game over outcome
  const isMatchOver = Boolean(gameOverResult || activeRoom?.phase === 'finished')
  const isWinner = Boolean(
    gameOverResult
      ? gameOverResult.winnerPlayerId === player?.playerId
      : activeRoom?.winnerPlayerId === player?.playerId,
  )
  const gameOverReason = gameOverResult?.reason || activeRoom?.endReason || 'all_sunk'
  const winnerName = isWinner ? playerName : opponentName

  // Timer urgency style
  const isTimerUrgent = secondsRemaining <= 10 && !isMatchOver
  const timerPercentage = Math.min(100, Math.max(0, (secondsRemaining / 30) * 100))

  return (
    <div className="w-full max-w-6xl mx-auto flex flex-col gap-4">
      {/* Sector Header */}
      <header className="flex flex-wrap items-center justify-between gap-4 p-4 rounded-xl bg-slate-900/80 border border-slate-800 shadow-xl backdrop-blur-sm">
        <div className="flex items-center gap-3">
          <div className="w-3 h-3 rounded-full bg-cyan-400 animate-ping" />
          <div>
            <h1 className="text-xl sm:text-2xl font-mono font-black tracking-wider uppercase text-white flex items-center gap-2">
              Battleship Combat
              <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/30 text-cyan-300 font-mono">
                SEC-{sectorId}
              </span>
            </h1>
            <p className="text-xs text-slate-400 font-mono">
              Role: <strong className="text-cyan-300 uppercase">{role}</strong> | Commander{' '}
              <strong className="text-white">{playerName}</strong> vs{' '}
              <strong className="text-slate-300">{opponentName}</strong>
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
          {!isMatchOver && (
            <button
              type="button"
              onClick={() => setShowSurrenderConfirm(true)}
              className="text-xs font-mono text-rose-400 hover:text-rose-300 underline underline-offset-4 transition-colors"
            >
              Surrender
            </button>
          )}
        </div>
      </header>

      {/* Error Banner */}
      {lobbyError && (
        <div
          role="alert"
          className="flex items-center justify-between p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs sm:text-sm font-mono"
        >
          <div className="flex items-center gap-2">
            <span className="text-rose-400 font-bold">WARNING:</span>
            <span>{lobbyError}</span>
          </div>
          <button
            type="button"
            onClick={clearLobbyError}
            className="text-rose-400 hover:text-rose-200 ml-4 font-bold text-base leading-none"
            aria-label="Dismiss alert"
          >
            &times;
          </button>
        </div>
      )}

      {/* Turn Status Banner & 30s Urgency Countdown */}
      <section
        role="status"
        aria-live="polite"
        className={`p-4 rounded-xl border transition-all duration-300 relative overflow-hidden shadow-lg ${
          isMatchOver
            ? 'bg-slate-900/90 border-slate-700'
            : isMyTurn
            ? 'bg-gradient-to-r from-cyan-950/80 via-emerald-950/50 to-slate-900/80 border-emerald-500/50 shadow-emerald-900/20'
            : 'bg-gradient-to-r from-slate-900/90 via-amber-950/40 to-slate-900/90 border-amber-500/40 shadow-amber-950/20'
        }`}
      >
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 relative z-10">
          <div className="flex items-center gap-3">
            <div
              className={`w-10 h-10 rounded-lg flex items-center justify-center font-mono font-bold text-lg border ${
                isMatchOver
                  ? 'bg-slate-800 border-slate-700 text-slate-300'
                  : isMyTurn
                  ? 'bg-emerald-500/20 border-emerald-400 text-emerald-300 shadow-sm shadow-emerald-500/50 animate-pulse'
                  : 'bg-amber-500/10 border-amber-500/40 text-amber-400'
              }`}
            >
              {isMatchOver ? '🏁' : isMyTurn ? '🎯' : '🛡️'}
            </div>
            <div>
              <div className="text-xs uppercase font-mono tracking-widest text-slate-400">
                {isMatchOver
                  ? 'Engagement Concluded'
                  : isMyTurn
                  ? 'Command Priority'
                  : 'Hostile Maneuver'}
              </div>
              <h2 className="text-base sm:text-lg font-mono font-black uppercase text-white">
                {isMatchOver
                  ? 'Match Finished'
                  : isMyTurn
                  ? 'YOUR TURN — SELECT TARGET CELL'
                  : `${opponentName.toUpperCase()}'S TURN — BRACE FOR IMPACT`}
              </h2>
            </div>
          </div>

          {/* Turn Countdown Display */}
          {!isMatchOver && (
            <div className="flex items-center gap-3 self-end sm:self-center">
              <div className="text-right">
                <div className="text-[10px] uppercase font-mono tracking-wider text-slate-400">
                  {isTimerUrgent ? 'Auto-Shot Imminent' : 'Salvo Deadline'}
                </div>
                <div
                  className={`text-xl sm:text-2xl font-mono font-black ${
                    isTimerUrgent
                      ? 'text-rose-400 animate-pulse'
                      : isMyTurn
                      ? 'text-emerald-300'
                      : 'text-amber-300'
                  }`}
                >
                  {secondsRemaining}s
                </div>
              </div>
            </div>
          )}
        </div>

        {/* 30s Countdown Progress Bar */}
        {!isMatchOver && (
          <div className="w-full h-1.5 bg-slate-800/80 rounded-full mt-3 overflow-hidden">
            <div
              className={`h-full transition-all duration-300 ease-linear rounded-full ${
                isTimerUrgent
                  ? 'bg-rose-500'
                  : isMyTurn
                  ? 'bg-emerald-400'
                  : 'bg-amber-400'
              }`}
              style={{ width: `${timerPercentage}%` }}
            />
          </div>
        )}
      </section>

      {/* Fleet Status Summary Bars */}
      <section className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {/* Friendly Fleet (You) */}
        <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-cyan-400" />
              <span className="text-xs font-mono font-bold uppercase text-slate-200">
                Friendly Fleet ({playerName})
              </span>
            </div>
            <span className="text-xs font-mono text-cyan-300">
              {5 - ownSunkShips.length}/5 Vessels Operational
            </span>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-5 gap-2">
            {FLEET_SHIPS.map((ship) => {
              const isSunk = ownSunkShipTypes.has(ship.type)
              const placed = ownFleet.find((s) => s.type === ship.type)
              const cells = placed ? getShipCells(placed.origin, ship.size, placed.orientation) : []
              const hitsOnShip = cells.filter((c) => {
                const res = ownShots.get(`${c.x},${c.y}`)
                return res === 'hit' || res === 'sunk'
              }).length

              return (
                <div
                  key={ship.type}
                  className={`p-2 rounded border text-center transition-all ${
                    isSunk
                      ? 'bg-rose-950/40 border-rose-800/60 text-rose-400 opacity-60'
                      : 'bg-slate-950/60 border-slate-800 text-slate-300'
                  }`}
                >
                  <div className="text-[11px] font-mono font-bold truncate">{ship.name}</div>
                  <div className="flex items-center justify-center gap-1 mt-1">
                    {Array.from({ length: ship.size }).map((_, i) => (
                      <span
                        key={i}
                        className={`w-2 h-2 rounded-full ${
                          isSunk || i < hitsOnShip ? 'bg-rose-500' : 'bg-cyan-400'
                        }`}
                      />
                    ))}
                  </div>
                  <div className="text-[9px] font-mono mt-1 uppercase text-slate-400">
                    {isSunk ? 'SUNK' : `${ship.size - hitsOnShip}/${ship.size}`}
                  </div>
                </div>
              )
            })}
          </div>
        </div>

        {/* Hostile Fleet (Opponent) */}
        <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-rose-400" />
              <span className="text-xs font-mono font-bold uppercase text-slate-200">
                Hostile Fleet ({opponentName})
              </span>
            </div>
            <span className="text-xs font-mono text-rose-400">
              {5 - opponentSunkShips.length}/5 Vessels Unconfirmed
            </span>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-5 gap-2">
            {FLEET_SHIPS.map((ship) => {
              const isSunk = opponentSunkShipTypes.has(ship.type)

              return (
                <div
                  key={ship.type}
                  className={`p-2 rounded border text-center transition-all ${
                    isSunk
                      ? 'bg-rose-950/60 border-rose-600 text-rose-300'
                      : 'bg-slate-950/60 border-slate-800 text-slate-400'
                  }`}
                >
                  <div className="text-[11px] font-mono font-bold truncate">{ship.name}</div>
                  <div className="flex items-center justify-center gap-1 mt-1">
                    {Array.from({ length: ship.size }).map((_, i) => (
                      <span
                        key={i}
                        className={`w-2 h-2 rounded-full ${
                          isSunk ? 'bg-rose-500' : 'bg-slate-600'
                        }`}
                      />
                    ))}
                  </div>
                  <div className="text-[9px] font-mono mt-1 uppercase">
                    {isSunk ? (
                      <span className="text-rose-400 font-bold">SUNK</span>
                    ) : (
                      <span className="text-slate-500">ACTIVE</span>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      </section>

      {/* Mobile Tab Switcher (< 1024px) */}
      <div className="flex lg:hidden rounded-lg bg-slate-900 p-1 border border-slate-800 gap-1">
        <button
          type="button"
          onClick={() => setActiveMobileTab('target')}
          className={`flex-1 py-2 rounded text-xs font-mono font-bold uppercase transition-all ${
            activeMobileTab === 'target'
              ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30'
              : 'text-slate-400 hover:text-white'
          }`}
        >
          🎯 Target Radar
        </button>
        <button
          type="button"
          onClick={() => setActiveMobileTab('defense')}
          className={`flex-1 py-2 rounded text-xs font-mono font-bold uppercase transition-all ${
            activeMobileTab === 'defense'
              ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30'
              : 'text-slate-400 hover:text-white'
          }`}
        >
          🛡️ Own Fleet
        </button>
        <button
          type="button"
          onClick={() => setActiveMobileTab('log')}
          className={`flex-1 py-2 rounded text-xs font-mono font-bold uppercase transition-all ${
            activeMobileTab === 'log'
              ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30'
              : 'text-slate-400 hover:text-white'
          }`}
        >
          📜 Log ({recentEvents.length})
        </button>
      </div>

      {/* Combat Grid Arena */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        {/* Primary Target Radar Grid (Left on desktop) */}
        <div
          className={`lg:col-span-7 p-4 sm:p-5 rounded-2xl bg-slate-900/70 border border-slate-800 shadow-xl backdrop-blur-sm ${
            activeMobileTab !== 'target' ? 'hidden lg:block' : 'block'
          }`}
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse" />
              <h3 className="text-sm font-mono font-bold uppercase tracking-wider text-white">
                Targeting Radar (Hostile Waters)
              </h3>
            </div>
            <span className="text-xs font-mono text-slate-400">
              {targetShots.size}/100 Targeted
            </span>
          </div>

          {/* Radar Tactical Grid */}
          <div className="relative inline-block select-none overflow-x-auto max-w-full">
            {/* Column Headers A-J */}
            <div className="flex ml-6 sm:ml-7 mb-1">
              {COLUMN_LABELS.map((col) => (
                <div
                  key={col}
                  className="w-7 h-6 sm:w-9 sm:h-7 flex items-center justify-center text-[10px] sm:text-xs font-mono font-bold text-slate-400"
                >
                  {col}
                </div>
              ))}
            </div>

            {/* Grid Rows */}
            <div className="flex flex-col gap-1">
              {ROW_LABELS.map((row, y) => (
                <div key={row} className="flex items-center gap-1">
                  {/* Row Label 1-10 */}
                  <div className="w-5 sm:w-6 text-right pr-1 sm:pr-2 text-[10px] sm:text-xs font-mono font-bold text-slate-400">
                    {row}
                  </div>

                  {/* Grid Cells */}
                  {COLUMN_LABELS.map((_, x) => {
                    const cellKey = `${x},${y}`
                    const notation = coordToNotation({ x, y })
                    const shotResult = targetShots.get(cellKey)
                    const sunkShipType = opponentSunkCellMap.get(cellKey)

                    const isTargeted = Boolean(shotResult)
                    const isSunk = Boolean(sunkShipType || shotResult === 'sunk')
                    const isHit = shotResult === 'hit' || isSunk
                    const isMiss = shotResult === 'miss'

                    let cellStyle =
                      'bg-slate-950/70 border-slate-800/80 text-slate-500'
                    let cellContent: React.ReactNode = null
                    let ariaLabel = `Target cell ${notation}`

                    if (isSunk) {
                      cellStyle =
                        'bg-rose-950/90 border-rose-600 text-rose-300 font-bold shadow-inner shadow-rose-900/50'
                      cellContent = (
                        <div className="flex flex-col items-center justify-center">
                          <span className="text-[11px] sm:text-xs leading-none">✕</span>
                          <span className="text-[7px] font-mono leading-none uppercase mt-0.5">
                            {sunkShipType ? sunkShipType.slice(0, 1) : ''}
                          </span>
                        </div>
                      )
                      ariaLabel = `Target cell ${notation}, SUNK ${sunkShipType || 'ship'}`
                    } else if (isHit) {
                      cellStyle =
                        'bg-rose-600/30 border-rose-500 text-rose-300 font-black animate-pulse'
                      cellContent = <span className="text-xs sm:text-sm leading-none">✕</span>
                      ariaLabel = `Target cell ${notation}, HIT`
                    } else if (isMiss) {
                      cellStyle =
                        'bg-slate-900/90 border-slate-800 text-cyan-400/90 font-bold'
                      cellContent = <span className="text-xs sm:text-sm leading-none">•</span>
                      ariaLabel = `Target cell ${notation}, MISS`
                    } else {
                      // Untargeted
                      if (isMyTurn && !isMatchOver) {
                        cellStyle =
                          'bg-slate-950/70 border-slate-800 hover:border-cyan-400 hover:bg-cyan-500/20 text-cyan-300 cursor-crosshair active:scale-95'
                        ariaLabel = `Target cell ${notation}, ready to fire`
                      } else {
                        cellStyle =
                          'bg-slate-950/40 border-slate-900 text-slate-700 cursor-not-allowed opacity-75'
                        ariaLabel = `Target cell ${notation}, stand by`
                      }
                    }

                    return (
                      <button
                        key={cellKey}
                        type="button"
                        aria-label={ariaLabel}
                        onClick={() => handleTargetCellClick(x, y)}
                        disabled={isTargeted || !isMyTurn || isMatchOver}
                        className={`w-7 h-7 sm:w-9 sm:h-9 rounded border flex items-center justify-center font-mono transition-all duration-150 relative ${cellStyle}`}
                      >
                        {cellContent}
                      </button>
                    )
                  })}
                </div>
              ))}
            </div>
          </div>

          <div className="mt-3 flex items-center justify-between text-xs text-slate-400 font-mono">
            <div className="flex items-center gap-3">
              <span className="inline-flex items-center gap-1">
                <span className="w-2 h-2 rounded-full bg-cyan-400" /> • Miss
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="w-2 h-2 rounded-full bg-rose-500" /> ✕ Hit
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="w-2 h-2 rounded bg-rose-900 border border-rose-600" /> Sunk
              </span>
            </div>
            {isMyTurn && !isMatchOver && (
              <span className="text-emerald-400 font-semibold animate-pulse hidden sm:inline">
                Click grid to launch salvo
              </span>
            )}
          </div>
        </div>

        {/* Right Column: Defensive Grid & Combat Feed */}
        <div className="lg:col-span-5 flex flex-col gap-4">
          {/* Defensive Fleet Grid */}
          <div
            className={`p-4 sm:p-5 rounded-2xl bg-slate-900/70 border border-slate-800 shadow-xl backdrop-blur-sm ${
              activeMobileTab !== 'defense' ? 'hidden lg:block' : 'block'
            }`}
          >
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full bg-cyan-400" />
                <h3 className="text-sm font-mono font-bold uppercase tracking-wider text-white">
                  Defensive Grid (Your Fleet)
                </h3>
              </div>
              <span className="text-xs font-mono text-slate-400">
                {ownShots.size} Enemy Salvos
              </span>
            </div>

            {/* Defensive Tactical Grid */}
            <div className="relative inline-block select-none overflow-x-auto max-w-full">
              {/* Column Headers A-J */}
              <div className="flex ml-6 sm:ml-7 mb-1">
                {COLUMN_LABELS.map((col) => (
                  <div
                    key={col}
                    className="w-7 h-6 sm:w-8 sm:h-7 flex items-center justify-center text-[10px] sm:text-xs font-mono font-bold text-slate-400"
                  >
                    {col}
                  </div>
                ))}
              </div>

              {/* Grid Rows */}
              <div className="flex flex-col gap-1">
                {ROW_LABELS.map((row, y) => (
                  <div key={row} className="flex items-center gap-1">
                    {/* Row Label 1-10 */}
                    <div className="w-5 sm:w-6 text-right pr-1 sm:pr-2 text-[10px] sm:text-xs font-mono font-bold text-slate-400">
                      {row}
                    </div>

                    {/* Grid Cells */}
                    {COLUMN_LABELS.map((_, x) => {
                      const cellKey = `${x},${y}`
                      const notation = coordToNotation({ x, y })
                      const occupied = ownOccupiedMap.get(cellKey)
                      const shotResult = ownShots.get(cellKey)
                      const isSunk = Boolean(ownSunkCellMap.get(cellKey))

                      let cellStyle = 'bg-slate-950/60 border-slate-850'
                      let cellContent: React.ReactNode = null
                      let ariaLabel = `Defensive cell ${notation}`

                      if (occupied) {
                        if (isSunk) {
                          cellStyle =
                            'bg-rose-950/90 border-rose-600 text-rose-300 font-bold'
                          cellContent = (
                            <div className="flex flex-col items-center justify-center leading-none">
                              <span className="text-[10px]">✕</span>
                              <span className="text-[7px] uppercase font-mono">
                                {occupied.ship.type.slice(0, 1)}
                              </span>
                            </div>
                          )
                          ariaLabel = `Defensive cell ${notation}, SUNK ${occupied.ship.type}`
                        } else if (shotResult === 'hit') {
                          cellStyle =
                            'bg-rose-600/40 border-rose-500 text-rose-200 font-black animate-pulse'
                          cellContent = <span className="text-xs leading-none">✕</span>
                          ariaLabel = `Defensive cell ${notation}, HIT on ${occupied.ship.type}`
                        } else {
                          cellStyle =
                            'bg-cyan-950/70 border-cyan-600/70 text-cyan-200 font-mono font-bold'
                          cellContent = (
                            <span className="text-[9px] uppercase tracking-tighter">
                              {occupied.ship.type.slice(0, 1)}
                            </span>
                          )
                          ariaLabel = `Defensive cell ${notation}, ${occupied.ship.type}`
                        }
                      } else {
                        // Empty water cell
                        if (shotResult === 'miss') {
                          cellStyle = 'bg-slate-900 border-slate-800 text-slate-400'
                          cellContent = <span className="text-xs leading-none">•</span>
                          ariaLabel = `Defensive cell ${notation}, enemy miss`
                        } else {
                          cellStyle = 'bg-slate-950/40 border-slate-900 text-slate-700'
                        }
                      }

                      return (
                        <div
                          key={cellKey}
                          aria-label={ariaLabel}
                          className={`w-7 h-7 sm:w-8 sm:h-8 rounded border flex items-center justify-center font-mono ${cellStyle}`}
                        >
                          {cellContent}
                        </div>
                      )
                    })}
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Combat Events Log */}
          <div
            className={`p-4 sm:p-5 rounded-2xl bg-slate-900/70 border border-slate-800 shadow-xl backdrop-blur-sm ${
              activeMobileTab !== 'log' ? 'hidden lg:block' : 'block'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-mono font-bold uppercase tracking-wider text-white flex items-center gap-2">
                <span>📜</span> Tactical Combat Log
              </h3>
              <span className="text-[10px] font-mono text-slate-400">
                Latest Salvos
              </span>
            </div>

            <div
              tabIndex={0}
              aria-label="Combat event log"
              className="max-h-52 overflow-y-auto space-y-1.5 pr-1 font-mono text-xs focus:outline-none focus:ring-1 focus:ring-cyan-500/40 rounded"
            >
              {recentEvents.length === 0 ? (
                <div className="text-slate-500 italic py-4 text-center">
                  Engagement in progress. Waiting for opening salvos...
                </div>
              ) : (
                recentEvents.map((evt, idx) => {
                  const isMe = evt.by === player?.playerId
                  const shooterName = isMe ? playerName : opponentName
                  const notCoord = coordToNotation(evt.cell)

                  let badgeColor = 'text-cyan-400 bg-cyan-500/10 border-cyan-500/30'
                  let outcomeText = 'MISS'

                  if (evt.result === 'sunk') {
                    badgeColor = 'text-rose-300 bg-rose-500/20 border-rose-500/50'
                    outcomeText = `SUNK ${evt.sunkShip?.type.toUpperCase()}!`
                  } else if (evt.result === 'hit') {
                    badgeColor = 'text-amber-300 bg-amber-500/20 border-amber-500/50'
                    outcomeText = 'DIRECT HIT'
                  }

                  return (
                    <div
                      key={idx}
                      className="flex items-center justify-between p-1.5 rounded bg-slate-950/60 border border-slate-800/80 gap-2"
                    >
                      <div className="flex items-center gap-1.5 truncate">
                        <span className="text-slate-400 font-bold truncate">
                          {shooterName}:
                        </span>
                        <span className="text-white font-bold">{notCoord}</span>
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
        </div>
      </div>

      {/* Surrender Confirmation Modal */}
      {showSurrenderConfirm && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="surrender-title"
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm"
        >
          <div className="w-full max-w-md p-6 rounded-2xl bg-slate-900 border border-slate-800 shadow-2xl text-center space-y-4">
            <div className="w-12 h-12 mx-auto rounded-full bg-rose-500/20 border border-rose-500/40 flex items-center justify-center text-rose-400 text-xl font-bold">
              ⚠️
            </div>
            <h3 id="surrender-title" className="text-lg font-mono font-bold uppercase text-white">
              Surrender Engagement?
            </h3>
            <p className="text-xs text-slate-300 font-mono">
              Conceding will forfeit the match immediately and award the victory to{' '}
              <strong className="text-white">{opponentName}</strong>. This cannot be undone.
            </p>
            <div className="flex items-center justify-center gap-3 pt-2">
              <button
                type="button"
                onClick={() => setShowSurrenderConfirm(false)}
                className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-mono font-bold transition-all"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={isLeavingRoom}
                onClick={() => {
                  setShowSurrenderConfirm(false)
                  leaveRoom()
                }}
                className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-mono font-bold transition-all disabled:opacity-50"
              >
                {isLeavingRoom ? 'Surrendering...' : 'Confirm Forfeit'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Game Over Modal / Overlay */}
      {isMatchOver && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="gameover-title"
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/85 backdrop-blur-md animate-fade-in"
        >
          <div className="w-full max-w-lg p-6 sm:p-8 rounded-2xl bg-slate-900 border-2 shadow-2xl text-center space-y-6 relative overflow-hidden">
            {/* Header / Outcome Banner */}
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

            {/* Match Statistics */}
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
                    ? 'Enemy Forfeited'
                    : gameOverReason === 'disconnect_timeout'
                    ? 'Connection Lost'
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
                <span className="text-slate-400 block text-[10px] uppercase">Hostile Vessels Sunk</span>
                <span className="text-emerald-400 font-bold text-sm block mt-0.5">
                  {opponentSunkShips.length}/5 Sunk
                </span>
              </div>
            </div>

            {/* Back to Lobby Action */}
            <div className="pt-2">
              <button
                type="button"
                onClick={leaveRoom}
                disabled={isLeavingRoom}
                className="w-full py-3.5 px-6 rounded-xl bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 text-white font-mono font-bold tracking-wider uppercase text-sm sm:text-base shadow-lg shadow-cyan-900/40 transition-all duration-150 disabled:opacity-50"
              >
                {isLeavingRoom ? 'Returning to Lobby...' : 'Return to Lobby'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
