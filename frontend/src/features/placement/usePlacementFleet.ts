import { useCallback, useEffect, useMemo, useState } from 'react'
import { Coord, Orientation, PlacedShip, ShipType } from '../../types/protocol'
import {
  SHIP_SIZES,
  generateRandomFleet,
  getShipCells,
  isShipInBounds,
  isValidPlacement,
  validateCompleteFleet,
} from '../../utils/placement'

export interface HoverPreview {
  cells: Coord[]
  valid: boolean
  inBounds: boolean
  ship: PlacedShip
}

export interface UsePlacementFleetOptions {
  isFleetConfirmed: boolean
  /** Fleet echoed by the server (auto-confirm or reconnect); replaces local ships when non-empty. */
  serverFleet: PlacedShip[]
  submitFleet: (ships: PlacedShip[]) => void
  syncDraftFleet?: (ships: PlacedShip[]) => void
  /** Cell used for the placement preview while navigating by keyboard, else null. */
  keyboardCoord: Coord | null
}

/** Local fleet-placement state machine: selection, rotation, drag/drop, preview and draft sync. */
export function usePlacementFleet({
  isFleetConfirmed,
  serverFleet,
  submitFleet,
  syncDraftFleet,
  keyboardCoord,
}: UsePlacementFleetOptions) {
  const [ships, setShips] = useState<PlacedShip[]>([])
  const [selectedShipType, setSelectedShipType] = useState<ShipType | null>(null)
  const [currentOrientation, setCurrentOrientation] = useState<Orientation>('horizontal')
  const [draggedShipType, setDraggedShipType] = useState<ShipType | null>(null)
  const [hoveredCoord, setHoveredCoord] = useState<Coord | null>(null)

  useEffect(() => {
    if (serverFleet && serverFleet.length > 0) {
      setShips(serverFleet)
    }
  }, [serverFleet])

  const occupiedCellMap = useMemo(() => {
    const map = new Map<string, { ship: PlacedShip; index: number }>()
    ships.forEach((s) => {
      getShipCells(s.origin, SHIP_SIZES[s.type], s.orientation).forEach((c, idx) => {
        map.set(`${c.x},${c.y}`, { ship: s, index: idx })
      })
    })
    return map
  }, [ships])

  const placedShipTypes = useMemo(() => new Set(ships.map((s) => s.type)), [ships])
  const isFleetComplete = useMemo(() => validateCompleteFleet(ships).valid, [ships])

  const activeShipTypeForPreview = draggedShipType || selectedShipType
  const activePreviewCoord = hoveredCoord || (selectedShipType && keyboardCoord ? keyboardCoord : null)
  const hoverPreview = useMemo<HoverPreview | null>(() => {
    if (!activePreviewCoord || !activeShipTypeForPreview || isFleetConfirmed) {
      return null
    }
    const size = SHIP_SIZES[activeShipTypeForPreview]
    const inBounds = isShipInBounds(activePreviewCoord, size, currentOrientation)
    const ship: PlacedShip = {
      type: activeShipTypeForPreview,
      origin: activePreviewCoord,
      orientation: currentOrientation,
    }
    return {
      cells: inBounds ? getShipCells(activePreviewCoord, size, currentOrientation) : [],
      valid: isValidPlacement(ship, ships),
      inBounds,
      ship,
    }
  }, [activePreviewCoord, activeShipTypeForPreview, currentOrientation, isFleetConfirmed, ships])

  const previewCellMap = useMemo(() => {
    const map = new Map<string, boolean>()
    hoverPreview?.cells.forEach((c) => map.set(`${c.x},${c.y}`, hoverPreview.valid))
    return map
  }, [hoverPreview])

  /** Applies a new fleet locally and mirrors it to the server as a draft. */
  const commitShips = useCallback(
    (next: PlacedShip[]) => {
      setShips(next)
      syncDraftFleet?.(next)
    },
    [syncDraftFleet],
  )

  const handleRotate = useCallback(() => {
    if (isFleetConfirmed) return

    const nextOrientation: Orientation =
      currentOrientation === 'horizontal' ? 'vertical' : 'horizontal'
    setCurrentOrientation(nextOrientation)

    // A placed ship that is selected rotates in place when the result is still valid.
    if (selectedShipType && placedShipTypes.has(selectedShipType)) {
      const existing = ships.find((s) => s.type === selectedShipType)
      if (existing) {
        const candidate: PlacedShip = { ...existing, orientation: nextOrientation }
        if (isValidPlacement(candidate, ships)) {
          commitShips(ships.map((s) => (s.type === selectedShipType ? candidate : s)))
        }
      }
    }
  }, [isFleetConfirmed, currentOrientation, selectedShipType, placedShipTypes, ships, commitShips])

  const placeShipAt = useCallback(
    (type: ShipType, coord: Coord, orientation: Orientation) => {
      if (isFleetConfirmed) return false
      const candidate: PlacedShip = { type, origin: coord, orientation }
      if (!isValidPlacement(candidate, ships)) {
        return false
      }
      commitShips([...ships.filter((s) => s.type !== type), candidate])
      setSelectedShipType(null)
      setDraggedShipType(null)
      return true
    },
    [isFleetConfirmed, ships, commitShips],
  )

  /** Tap-to-place with a selected ship, otherwise select the ship under the cell. */
  const handleCellClick = useCallback(
    (x: number, y: number) => {
      if (isFleetConfirmed) return
      const occupied = occupiedCellMap.get(`${x},${y}`)

      if (selectedShipType) {
        if (placeShipAt(selectedShipType, { x, y }, currentOrientation)) return
      }

      if (occupied) {
        setSelectedShipType(occupied.ship.type)
        setCurrentOrientation(occupied.ship.orientation)
      } else {
        setSelectedShipType(null)
      }
    },
    [isFleetConfirmed, occupiedCellMap, selectedShipType, placeShipAt, currentOrientation],
  )

  const handleConfirm = useCallback(() => {
    if (isFleetConfirmed || !isFleetComplete) return
    submitFleet(ships)
  }, [isFleetConfirmed, isFleetComplete, submitFleet, ships])

  const handleRemoveShip = (type: ShipType) => {
    if (isFleetConfirmed) return
    commitShips(ships.filter((s) => s.type !== type))
    if (selectedShipType === type) {
      setSelectedShipType(null)
    }
  }

  const handleRandomize = () => {
    if (isFleetConfirmed) return
    commitShips(generateRandomFleet())
    setSelectedShipType(null)
  }

  const handleReset = () => {
    if (isFleetConfirmed) return
    commitShips([])
    setSelectedShipType(null)
  }

  /** Dock entry click: toggle selection, adopting the orientation of an already placed ship. */
  const handleDockClick = (type: ShipType) => {
    if (isFleetConfirmed) return
    const placed = ships.find((s) => s.type === type)
    setSelectedShipType(selectedShipType === type ? null : type)
    if (placed) {
      setCurrentOrientation(placed.orientation)
    }
  }

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

  return {
    ships,
    selectedShipType,
    setSelectedShipType,
    currentOrientation,
    setHoveredCoord,
    occupiedCellMap,
    isFleetComplete,
    hoverPreview,
    previewCellMap,
    handleRotate,
    handleCellClick,
    handleConfirm,
    handleRemoveShip,
    handleRandomize,
    handleReset,
    handleDockClick,
    handleDragStart,
    handleCellDragOver,
    handleDrop,
  }
}
