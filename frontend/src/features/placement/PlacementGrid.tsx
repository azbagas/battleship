import React from 'react'
import { GridFrame } from '../../components/GridFrame'
import { GridShipOverlay } from '../../components/ShipSvg'
import { Coord, PlacedShip, ShipType } from '../../types/protocol'
import { COLUMN_LABELS, ROW_LABELS } from '../../utils/placement'
import { HoverPreview } from './usePlacementFleet'

export interface PlacementGridProps {
  ships: PlacedShip[]
  selectedShipType: ShipType | null
  hoverPreview: HoverPreview | null
  previewCellMap: Map<string, boolean>
  occupiedCellMap: Map<string, { ship: PlacedShip; index: number }>
  focusedCoord: Coord
  isKeyboardNav: boolean
  isFleetConfirmed: boolean
  onFocusCell: (coord: Coord) => void
  onCellClick: (x: number, y: number) => void
  onCellHover: (x: number, y: number) => void
  onHoverLeave: () => void
  onCellDragOver: (x: number, y: number, e: React.DragEvent) => void
  onCellDrop: (x: number, y: number, e: React.DragEvent) => void
}

const KBD = 'px-1 py-0.5 bg-slate-800 rounded text-cyan-300'

/** Interactive 10x10 ocean grid with preview, selection and keyboard focus ring. */
export const PlacementGrid: React.FC<PlacementGridProps> = ({
  ships,
  selectedShipType,
  hoverPreview,
  previewCellMap,
  occupiedCellMap,
  focusedCoord,
  isKeyboardNav,
  isFleetConfirmed,
  onFocusCell,
  onCellClick,
  onCellHover,
  onHoverLeave,
  onCellDragOver,
  onCellDrop,
}) => (
  <div className="lg:col-span-7 bg-slate-900/90 backdrop-blur-md rounded-2xl p-2.5 sm:p-4 md:p-6 shadow-2xl border border-slate-800 flex flex-col items-center">
    <div className="w-full flex items-center justify-between mb-4">
      <h2 className="text-sm font-bold uppercase tracking-wider text-slate-300 flex items-center gap-2">
        <span className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse" />
        Ocean Grid (10×10)
      </h2>
      <span className="text-xs font-mono text-slate-400">{ships.length}/5 Ships Deployed</span>
    </div>

    <GridFrame
      colSizeClass="md:w-10 md:h-7"
      rowSizeClass="md:h-10"
      labelColorClass="text-cyan-400/80"
      matrixClassName="border border-cyan-500/30 shadow-[inset_0_0_28px_rgba(6,182,212,0.12)]"
      onMatrixMouseLeave={onHoverLeave}
    >
      <GridShipOverlay
        ships={ships}
        selectedShipType={selectedShipType}
        previewShip={
          hoverPreview && hoverPreview.inBounds
            ? { ship: hoverPreview.ship, valid: hoverPreview.valid }
            : null
        }
      />

      {ROW_LABELS.map((row, y) =>
        COLUMN_LABELS.map((_, x) => {
          const cellKey = `${x},${y}`
          const occupied = occupiedCellMap.get(cellKey)
          const isPreview = previewCellMap.has(cellKey)
          const isPreviewValid = previewCellMap.get(cellKey)
          const isSelected = selectedShipType && occupied?.ship.type === selectedShipType
          const isFocused = focusedCoord.x === x && focusedCoord.y === y
          const focusRing =
            isFocused && isKeyboardNav && !isFleetConfirmed
              ? 'ring-2 ring-cyan-400 ring-offset-1 ring-offset-slate-950 z-20'
              : ''

          let cellBg =
            'bg-slate-900/45 border-slate-800/75 hover:border-cyan-400/50 hover:bg-cyan-500/10'

          if (occupied) {
            cellBg = isSelected
              ? 'bg-amber-500/15 border-amber-400/70 text-amber-200'
              : 'bg-cyan-950/35 border-cyan-500/40 text-cyan-200'
          }

          if (isPreview) {
            cellBg = isPreviewValid
              ? 'bg-emerald-500/25 border-emerald-400/80 text-emerald-200'
              : 'bg-rose-500/30 border-rose-500/90 text-rose-200'
          }

          return (
            <button
              key={cellKey}
              type="button"
              tabIndex={isFocused ? 0 : -1}
              onFocus={() => onFocusCell({ x, y })}
              aria-label={`Cell ${COLUMN_LABELS[x]}${row}${
                occupied ? `, ${occupied.ship.type}` : ''
              }`}
              onClick={() => onCellClick(x, y)}
              onMouseEnter={() => onCellHover(x, y)}
              onDragOver={(e) => onCellDragOver(x, y, e)}
              onDrop={(e) => onCellDrop(x, y, e)}
              disabled={isFleetConfirmed}
              className={`w-[26px] h-[26px] sm:w-8 sm:h-8 md:w-10 md:h-10 rounded border text-[9px] sm:text-[10px] font-mono flex items-center justify-center transition-all duration-150 relative z-10 ${cellBg} ${
                isFleetConfirmed ? 'cursor-not-allowed opacity-90' : 'cursor-pointer'
              } ${focusRing}`}
            >
              {occupied && occupied.index === 0 && (
                <span className="sr-only">{occupied.ship.type}</span>
              )}
            </button>
          )
        }),
      )}
    </GridFrame>

    <div className="mt-4 text-center text-xs text-slate-400">
      {isFleetConfirmed ? (
        <span className="text-cyan-400 font-semibold">
          Fleet confirmed and locked. Standing by for combat launch...
        </span>
      ) : selectedShipType ? (
        <span>
          Selected: <strong className="text-white uppercase">{selectedShipType}</strong>. Tap or
          press <kbd className={KBD}>Enter</kbd> to place, or press <kbd className={KBD}>R</kbd> to
          rotate.
        </span>
      ) : (
        <span>
          Click, drag, or use <kbd className={KBD}>1–5</kbd> + <kbd className={KBD}>Arrows</kbd> to
          deploy fleet. Press <kbd className={KBD}>C</kbd> to confirm.
        </span>
      )}
    </div>
  </div>
)
