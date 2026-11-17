import React from 'react'
import { ShipType, ShotResult } from '../../types/protocol'
import { OccupiedCell } from './useBattleView'

export interface CellAppearance {
  cellStyle: string
  cellContent: React.ReactNode
  ariaLabel: string
}

/** Diagonal stripe overlay applied to cells of sunk ships. */
export const SUNK_PATTERN_STYLE: React.CSSProperties = {
  backgroundImage:
    'repeating-linear-gradient(45deg, rgba(225, 29, 72, 0.25), rgba(225, 29, 72, 0.25) 3px, rgba(136, 19, 55, 0.55) 3px, rgba(136, 19, 55, 0.55) 6px)',
}

export interface TargetCellInput {
  notation: string
  shotResult: ShotResult | undefined
  sunkShipType: ShipType | undefined
  isMyTurn: boolean
  isMatchOver: boolean
  isFocused: boolean
}

/** Visual and accessible state of one cell on the hostile target radar. */
export function targetCellAppearance({
  notation,
  shotResult,
  sunkShipType,
  isMyTurn,
  isMatchOver,
  isFocused,
}: TargetCellInput): CellAppearance & { isSunk: boolean } {
  const isSunk = Boolean(sunkShipType || shotResult === 'sunk')
  const isHit = shotResult === 'hit' || isSunk
  const isMiss = shotResult === 'miss'

  if (isSunk) {
    return {
      isSunk,
      cellStyle: 'bg-rose-950/45 border-rose-500/80 text-rose-100 font-bold shadow-inner',
      cellContent: (
        <span className="text-xs sm:text-sm leading-none drop-shadow-[0_1px_2px_rgba(0,0,0,0.9)]">
          ⚓
        </span>
      ),
      ariaLabel: `Target cell ${notation}, SUNK ${sunkShipType || 'vessel'} (diagonal striped)`,
    }
  }
  if (isHit) {
    return {
      isSunk,
      cellStyle:
        'bg-rose-600/35 border-rose-500 text-rose-100 font-black shadow-[inset_0_0_10px_rgba(244,63,94,0.45)]',
      cellContent: (
        <span className="relative flex items-center justify-center w-full h-full">
          <span className="absolute w-4 h-4 sm:w-5 sm:h-5 rounded-full border border-rose-400/70 animate-ping opacity-40" />
          <span className="text-xs sm:text-sm leading-none drop-shadow-[0_0_4px_rgba(244,63,94,0.9)]">
            ✕
          </span>
        </span>
      ),
      ariaLabel: `Target cell ${notation}, HIT (vessel damaged)`,
    }
  }
  if (isMiss) {
    return {
      isSunk,
      cellStyle: 'bg-slate-900/80 border-slate-700/80 text-cyan-300 font-bold',
      cellContent: (
        <span className="relative flex items-center justify-center w-full h-full">
          <span className="absolute w-3.5 h-3.5 sm:w-4 sm:h-4 rounded-full border border-cyan-500/30" />
          <span className="text-xs sm:text-sm leading-none">○</span>
        </span>
      ),
      ariaLabel: `Target cell ${notation}, MISS (water splash)`,
    }
  }
  if (isMyTurn && !isMatchOver) {
    return {
      isSunk,
      cellStyle:
        'bg-slate-950/55 border-slate-800/80 hover:border-emerald-400 hover:bg-emerald-500/15 text-emerald-300 cursor-crosshair active:scale-95',
      cellContent: null,
      ariaLabel: `Target cell ${notation}, ready to fire${isFocused ? ' (press Enter or Space to fire)' : ''}`,
    }
  }
  return {
    isSunk,
    cellStyle: 'bg-slate-950/40 border-slate-900 text-slate-700 cursor-not-allowed opacity-75',
    cellContent: null,
    ariaLabel: `Target cell ${notation}, stand by`,
  }
}

export interface DefenseCellInput {
  notation: string
  occupied: OccupiedCell | undefined
  shotResult: ShotResult | undefined
  isSunk: boolean
}

/** Visual and accessible state of one cell on the player's own defensive grid. */
export function defenseCellAppearance({
  notation,
  occupied,
  shotResult,
  isSunk,
}: DefenseCellInput): CellAppearance {
  if (occupied) {
    const type = occupied.ship.type
    if (isSunk) {
      return {
        cellStyle: 'bg-rose-950/45 border-rose-500/80 text-rose-100 font-bold',
        cellContent: (
          <span className="text-xs leading-none drop-shadow-[0_1px_2px_rgba(0,0,0,0.9)]">⚓</span>
        ),
        ariaLabel: `Defensive cell ${notation}, SUNK ${type} (diagonal striped)`,
      }
    }
    if (shotResult === 'hit') {
      return {
        cellStyle:
          'bg-rose-600/35 border-rose-500 text-rose-100 font-black shadow-[inset_0_0_8px_rgba(244,63,94,0.45)]',
        cellContent: (
          <span className="text-xs leading-none drop-shadow-[0_0_4px_rgba(244,63,94,0.95)]">✕</span>
        ),
        ariaLabel: `Defensive cell ${notation}, HIT on ${type}`,
      }
    }
    return {
      cellStyle: 'bg-cyan-950/25 border-cyan-500/35 text-cyan-200 font-mono font-bold',
      cellContent: occupied.index === 0 ? <span className="sr-only">{type}</span> : null,
      ariaLabel: `Defensive cell ${notation}, ${type}`,
    }
  }

  if (shotResult === 'miss') {
    return {
      cellStyle: 'bg-slate-900/80 border-slate-700 text-cyan-300 font-bold',
      cellContent: <span className="text-xs leading-none">○</span>,
      ariaLabel: `Defensive cell ${notation}, enemy miss`,
    }
  }
  return {
    cellStyle: 'bg-slate-950/40 border-slate-900 text-slate-700',
    cellContent: null,
    ariaLabel: `Defensive cell ${notation}`,
  }
}
