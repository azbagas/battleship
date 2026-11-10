import React, { useState, useEffect, useCallback, useMemo } from 'react'
import { useLobby } from '../context/LobbyContext'
import { useSession } from '../context/SessionContext'
import { ConnectionBadge } from './ConnectionBadge'
import { Coord, Orientation, PlacedShip, ShipType } from '../types/protocol'
import {
  COLUMN_LABELS,
  ROW_LABELS,
  FLEET_SHIPS,
  SHIP_SIZES,
  getShipCells,
  isShipInBounds,
  isValidPlacement,
  validateCompleteFleet,
  generateRandomFleet,
  coordToNotation,
} from '../utils/placement'

export const PlacementScreen: React.FC = () => {
  const {
    activeRoom,
    isFleetConfirmed,
    isOpponentConfirmed,
    ownFleet,
    placementDeadline,
    submitFleet,
    lobbyError,
    clearLobbyError,
  } = useLobby()
  const { connectionStatus, isSessionConflict } = useSession()

  // Local fleet placements state
  const [ships, setShips] = useState<PlacedShip[]>([])
  // Selected ship type (either from dock or placed on grid)
  const [selectedShipType, setSelectedShipType] = useState<ShipType | null>(null)
  // Current orientation for placing or rotating
  const [currentOrientation, setCurrentOrientation] = useState<Orientation>('horizontal')
  // Dragging state
  const [draggedShipType, setDraggedShipType] = useState<ShipType | null>(null)
  // Hovered cell for placement preview
  const [hoveredCoord, setHoveredCoord] = useState<Coord | null>(null)

  // Timer countdown
  const [secondsRemaining, setSecondsRemaining] = useState<number>(60)

  // Sync server ownFleet if updated (e.g. on auto-confirm or reconnect)
  useEffect(() => {
    if (ownFleet && ownFleet.length > 0) {
      setShips(ownFleet)
    }
  }, [ownFleet])

  // Countdown timer derived from placementDeadline
  useEffect(() => {
    if (!placementDeadline) return

    const targetTime = new Date(placementDeadline).getTime()

    const updateTimer = () => {
      const now = Date.now()
      const diffSec = Math.max(0, Math.ceil((targetTime - now) / 1000))
      setSecondsRemaining(diffSec)
    }

    updateTimer()
    const interval = setInterval(updateTimer, 500)
    return () => clearInterval(interval)
  }, [placementDeadline])

  // Player and opponent display details
  const hostName = activeRoom?.host?.name || 'Host Commander'
  const guestName = activeRoom?.guest?.name || activeRoom?.opponentName || 'Guest Commander'
  const role = activeRoom?.role || 'host'
  const playerName = role === 'host' ? hostName : guestName
  const opponentName = role === 'host' ? guestName : hostName

  // Map occupied cells for easy lookup
  const occupiedCellMap = useMemo(() => {
    const map = new Map<string, { ship: PlacedShip; index: number }>()
    ships.forEach((s) => {
      const size = SHIP_SIZES[s.type]
      const cells = getShipCells(s.origin, size, s.orientation)
      cells.forEach((c, idx) => {
        map.set(`${c.x},${c.y}`, { ship: s, index: idx })
      })
    })
    return map
  }, [ships])

  // Set of placed ship types
  const placedShipTypes = useMemo(() => new Set(ships.map((s) => s.type)), [ships])

  // Check if current fleet is complete and valid
  const fleetValidation = useMemo(() => validateCompleteFleet(ships), [ships])
  const isFleetComplete = fleetValidation.valid

  // Hover preview calculation
  const activeShipTypeForPreview = draggedShipType || selectedShipType
  const hoverPreview = useMemo(() => {
    if (!hoveredCoord || !activeShipTypeForPreview || isFleetConfirmed) {
      return null
    }
    const size = SHIP_SIZES[activeShipTypeForPreview]
    const inBounds = isShipInBounds(hoveredCoord, size, currentOrientation)
    const candidate: PlacedShip = {
      type: activeShipTypeForPreview,
      origin: hoveredCoord,
      orientation: currentOrientation,
    }
    const valid = isValidPlacement(candidate, ships)
    const cells = inBounds
      ? getShipCells(hoveredCoord, size, currentOrientation)
      : []

    return {
      cells,
      valid,
    }
  }, [hoveredCoord, activeShipTypeForPreview, currentOrientation, isFleetConfirmed, ships])

  // Set of preview cell keys
  const previewCellMap = useMemo(() => {
    if (!hoverPreview) return new Map<string, boolean>()
    const map = new Map<string, boolean>()
    hoverPreview.cells.forEach((c) => {
      map.set(`${c.x},${c.y}`, hoverPreview.valid)
    })
    return map
  }, [hoverPreview])

  // Rotate handler
  const handleRotate = useCallback(() => {
    if (isFleetConfirmed) return

    const nextOrientation: Orientation =
      currentOrientation === 'horizontal' ? 'vertical' : 'horizontal'
    setCurrentOrientation(nextOrientation)

    // If an existing placed ship is selected, try to rotate it in-place
    if (selectedShipType && placedShipTypes.has(selectedShipType)) {
      const existing = ships.find((s) => s.type === selectedShipType)
      if (existing) {
        const candidate: PlacedShip = {
          ...existing,
          orientation: nextOrientation,
        }
        if (isValidPlacement(candidate, ships)) {
          setShips((prev) =>
            prev.map((s) => (s.type === selectedShipType ? candidate : s)),
          )
        }
      }
    }
  }, [isFleetConfirmed, currentOrientation, selectedShipType, placedShipTypes, ships])

  // Keyboard shortcut: 'R' key rotates active orientation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (isFleetConfirmed) return
      if (e.key === 'r' || e.key === 'R') {
        e.preventDefault()
        handleRotate()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handleRotate, isFleetConfirmed])

  // Place ship at target coordinate
  const placeShipAt = useCallback(
    (type: ShipType, coord: Coord, orientation: Orientation) => {
      if (isFleetConfirmed) return
      const candidate: PlacedShip = {
        type,
        origin: coord,
        orientation,
      }
      if (!isValidPlacement(candidate, ships)) {
        return false
      }

      setShips((prev) => {
        const filtered = prev.filter((s) => s.type !== type)
        return [...filtered, candidate]
      })
      setSelectedShipType(null)
      setDraggedShipType(null)
      return true
    },
    [isFleetConfirmed, ships],
  )

  // Cell Click (tap-to-place or select placed ship)
  const handleCellClick = (x: number, y: number) => {
    if (isFleetConfirmed) return
    const cellKey = `${x},${y}`
    const occupied = occupiedCellMap.get(cellKey)

    // If a ship is selected from dock or moved: try to place it
    if (selectedShipType) {
      const placed = placeShipAt(selectedShipType, { x, y }, currentOrientation)
      if (placed) return
    }

    // If clicking an already placed ship on the grid, select it for rotation or repositioning
    if (occupied) {
      setSelectedShipType(occupied.ship.type)
      setCurrentOrientation(occupied.ship.orientation)
    } else {
      setSelectedShipType(null)
    }
  }

  // Remove ship back to dock
  const handleRemoveShip = (type: ShipType) => {
    if (isFleetConfirmed) return
    setShips((prev) => prev.filter((s) => s.type !== type))
    if (selectedShipType === type) {
      setSelectedShipType(null)
    }
  }

  // Randomize all 5 ships
  const handleRandomize = () => {
    if (isFleetConfirmed) return
    const randomFleet = generateRandomFleet()
    setShips(randomFleet)
    setSelectedShipType(null)
  }

  // Reset fleet (clear to dock)
  const handleReset = () => {
    if (isFleetConfirmed) return
    setShips([])
    setSelectedShipType(null)
  }

  // Confirm fleet
  const handleConfirm = () => {
    if (isFleetConfirmed || !isFleetComplete) return
    submitFleet(ships)
  }

  // Drag and drop handlers
  const handleDragStart = (type: ShipType, e: React.DragEvent) => {
    if (isFleetConfirmed) return
    setDraggedShipType(type)
    setSelectedShipType(type)
    e.dataTransfer.setData('text/plain', type)
    e.dataTransfer.effectAllowed = 'move'
  }

  const handleCellDragOver = (x: number, y: number, e: React.DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setHoveredCoord({ x, y })
  }

  const handleDrop = (x: number, y: number, e: React.DragEvent) => {
    e.preventDefault()
    const shipType = (e.dataTransfer.getData('text/plain') as ShipType) || draggedShipType
    if (shipType) {
      placeShipAt(shipType, { x, y }, currentOrientation)
    }
    setDraggedShipType(null)
    setHoveredCoord(null)
  }

  return (
    <div className="w-full max-w-5xl mx-auto px-2 sm:px-4 py-4 space-y-4">
      {/* Top Header Card */}
      <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-4 sm:p-6 shadow-2xl border border-slate-800 text-slate-100">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-4 border-b border-slate-800/80">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400 shadow-[0_0_15px_rgba(6,182,212,0.15)]">
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
                <circle cx="12" cy="12" r="10" />
                <line x1="22" y1="12" x2="18" y2="12" />
                <line x1="6" y1="12" x2="2" y2="12" />
                <line x1="12" y1="6" x2="12" y2="2" />
                <line x1="12" y1="22" x2="12" y2="18" />
              </svg>
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-xl font-bold tracking-tight text-white">Battleship Combat</h1>
                <span className="px-2 py-0.5 text-[10px] font-mono uppercase rounded bg-slate-800 text-slate-300 border border-slate-700">
                  Sector {activeRoom?.roomId.slice(-6).toUpperCase() || 'ALPHA'}
                </span>
              </div>
              <p className="text-xs text-slate-400">Tactical Deployment Phase</p>
            </div>
          </div>
          <ConnectionBadge status={connectionStatus} isConflict={isSessionConflict} />
        </div>

        {/* Command Status Banner */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mt-4 items-center">
          {/* Commander Info */}
          <div className="flex items-center gap-3 p-3 rounded-xl bg-slate-950/60 border border-slate-800/80">
            <div className="w-8 h-8 rounded-lg bg-cyan-950/80 border border-cyan-700/40 flex items-center justify-center text-cyan-400 font-bold text-xs uppercase">
              {role === 'host' ? 'H' : 'G'}
            </div>
            <div className="min-w-0">
              <span className="text-[10px] font-mono uppercase text-slate-400">Commander</span>
              <p className="text-sm font-semibold text-white truncate">{playerName}</p>
            </div>
          </div>

          {/* 60s Deployment Timer Banner */}
          <div className="p-3 rounded-xl bg-slate-950/60 border border-slate-800/80 text-center relative overflow-hidden">
            <div className="flex items-center justify-between text-xs font-mono mb-1">
              <span className="text-slate-400 uppercase text-[10px]">Launch Deadline</span>
              <span
                className={`font-bold transition-colors ${
                  secondsRemaining <= 10
                    ? 'text-rose-400 animate-pulse'
                    : secondsRemaining <= 20
                      ? 'text-amber-400'
                      : 'text-cyan-400'
                }`}
              >
                {secondsRemaining}s
              </span>
            </div>
            {/* Visual Progress Bar */}
            <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-500 ease-linear ${
                  secondsRemaining <= 10
                    ? 'bg-rose-500'
                    : secondsRemaining <= 20
                      ? 'bg-amber-500'
                      : 'bg-cyan-500'
                }`}
                style={{ width: `${Math.min(100, (secondsRemaining / 60) * 100)}%` }}
              />
            </div>
          </div>

          {/* Opponent Status */}
          <div className="flex items-center gap-3 p-3 rounded-xl bg-slate-950/60 border border-slate-800/80">
            <div
              className={`w-8 h-8 rounded-lg border flex items-center justify-center font-bold text-xs uppercase ${
                isOpponentConfirmed
                  ? 'bg-emerald-950/80 border-emerald-500/50 text-emerald-400'
                  : 'bg-slate-900 border-slate-700 text-slate-400'
              }`}
            >
              {isOpponentConfirmed ? '✓' : '...'}
            </div>
            <div className="min-w-0">
              <span className="text-[10px] font-mono uppercase text-slate-400 truncate block">
                Target: {opponentName}
              </span>
              <p
                className={`text-xs font-semibold truncate ${
                  isOpponentConfirmed ? 'text-emerald-400' : 'text-slate-400'
                }`}
              >
                {isOpponentConfirmed ? 'Fleet Confirmed' : 'Positioning Fleet...'}
              </p>
            </div>
          </div>
        </div>

        {/* Global Error Banner */}
        {lobbyError && (
          <div className="mt-4 p-3 rounded-xl bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs flex items-center justify-between">
            <span>{lobbyError}</span>
            <button
              onClick={clearLobbyError}
              className="text-rose-400 hover:text-white px-2 py-0.5 rounded text-[11px]"
            >
              Dismiss
            </button>
          </div>
        )}
      </div>

      {/* Main Grid & Dock Content */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4 items-start">
        {/* Ocean Grid Area (Left / Center) */}
        <div className="lg:col-span-7 bg-slate-900/90 backdrop-blur-md rounded-2xl p-4 sm:p-6 shadow-2xl border border-slate-800 flex flex-col items-center">
          <div className="w-full flex items-center justify-between mb-4">
            <h2 className="text-sm font-bold uppercase tracking-wider text-slate-300 flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse" />
              Ocean Grid (10×10)
            </h2>
            <span className="text-xs font-mono text-slate-400">
              {ships.length}/5 Ships Deployed
            </span>
          </div>

          {/* Tactical 10x10 Grid */}
          <div className="relative inline-block select-none overflow-x-auto max-w-full">
            {/* Column Headers A-J */}
            <div className="flex ml-7 mb-1">
              {COLUMN_LABELS.map((col) => (
                <div
                  key={col}
                  className="w-8 h-6 sm:w-10 sm:h-7 flex items-center justify-center text-[11px] sm:text-xs font-mono font-bold text-slate-400"
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
                  <div className="w-6 sm:w-6 text-right pr-2 text-[11px] sm:text-xs font-mono font-bold text-slate-400">
                    {row}
                  </div>

                  {/* Grid Cells */}
                  {COLUMN_LABELS.map((_, x) => {
                    const cellKey = `${x},${y}`
                    const occupied = occupiedCellMap.get(cellKey)
                    const isPreview = previewCellMap.has(cellKey)
                    const isPreviewValid = previewCellMap.get(cellKey)
                    const isSelected =
                      selectedShipType && occupied?.ship.type === selectedShipType

                    let cellBg = 'bg-slate-950/70 border-slate-800/80 hover:border-cyan-500/40'

                    if (occupied) {
                      cellBg = isSelected
                        ? 'bg-amber-500/20 border-amber-400 text-amber-200'
                        : 'bg-cyan-950/70 border-cyan-600/70 text-cyan-200'
                    }

                    if (isPreview) {
                      cellBg = isPreviewValid
                        ? 'bg-emerald-500/30 border-emerald-400 text-emerald-200'
                        : 'bg-rose-500/30 border-rose-500 text-rose-200'
                    }

                    return (
                      <button
                        key={cellKey}
                        type="button"
                        aria-label={`Cell ${COLUMN_LABELS[x]}${row}${
                          occupied ? `, ${occupied.ship.type}` : ''
                        }`}
                        onClick={() => handleCellClick(x, y)}
                        onMouseEnter={() => !isFleetConfirmed && setHoveredCoord({ x, y })}
                        onMouseLeave={() => setHoveredCoord(null)}
                        onDragOver={(e) => handleCellDragOver(x, y, e)}
                        onDrop={(e) => handleDrop(x, y, e)}
                        disabled={isFleetConfirmed}
                        className={`w-8 h-8 sm:w-10 sm:h-10 rounded border text-[10px] font-mono flex items-center justify-center transition-all duration-150 relative ${cellBg} ${
                          isFleetConfirmed ? 'cursor-not-allowed opacity-90' : 'cursor-pointer'
                        }`}
                      >
                        {occupied && (
                          <div className="w-full h-full flex items-center justify-center font-bold uppercase text-[9px] sm:text-[10px] tracking-tight">
                            {occupied.ship.type.slice(0, 1)}
                          </div>
                        )}
                      </button>
                    )
                  })}
                </div>
              ))}
            </div>
          </div>

          {/* Sub-grid Guidance Note */}
          <div className="mt-4 text-center text-xs text-slate-400">
            {isFleetConfirmed ? (
              <span className="text-cyan-400 font-semibold">
                Fleet confirmed and locked. Standing by for combat launch...
              </span>
            ) : selectedShipType ? (
              <span>
                Selected: <strong className="text-white uppercase">{selectedShipType}</strong>. Tap
                any grid cell to place, or press <kbd className="px-1 py-0.5 bg-slate-800 rounded text-cyan-300">R</kbd> to rotate.
              </span>
            ) : (
              <span>
                Click or drag ships from the Fleet Dock to deploy. Ships may touch but cannot overlap.
              </span>
            )}
          </div>
        </div>

        {/* Fleet Dock & Controls (Right Side) */}
        <div className="lg:col-span-5 space-y-4">
          {/* Controls Toolbar */}
          <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-4 sm:p-5 shadow-2xl border border-slate-800 space-y-3">
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300">
              Tactical Controls
            </h3>

            <div className="grid grid-cols-2 gap-2">
              {/* Rotate Button */}
              <button
                type="button"
                onClick={handleRotate}
                disabled={isFleetConfirmed}
                className="flex items-center justify-center gap-2 p-2.5 rounded-xl bg-slate-800/80 hover:bg-slate-700/80 border border-slate-700 text-white text-xs font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed"
                title="Rotate orientation (or press R key)"
              >
                <svg
                  className="w-4 h-4 text-cyan-400"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67" />
                </svg>
                <span>Rotate ({currentOrientation === 'horizontal' ? 'H' : 'V'})</span>
              </button>

              {/* Randomize Button */}
              <button
                type="button"
                onClick={handleRandomize}
                disabled={isFleetConfirmed}
                className="flex items-center justify-center gap-2 p-2.5 rounded-xl bg-slate-800/80 hover:bg-slate-700/80 border border-slate-700 text-white text-xs font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <svg
                  className="w-4 h-4 text-amber-400"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polyline points="16 3 21 3 21 8" />
                  <line x1="4" y1="20" x2="21" y2="3" />
                  <polyline points="21 16 21 21 16 21" />
                  <line x1="15" y1="15" x2="21" y2="21" />
                  <line x1="4" y1="4" x2="9" y2="9" />
                </svg>
                <span>Randomize</span>
              </button>
            </div>

            {/* Clear / Reset Fleet */}
            <button
              type="button"
              onClick={handleReset}
              disabled={isFleetConfirmed || ships.length === 0}
              className="w-full py-2 rounded-xl bg-slate-950/60 hover:bg-slate-900 border border-slate-800 text-slate-400 hover:text-slate-200 text-xs font-mono transition disabled:opacity-30 disabled:cursor-not-allowed"
            >
              Reset All Ships to Dock
            </button>

            {/* Confirm Fleet Action Button */}
            <div className="pt-2 border-t border-slate-800">
              <button
                type="button"
                onClick={handleConfirm}
                disabled={isFleetConfirmed || !isFleetComplete}
                className={`w-full py-3.5 px-4 rounded-xl font-bold text-sm tracking-wide transition-all shadow-lg flex items-center justify-center gap-2 ${
                  isFleetConfirmed
                    ? 'bg-emerald-600 text-white cursor-default shadow-emerald-950/50'
                    : isFleetComplete
                      ? 'bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-slate-950 shadow-cyan-500/20 active:scale-[0.99] cursor-pointer'
                      : 'bg-slate-800 text-slate-500 border border-slate-700/60 cursor-not-allowed'
                }`}
              >
                {isFleetConfirmed ? (
                  <>
                    <svg className="w-5 h-5 text-emerald-200" viewBox="0 0 20 20" fill="currentColor">
                      <path
                        fillRule="evenodd"
                        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                        clipRule="evenodd"
                      />
                    </svg>
                    <span>Fleet Confirmed</span>
                  </>
                ) : (
                  <>
                    <svg
                      className="w-4 h-4"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                    </svg>
                    <span>
                      {isFleetComplete ? 'Confirm Fleet' : `Place All Ships (${ships.length}/5)`}
                    </span>
                  </>
                )}
              </button>
            </div>
          </div>

          {/* Fleet Dock (Vessels List) */}
          <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-4 sm:p-5 shadow-2xl border border-slate-800 space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300">
                Fleet Dock
              </h3>
              <span className="text-[11px] font-mono text-slate-400">
                {FLEET_SHIPS.length - ships.length} In Reserve
              </span>
            </div>

            <div className="space-y-2">
              {FLEET_SHIPS.map((ship) => {
                const placed = ships.find((s) => s.type === ship.type)
                const isSelected = selectedShipType === ship.type

                return (
                  <div
                    key={ship.type}
                    draggable={!isFleetConfirmed}
                    onDragStart={(e) => handleDragStart(ship.type, e)}
                    onClick={() => {
                      if (isFleetConfirmed) return
                      if (placed) {
                        setSelectedShipType(isSelected ? null : ship.type)
                        setCurrentOrientation(placed.orientation)
                      } else {
                        setSelectedShipType(isSelected ? null : ship.type)
                      }
                    }}
                    className={`p-3 rounded-xl border transition flex items-center justify-between ${
                      isSelected
                        ? 'bg-cyan-950/70 border-cyan-500 shadow-[0_0_12px_rgba(6,182,212,0.2)]'
                        : placed
                          ? 'bg-slate-950/40 border-slate-800 text-slate-400'
                          : 'bg-slate-950/80 border-slate-700/60 hover:border-slate-600 text-slate-200 cursor-pointer'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-xs text-white">{ship.name}</span>
                          <span className="text-[10px] font-mono text-slate-400">
                            ({ship.size} cells)
                          </span>
                        </div>

                        {/* Ship segments preview bar */}
                        <div className="flex gap-1 mt-1.5">
                          {Array.from({ length: ship.size }).map((_, i) => (
                            <div
                              key={i}
                              className={`w-3.5 h-3.5 rounded-sm border ${
                                placed
                                  ? 'bg-cyan-600/50 border-cyan-500/70'
                                  : 'bg-slate-800 border-slate-700'
                              }`}
                            />
                          ))}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      {placed ? (
                        <div className="flex items-center gap-1.5">
                          <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-cyan-950/80 text-cyan-300 border border-cyan-800/60">
                            {coordToNotation(placed.origin)}{' '}
                            {placed.orientation === 'horizontal' ? 'H' : 'V'}
                          </span>
                          {!isFleetConfirmed && (
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation()
                                handleRemoveShip(ship.type)
                              }}
                              className="w-6 h-6 rounded flex items-center justify-center text-slate-400 hover:text-rose-400 hover:bg-rose-950/50 border border-transparent hover:border-rose-800 transition"
                              title="Return to dock"
                            >
                              ✕
                            </button>
                          )}
                        </div>
                      ) : (
                        <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-800 text-slate-300">
                          Ready
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
