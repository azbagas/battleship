import React from 'react'
import { GridFrame } from '../../components/GridFrame'
import { GridShipOverlay } from '../../components/ShipSvg'
import { PlacedShip, ShipType, ShotResult } from '../../types/protocol'
import { COLUMN_LABELS, ROW_LABELS, coordToNotation } from '../../utils/placement'
import { SUNK_PATTERN_STYLE, defenseCellAppearance } from './cellAppearance'
import { OccupiedCell } from './useBattleView'

export interface DefenseGridProps {
  ownFleet: PlacedShip[]
  ownShots: Map<string, ShotResult>
  occupiedMap: Map<string, OccupiedCell>
  sunkCellMap: Map<string, ShipType>
  sunkShipTypes: Set<ShipType>
  visibleOnMobile: boolean
}

/** The player's own fleet with incoming enemy salvos; read-only. */
export const DefenseGrid: React.FC<DefenseGridProps> = ({
  ownFleet,
  ownShots,
  occupiedMap,
  sunkCellMap,
  sunkShipTypes,
  visibleOnMobile,
}) => (
  <div
    className={`p-2.5 sm:p-4 md:p-5 rounded-2xl bg-slate-900/70 border border-slate-800 shadow-xl backdrop-blur-sm ${
      visibleOnMobile ? 'block' : 'hidden lg:block'
    }`}
  >
    <div className="flex items-center justify-between mb-3">
      <div className="flex items-center gap-2">
        <span className="w-2.5 h-2.5 rounded-full bg-cyan-400" />
        <h3 className="text-sm font-mono font-bold uppercase tracking-wider text-white">
          Defensive Grid (Your Fleet)
        </h3>
      </div>
      <span className="text-xs font-mono text-slate-400">{ownShots.size} Enemy Salvos</span>
    </div>

    <GridFrame
      colSizeClass="md:w-8 md:h-7"
      rowSizeClass="md:h-8"
      labelColorClass="text-cyan-400/80"
      matrixClassName="border border-cyan-500/30 shadow-[inset_0_0_24px_rgba(6,182,212,0.12)]"
    >
      <GridShipOverlay ships={ownFleet} sunkShipTypes={sunkShipTypes} defaultVariant="deployed" />

      {ROW_LABELS.map((_, y) =>
        COLUMN_LABELS.map((_, x) => {
          const cellKey = `${x},${y}`
          const isSunk = Boolean(sunkCellMap.get(cellKey))
          const { cellStyle, cellContent, ariaLabel } = defenseCellAppearance({
            notation: coordToNotation({ x, y }),
            occupied: occupiedMap.get(cellKey),
            shotResult: ownShots.get(cellKey),
            isSunk,
          })

          return (
            <div
              key={cellKey}
              aria-label={ariaLabel}
              style={isSunk ? SUNK_PATTERN_STYLE : undefined}
              className={`w-[26px] h-[26px] sm:w-8 sm:h-8 md:w-8 md:h-8 rounded border flex items-center justify-center font-mono relative z-10 ${cellStyle}`}
            >
              {cellContent}
            </div>
          )
        }),
      )}
    </GridFrame>
  </div>
)
