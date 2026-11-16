import React from 'react'
import { Orientation, PlacedShip, ShipType } from '../types/protocol'
import { SHIP_SIZES } from '../utils/placement'

export type ShipVisualVariant =
  | 'deployed'
  | 'selected'
  | 'preview-valid'
  | 'preview-invalid'
  | 'dock'
  | 'damaged'
  | 'sunk'

export interface ShipSvgProps {
  type: ShipType
  orientation?: Orientation
  variant?: ShipVisualVariant
  className?: string
  ariaHidden?: boolean
}

interface Palette {
  hullFill: string
  hullStroke: string
  deckFill: string
  detailStroke: string
  accentFill: string
  glowColor: string
}

const VARIANT_PALETTES: Record<ShipVisualVariant, Palette> = {
  deployed: {
    hullFill: '#0b2239',
    hullStroke: '#22d3ee',
    deckFill: '#103152',
    detailStroke: '#67e8f9',
    accentFill: '#06b6d4',
    glowColor: 'rgba(34, 211, 238, 0.35)',
  },
  selected: {
    hullFill: '#3b2106',
    hullStroke: '#fbbf24',
    deckFill: '#543009',
    detailStroke: '#fde68a',
    accentFill: '#f59e0b',
    glowColor: 'rgba(251, 191, 36, 0.45)',
  },
  'preview-valid': {
    hullFill: 'rgba(6, 78, 59, 0.72)',
    hullStroke: '#34d399',
    deckFill: 'rgba(4, 120, 87, 0.65)',
    detailStroke: '#a7f3d0',
    accentFill: '#10b981',
    glowColor: 'rgba(52, 211, 153, 0.4)',
  },
  'preview-invalid': {
    hullFill: 'rgba(76, 5, 25, 0.72)',
    hullStroke: '#f43f5e',
    deckFill: 'rgba(136, 19, 55, 0.65)',
    detailStroke: '#fecdd3',
    accentFill: '#e11d48',
    glowColor: 'rgba(244, 63, 94, 0.4)',
  },
  dock: {
    hullFill: '#0f172a',
    hullStroke: '#38bdf8',
    deckFill: '#1e293b',
    detailStroke: '#7dd3fc',
    accentFill: '#0284c7',
    glowColor: 'rgba(56, 189, 248, 0.25)',
  },
  damaged: {
    hullFill: '#1e1b2e',
    hullStroke: '#fb7185',
    deckFill: '#2d1c2e',
    detailStroke: '#fda4af',
    accentFill: '#f43f5e',
    glowColor: 'rgba(244, 63, 94, 0.35)',
  },
  sunk: {
    hullFill: '#2c0612',
    hullStroke: '#f43f5e',
    deckFill: '#4c0519',
    detailStroke: '#fda4af',
    accentFill: '#be123c',
    glowColor: 'rgba(225, 29, 72, 0.4)',
  },
}

