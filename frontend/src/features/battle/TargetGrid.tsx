import React from 'react'
import { GridFrame } from '../../components/GridFrame'
import { GridShipOverlay } from '../../components/ShipSvg'
import { Coord, PlacedShip, ShipType, ShotResult } from '../../types/protocol'
import { COLUMN_LABELS, ROW_LABELS, coordToNotation } from '../../utils/placement'
import { SUNK_PATTERN_STYLE, targetCellAppearance } from './cellAppearance'

export interface TargetGridProps {
  targetShots: Map<string, ShotResult>
  sunkCellMap: Map<string, ShipType>
  revealedShips: PlacedShip[]
  focused: Coord
  onFocusCell: (coord: Coord) => void
  isMyTurn: boolean
  isMatchOver: boolean
  onCellClick: (x: number, y: number) => void
  visibleOnMobile: boolean
}

const SUNK_LEGEND_STYLE: React.CSSProperties = {
  backgroundImage:
    'repeating-linear-gradient(45deg, rgba(225, 29, 72, 0.4), rgba(225, 29, 72, 0.4) 2px, rgba(136, 19, 55, 0.85) 2px, rgba(136, 19, 55, 0.85) 4px)',
}

/** Hostile-waters radar: the only board the player fires on. */
export const TargetGrid: React.FC<TargetGridProps> = ({
  targetShots,
  sunkCellMap,
  revealedShips,
  focused,
  onFocusCell,
  isMyTurn,
  isMatchOver,
  onCellClick,
  visibleOnMobile,
}) => (
  <div
    className={`lg:col-span-7 p-2.5 sm:p-4 md:p-5 rounded-2xl bg-slate-900/70 border border-slate-800 shadow-xl backdrop-blur-sm ${
      visibleOnMobile ? 'block' : 'hidden lg:block'
    }`}
  >
    <div className="flex items-center justify-between mb-3">
      <div className="flex items-center gap-2">
        <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse" />
        <h3 className="text-sm font-mono font-bold uppercase tracking-wider text-white">
          Targeting Radar (Hostile Waters)
        </h3>
      </div>
      <span className="text-xs font-mono text-slate-400">{targetShots.size}/100 Targeted</span>
    </div>

    <GridFrame
      colSizeClass="md:w-9 md:h-7"
      rowSizeClass="md:h-9"
      labelColorClass="text-emerald-400/80"
      matrixClassName="border border-emerald-500/30 shadow-[inset_0_0_28px_rgba(16,185,129,0.12)]"
    >
      <GridShipOverlay ships={revealedShips} defaultVariant="sunk" />

      {ROW_LABELS.map((_, y) =>
        COLUMN_LABELS.map((_, x) => {
          const cellKey = `${x},${y}`
          const shotResult = targetShots.get(cellKey)
          const isFocused = focused.x === x && focused.y === y
          const { cellStyle, cellContent, ariaLabel, isSunk } = targetCellAppearance({
            notation: coordToNotation({ x, y }),
            shotResult,
            sunkShipType: sunkCellMap.get(cellKey),
            isMyTurn,
            isMatchOver,
            isFocused,
          })
          const focusRing = isFocused
            ? 'ring-2 ring-cyan-400 ring-offset-1 ring-offset-slate-950 z-20'
            : ''

          return (
            <button
              key={cellKey}
              type="button"
              tabIndex={isFocused ? 0 : -1}
              onFocus={() => onFocusCell({ x, y })}
              aria-label={ariaLabel}
              onClick={() => onCellClick(x, y)}
              disabled={Boolean(shotResult) || !isMyTurn || isMatchOver}
              style={isSunk ? SUNK_PATTERN_STYLE : undefined}
              className={`w-[26px] h-[26px] sm:w-8 sm:h-8 md:w-9 md:h-9 rounded border flex items-center justify-center font-mono transition-all duration-150 relative z-10 ${cellStyle} ${focusRing}`}
            >
              {cellContent}
            </button>
          )
        }),
      )}
    </GridFrame>

    <div className="mt-3 flex items-center justify-between text-xs text-slate-400 font-mono">
      <div className="flex items-center gap-3">
        <span className="inline-flex items-center gap-1">
          <span className="w-2.5 h-2.5 rounded-full border border-cyan-400 flex items-center justify-center text-[8px] text-cyan-300">
            ○
          </span>{' '}
          Miss
        </span>
        <span className="inline-flex items-center gap-1">
          <span className="w-2.5 h-2.5 rounded bg-rose-500/40 border border-rose-500 flex items-center justify-center text-[8px] text-rose-200">
            ✕
          </span>{' '}
          Hit
        </span>
        <span className="inline-flex items-center gap-1">
          <span
            className="w-3.5 h-2.5 rounded border border-rose-600 flex items-center justify-center text-[8px] text-rose-200"
            style={SUNK_LEGEND_STYLE}
          >
            ⚓
          </span>
          Sunk (striped)
        </span>
      </div>
      {isMyTurn && !isMatchOver && (
        <span className="text-emerald-400 font-semibold animate-pulse hidden sm:inline">
          Click grid or press Enter to fire salvo
        </span>
      )}
    </div>
  </div>
)
