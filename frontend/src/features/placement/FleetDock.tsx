import React from 'react'
import { ShipSvg } from '../../components/ShipSvg'
import { PlacedShip, ShipType } from '../../types/protocol'
import { FLEET_SHIPS, coordToNotation } from '../../utils/placement'

export interface FleetDockProps {
  ships: PlacedShip[]
  selectedShipType: ShipType | null
  isFleetConfirmed: boolean
  onDockClick: (type: ShipType) => void
  onDragStart: (type: ShipType, e: React.DragEvent) => void
  onRemoveShip: (type: ShipType) => void
}

/** Vessel list: select, drag onto the grid, or return a placed ship to reserve. */
export const FleetDock: React.FC<FleetDockProps> = ({
  ships,
  selectedShipType,
  isFleetConfirmed,
  onDockClick,
  onDragStart,
  onRemoveShip,
}) => (
  <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-4 sm:p-5 shadow-2xl border border-slate-800 space-y-3">
    <div className="flex items-center justify-between">
      <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300">Fleet Dock</h3>
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
            onDragStart={(e) => onDragStart(ship.type, e)}
            onClick={() => onDockClick(ship.type)}
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
                  <span className="text-[10px] font-mono text-slate-400">({ship.size} cells)</span>
                </div>

                <div className="flex items-center gap-3 mt-1.5">
                  <div style={{ width: `${ship.size * 24}px`, height: '22px' }} className="shrink-0">
                    <ShipSvg
                      type={ship.type}
                      orientation="horizontal"
                      variant={isSelected ? 'selected' : placed ? 'deployed' : 'dock'}
                      className="w-full h-full"
                    />
                  </div>
                  <div className="flex gap-1">
                    {Array.from({ length: ship.size }).map((_, i) => (
                      <div
                        key={i}
                        className={`w-2.5 h-2.5 rounded-sm border ${
                          placed
                            ? 'bg-cyan-500/60 border-cyan-400/80'
                            : isSelected
                              ? 'bg-amber-500/60 border-amber-400/80'
                              : 'bg-slate-800 border-slate-700'
                        }`}
                      />
                    ))}
                  </div>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-2">
              {placed ? (
                <div className="flex items-center gap-1.5">
                  <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-cyan-950/80 text-cyan-300 border border-cyan-800/60">
                    {coordToNotation(placed.origin)} {placed.orientation === 'horizontal' ? 'H' : 'V'}
                  </span>
                  {!isFleetConfirmed && (
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation()
                        onRemoveShip(ship.type)
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
)