// Render Carrier top-down blueprint (500 x 100)
const renderCarrier = (p: Palette) => (
  <g>
    {/* Outer Flight Deck & Hull */}
    <polygon
      points="14,22 75,12 440,12 488,28 488,72 440,88 110,88 55,76 14,68"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="3.5"
      strokeLinejoin="round"
    />
    {/* Inner Angled Flight Deck Surface */}
    <polygon
      points="26,28 82,19 435,19 474,33 474,67 435,80 116,80 62,70 26,62"
      fill={p.deckFill}
      stroke={p.detailStroke}
      strokeWidth="1.2"
      strokeOpacity="0.6"
    />
    {/* Straight Axial Runway Centerline */}
    <line
      x1="36"
      y1="46"
      x2="462"
      y2="46"
      stroke={p.detailStroke}
      strokeWidth="2"
      strokeDasharray="14 8"
      strokeOpacity="0.85"
    />
    {/* Straight Runway Bound Lines */}
    <line
      x1="36"
      y1="31"
      x2="452"
      y2="31"
      stroke={p.detailStroke}
      strokeWidth="1"
      strokeOpacity="0.45"
    />
    <line
      x1="36"
      y1="61"
      x2="452"
      y2="61"
      stroke={p.detailStroke}
      strokeWidth="1"
      strokeOpacity="0.45"
    />
    {/* Forward Dual Steam Catapult Tracks */}
    <line x1="275" y1="38" x2="468" y2="38" stroke={p.accentFill} strokeWidth="2.2" />
    <line x1="275" y1="54" x2="468" y2="54" stroke={p.accentFill} strokeWidth="2.2" />
    {/* Arresting Gear Wires (Stern) */}
    <line x1="65" y1="31" x2="65" y2="61" stroke={p.detailStroke} strokeWidth="1.4" strokeOpacity="0.7" />
    <line x1="82" y1="31" x2="82" y2="61" stroke={p.detailStroke} strokeWidth="1.4" strokeOpacity="0.7" />
    <line x1="99" y1="31" x2="99" y2="61" stroke={p.detailStroke} strokeWidth="1.4" strokeOpacity="0.7" />
    {/* Port & Starboard Deck Elevators */}
    <rect x="170" y="13" width="38" height="9" fill={p.hullFill} stroke={p.detailStroke} strokeWidth="1.2" />
    <rect x="330" y="13" width="38" height="9" fill={p.hullFill} stroke={p.detailStroke} strokeWidth="1.2" />
    {/* Starboard Island Superstructure & Radar Mast */}
    <rect
      x="225"
      y="68"
      width="92"
      height="16"
      rx="3"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="2.2"
    />
    <rect x="245" y="71" width="32" height="10" rx="1.5" fill={p.accentFill} fillOpacity="0.7" />
    <circle cx="296" cy="76" r="4.5" fill={p.detailStroke} />
    <line x1="288" y1="76" x2="304" y2="76" stroke={p.hullFill} strokeWidth="1.5" />
  </g>
)

// Render Battleship top-down blueprint (400 x 100)
const renderBattleship = (p: Palette) => (
  <g>
    {/* Heavy Armored Dreadnought Hull */}
    <path
      d="M 14,32 Q 14,16 58,15 L 295,15 Q 355,18 390,50 Q 355,82 295,85 L 58,85 Q 14,84 14,68 Z"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="3.5"
      strokeLinejoin="round"
    />
    {/* Inner Armored Belt Deck */}
    <path
      d="M 26,35 Q 26,23 64,23 L 288,23 Q 340,26 372,50 Q 340,74 288,77 L 64,77 Q 26,77 26,65 Z"
      fill={p.deckFill}
      stroke={p.detailStroke}
      strokeWidth="1.2"
      strokeOpacity="0.55"
    />
    {/* Bow Wavebreaker Chevron */}
    <polyline
      points="330,33 352,50 330,67"
      fill="none"
      stroke={p.detailStroke}
      strokeWidth="2"
      strokeOpacity="0.75"
    />
    {/* Forward Main Turret A (Triple Barrel) */}
    <g>
      <rect x="292" y="34" width="28" height="32" rx="5" fill={p.hullFill} stroke={p.hullStroke} strokeWidth="2" />
      <line x1="320" y1="41" x2="348" y2="41" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
      <line x1="320" y1="50" x2="352" y2="50" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
      <line x1="320" y1="59" x2="348" y2="59" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
    </g>
    {/* Forward Superfiring Main Turret B (Triple Barrel) */}
    <g>
      <rect x="242" y="32" width="30" height="36" rx="5" fill={p.hullFill} stroke={p.hullStroke} strokeWidth="2" />
      <line x1="272" y1="40" x2="298" y2="40" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
      <line x1="272" y1="50" x2="302" y2="50" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
      <line x1="272" y1="60" x2="298" y2="60" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
    </g>
    {/* Central Citadel & Bridge Superstructure */}
    <rect
      x="130"
      y="27"
      width="98"
      height="46"
      rx="8"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="2.2"
    />
    <rect x="148" y="35" width="62" height="30" rx="4" fill={p.accentFill} fillOpacity="0.35" stroke={p.detailStroke} strokeWidth="1.2" />
    {/* Smokestack & Radar Array */}
    <circle cx="166" cy="50" r="8" fill={p.hullFill} stroke={p.detailStroke} strokeWidth="2" />
    <circle cx="192" cy="50" r="7" fill={p.hullFill} stroke={p.detailStroke} strokeWidth="2" />
    {/* Secondary Sponson Batteries */}
    <circle cx="155" cy="22" r="4" fill={p.accentFill} />
    <circle cx="195" cy="22" r="4" fill={p.accentFill} />
    <circle cx="155" cy="78" r="4" fill={p.accentFill} />
    <circle cx="195" cy="78" r="4" fill={p.accentFill} />
    {/* Aft Main Turret X (Triple Barrel facing Stern) */}
    <g>
      <rect x="74" y="34" width="30" height="32" rx="5" fill={p.hullFill} stroke={p.hullStroke} strokeWidth="2" />
      <line x1="44" y1="41" x2="74" y2="41" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
      <line x1="40" y1="50" x2="74" y2="50" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
      <line x1="44" y1="59" x2="74" y2="59" stroke={p.detailStroke} strokeWidth="2.8" strokeLinecap="round" />
    </g>
  </g>
)

