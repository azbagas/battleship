import React from 'react'
import { ShipSvg } from '../../components/ShipSvg'
import { PlacedShip, ShipType, ShotResult } from '../../types/protocol'
import { FLEET_SHIPS, getShipCells } from '../../utils/placement'

/** Scaled ship silhouette shared by both fleet panels. */
const ShipSilhouette: React.FC<{
  type: ShipType
  size: number
  variant: 'sunk' | 'damaged' | 'deployed' | 'dock'
  className: string
}> = ({ type, size, variant, className }) => (
  <div className="my-1.5 h-3.5 flex items-center justify-center px-1">
    <div
      style={{
        width: `${(size / 5) * 100}%`,
        maxWidth: `${size * 14}px`,
        height: '14px',
      }}
      className="flex items-center justify-center"
    >
      <ShipSvg type={type} orientation="horizontal" variant={variant} className={className} />
    </div>
  </div>
)

export interface FriendlyFleetPanelProps {
  playerName: string
  ownFleet: PlacedShip[]
  ownShots: Map<string, ShotResult>
  sunkTypes: Set<ShipType>
  sunkCount: number
}

export const FriendlyFleetPanel: React.FC<FriendlyFleetPanelProps> = ({
  playerName,
  ownFleet,
  ownShots,
  sunkTypes,
  sunkCount,
}) => (
  <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800">
    <div className="flex items-center justify-between mb-2">
      <div className="flex items-center gap-2">
        <span className="w-2 h-2 rounded-full bg-cyan-400" />
        <span className="text-xs font-mono font-bold uppercase text-slate-200">
          Friendly Fleet ({playerName})
        </span>
      </div>
      <span className="text-xs font-mono text-cyan-300">{5 - sunkCount}/5 Vessels Operational</span>
    </div>

    <div className="grid grid-cols-2 sm:grid-cols-5 gap-2">
      {FLEET_SHIPS.map((ship) => {
        const isSunk = sunkTypes.has(ship.type)
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
            <ShipSilhouette
              type={ship.type}
              size={ship.size}
              variant={isSunk ? 'sunk' : hitsOnShip > 0 ? 'damaged' : 'deployed'}
              className="w-full h-full"
            />
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
)

export interface HostileFleetPanelProps {
  opponentName: string
  sunkTypes: Set<ShipType>
  sunkCount: number
}

export const HostileFleetPanel: React.FC<HostileFleetPanelProps> = ({
  opponentName,
  sunkTypes,
  sunkCount,
}) => (
  <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800">
    <div className="flex items-center justify-between mb-2">
      <div className="flex items-center gap-2">
        <span className="w-2 h-2 rounded-full bg-rose-400" />
        <span className="text-xs font-mono font-bold uppercase text-slate-200">
          Hostile Fleet ({opponentName})
        </span>
      </div>
      <span className="text-xs font-mono text-rose-400">{5 - sunkCount}/5 Vessels Unconfirmed</span>
    </div>

    <div className="grid grid-cols-2 sm:grid-cols-5 gap-2">
      {FLEET_SHIPS.map((ship) => {
        const isSunk = sunkTypes.has(ship.type)

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
            <ShipSilhouette
              type={ship.type}
              size={ship.size}
              variant={isSunk ? 'sunk' : 'dock'}
              className={`w-full h-full ${isSunk ? 'opacity-95' : 'opacity-65'}`}
            />
            <div className="flex items-center justify-center gap-1 mt-1">
              {Array.from({ length: ship.size }).map((_, i) => (
                <span
                  key={i}
                  className={`w-2 h-2 rounded-full ${isSunk ? 'bg-rose-500' : 'bg-slate-600'}`}
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
)