// Render Cruiser top-down blueprint (300 x 100)
const renderCruiser = (p: Palette) => (
  <g>
    {/* Guided-Missile Cruiser Hull */}
    <path
      d="M 14,28 L 210,17 Q 262,20 290,50 Q 262,80 210,83 L 14,72 Z"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="3.5"
      strokeLinejoin="round"
    />
    {/* Inner Deck */}
    <path
      d="M 24,34 L 205,25 Q 248,28 272,50 Q 248,72 205,75 L 24,66 Z"
      fill={p.deckFill}
      stroke={p.detailStroke}
      strokeWidth="1.2"
      strokeOpacity="0.55"
    />
    {/* Forward Rapid-Fire Naval Gun */}
    <circle cx="232" cy="50" r="11" fill={p.hullFill} stroke={p.hullStroke} strokeWidth="2" />
    <line x1="243" y1="50" x2="266" y2="50" stroke={p.detailStroke} strokeWidth="3" strokeLinecap="round" />
    {/* Foredeck VLS Missile Silo Matrix */}
    <rect x="192" y="36" width="22" height="28" rx="2" fill={p.hullFill} stroke={p.detailStroke} strokeWidth="1.5" />
    <line x1="203" y1="36" x2="203" y2="64" stroke={p.detailStroke} strokeWidth="1" />
    <line x1="192" y1="45" x2="214" y2="45" stroke={p.detailStroke} strokeWidth="1" />
    <line x1="192" y1="55" x2="214" y2="55" stroke={p.detailStroke} strokeWidth="1" />
    {/* Central Aegis Radar Superstructure */}
    <polygon
      points="105,30 175,28 182,50 175,72 105,70 98,50"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="2.2"
    />
    <rect x="116" y="38" width="46" height="24" rx="3" fill={p.accentFill} fillOpacity="0.4" stroke={p.detailStroke} strokeWidth="1.2" />
    <circle cx="139" cy="50" r="6" fill={p.detailStroke} />
    {/* Stern Helipad & VLS */}
    <circle cx="54" cy="50" r="14" fill="none" stroke={p.detailStroke} strokeWidth="1.5" strokeDasharray="5 3" />
    <path d="M 49,44 L 49,56 M 59,44 L 59,56 M 49,50 L 59,50" stroke={p.detailStroke} strokeWidth="1.8" />
  </g>
)

// Render Submarine top-down blueprint (300 x 100)
const renderSubmarine = (p: Palette) => (
  <g>
    {/* Stern Cruciform Stabilizer Fins */}
    <polygon points="18,50 38,16 52,16 42,50 52,84 38,84" fill={p.deckFill} stroke={p.hullStroke} strokeWidth="2.2" />
    {/* Shrouded Propulsor Ring */}
    <rect x="8" y="39" width="10" height="22" rx="3" fill={p.accentFill} stroke={p.hullStroke} strokeWidth="2" />
    {/* Teardrop Pressure Hull */}
    <path
      d="M 22,50 Q 28,22 120,21 L 235,21 Q 288,22 290,50 Q 288,78 235,79 L 120,79 Q 28,78 22,50 Z"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="3.5"
    />
    {/* Forward Retractable Diving Planes (Hydroplanes) */}
    <polygon points="222,21 236,7 250,7 244,21" fill={p.deckFill} stroke={p.hullStroke} strokeWidth="2" />
    <polygon points="222,79 236,93 250,93 244,79" fill={p.deckFill} stroke={p.hullStroke} strokeWidth="2" />
    {/* Inner Dorsal Spine Casing */}
    <rect x="65" y="34" width="188" height="32" rx="16" fill={p.deckFill} stroke={p.detailStroke} strokeWidth="1.2" strokeOpacity="0.6" />
    {/* Ballistic Missile Silo Hatches (2 Rows) */}
    {[82, 100, 118, 136].map((x) => (
      <g key={x}>
        <circle cx={x} cy="43" r="5" fill={p.hullFill} stroke={p.detailStroke} strokeWidth="1.3" />
        <circle cx={x} cy="57" r="5" fill={p.hullFill} stroke={p.detailStroke} strokeWidth="1.3" />
      </g>
    ))}
    {/* Streamlined Conning Tower (Sail) & Periscope Masts */}
    <ellipse cx="192" cy="50" rx="28" ry="11" fill={p.hullFill} stroke={p.hullStroke} strokeWidth="2.5" />
    <circle cx="186" cy="50" r="3.5" fill={p.detailStroke} />
    <circle cx="198" cy="50" r="2.5" fill={p.accentFill} />
    {/* Bow Sonar Array Ribs */}
    <path d="M 264,34 Q 277,50 264,66" fill="none" stroke={p.detailStroke} strokeWidth="1.8" strokeOpacity="0.8" />
    <path d="M 273,39 Q 282,50 273,61" fill="none" stroke={p.detailStroke} strokeWidth="1.5" strokeOpacity="0.6" />
  </g>
)

// Render Destroyer top-down blueprint (200 x 100)
const renderDestroyer = (p: Palette) => (
  <g>
    {/* Angular Stealth Chined Hull */}
    <polygon
      points="14,28 128,18 188,50 128,82 14,72 20,50"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="3.5"
      strokeLinejoin="round"
    />
    {/* Faceted Stealth Deck */}
    <polygon
      points="26,35 124,26 170,50 124,74 26,65"
      fill={p.deckFill}
      stroke={p.detailStroke}
      strokeWidth="1.2"
      strokeOpacity="0.6"
    />
    {/* Forward Rapid-Fire Stealth Turret */}
    <polygon
      points="126,40 144,43 144,57 126,60"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="2"
    />
    <line x1="144" y1="50" x2="165" y2="50" stroke={p.detailStroke} strokeWidth="3" strokeLinecap="round" />
    {/* Angular Bridge & Phased-Array Mast */}
    <polygon
      points="66,32 114,30 120,50 114,70 66,68 58,50"
      fill={p.hullFill}
      stroke={p.hullStroke}
      strokeWidth="2.2"
    />
    <polygon
      points="78,40 104,40 108,50 104,60 78,60"
      fill={p.accentFill}
      fillOpacity="0.45"
      stroke={p.detailStroke}
      strokeWidth="1.2"
    />
    <circle cx="92" cy="50" r="4.5" fill={p.detailStroke} />
    {/* Aft Torpedo Launch Tubes */}
    <line x1="32" y1="39" x2="52" y2="43" stroke={p.detailStroke} strokeWidth="2.2" strokeLinecap="round" />
    <line x1="32" y1="61" x2="52" y2="57" stroke={p.detailStroke} strokeWidth="2.2" strokeLinecap="round" />
  </g>
)

export const ShipSvg: React.FC<ShipSvgProps> = ({
  type,
  orientation = 'horizontal',
  variant = 'deployed',
  className = '',
  ariaHidden = true,
}) => {
  const size = SHIP_SIZES[type] || 3
  const lengthUnits = size * 100
  const palette = VARIANT_PALETTES[variant]

  const isVertical = orientation === 'vertical'
  const viewBox = isVertical ? `0 0 100 ${lengthUnits}` : `0 0 ${lengthUnits} 100`

  const renderHullByType = () => {
    switch (type) {
      case 'carrier':
        return renderCarrier(palette)
      case 'battleship':
        return renderBattleship(palette)
      case 'cruiser':
        return renderCruiser(palette)
      case 'submarine':
        return renderSubmarine(palette)
      case 'destroyer':
        return renderDestroyer(palette)
    }
  }

  return (
    <svg
      viewBox={viewBox}
      preserveAspectRatio="none"
      aria-hidden={ariaHidden}
      data-testid={`ship-svg-${type}-${orientation}`}
      style={{
        filter: `drop-shadow(0 0 6px ${palette.glowColor})`,
      }}
      className={`select-none pointer-events-none ${className}`}
    >
      {isVertical ? (
        <g transform="translate(100, 0) rotate(90)">{renderHullByType()}</g>
      ) : (
        renderHullByType()
      )}
    </svg>
  )
}

export interface GridShipOverlayProps {
  ships: PlacedShip[]
  selectedShipType?: ShipType | null
  sunkShipTypes?: Set<ShipType>
  previewShip?: {
    ship: PlacedShip
    valid: boolean
  } | null
  defaultVariant?: ShipVisualVariant
}

// Renders continuous multi-cell SVG ships mapped accurately onto a 10x10 CSS Grid
export const GridShipOverlay: React.FC<GridShipOverlayProps> = ({
  ships,
  selectedShipType = null,
  sunkShipTypes,
  previewShip = null,
  defaultVariant = 'deployed',
}) => {
  return (
    <div
      aria-hidden="true"
      className="absolute inset-0 pointer-events-none grid grid-cols-10 grid-rows-10 gap-0.5 sm:gap-1 z-[1]"
    >
      {ships.map((ship) => {
        const size = SHIP_SIZES[ship.type]
        const isSelected = selectedShipType === ship.type
        const isSunk = sunkShipTypes?.has(ship.type) ?? false
        const variant: ShipVisualVariant = isSunk
          ? 'sunk'
          : isSelected
            ? 'selected'
            : defaultVariant

        const gridColumn =
          ship.orientation === 'horizontal'
            ? `${ship.origin.x + 1} / span ${size}`
            : `${ship.origin.x + 1} / span 1`
        const gridRow =
          ship.orientation === 'vertical'
            ? `${ship.origin.y + 1} / span ${size}`
            : `${ship.origin.y + 1} / span 1`

        return (
          <div
            key={`${ship.type}-${ship.origin.x}-${ship.origin.y}-${ship.orientation}`}
            style={{ gridColumn, gridRow }}
            className="w-full h-full flex items-center justify-center p-[1px]"
          >
            <ShipSvg
              type={ship.type}
              orientation={ship.orientation}
              variant={variant}
              className="w-full h-full"
            />
          </div>
        )
      })}

      {previewShip && (
        <div
          key="ship-placement-preview"
          style={{
            gridColumn:
              previewShip.ship.orientation === 'horizontal'
                ? `${previewShip.ship.origin.x + 1} / span ${SHIP_SIZES[previewShip.ship.type]}`
                : `${previewShip.ship.origin.x + 1} / span 1`,
            gridRow:
              previewShip.ship.orientation === 'vertical'
                ? `${previewShip.ship.origin.y + 1} / span ${SHIP_SIZES[previewShip.ship.type]}`
                : `${previewShip.ship.origin.y + 1} / span 1`,
          }}
          className="w-full h-full flex items-center justify-center p-[1px] z-[2]"
        >
          <ShipSvg
            type={previewShip.ship.type}
            orientation={previewShip.ship.orientation}
            variant={previewShip.valid ? 'preview-valid' : 'preview-invalid'}
            className="w-full h-full opacity-90"
          />
        </div>
      )}
    </div>
  )
}
