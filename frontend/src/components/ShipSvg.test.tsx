import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ShipSvg, GridShipOverlay } from './ShipSvg'
import { FLEET_SHIPS, sunkShipToPlacedShip } from '../utils/placement'

describe('ShipSvg & GridShipOverlay', () => {
  it('renders distinct top-down SVG blueprints for all 5 ship classes in horizontal and vertical orientations', () => {
    FLEET_SHIPS.forEach((ship) => {
      const { unmount: unmountH } = render(
        <ShipSvg type={ship.type} orientation="horizontal" variant="deployed" />,
      )
      const svgH = screen.getByTestId(`ship-svg-${ship.type}-horizontal`)
      expect(svgH).toBeDefined()
      expect(svgH.getAttribute('viewBox')).toBe(`0 0 ${ship.size * 100} 100`)
      unmountH()

      const { unmount: unmountV } = render(
        <ShipSvg type={ship.type} orientation="vertical" variant="selected" />,
      )
      const svgV = screen.getByTestId(`ship-svg-${ship.type}-vertical`)
      expect(svgV).toBeDefined()
      expect(svgV.getAttribute('viewBox')).toBe(`0 0 100 ${ship.size * 100}`)
      unmountV()
    })
  })

  it('reconstructs PlacedShip from SunkShipInfo in horizontal and vertical orientations', () => {
    const horizontalSunk = sunkShipToPlacedShip({
      type: 'destroyer',
      cells: [
        { x: 4, y: 2 },
        { x: 5, y: 2 },
      ],
    })
    expect(horizontalSunk).toEqual({
      type: 'destroyer',
      origin: { x: 4, y: 2 },
      orientation: 'horizontal',
    })

    const verticalSunk = sunkShipToPlacedShip({
      type: 'cruiser',
      cells: [
        { x: 7, y: 3 },
        { x: 7, y: 4 },
        { x: 7, y: 5 },
      ],
    })
    expect(verticalSunk).toEqual({
      type: 'cruiser',
      origin: { x: 7, y: 3 },
      orientation: 'vertical',
    })

    expect(sunkShipToPlacedShip({ type: 'destroyer', cells: [] })).toBeNull()
  })

  it('renders continuous multi-cell vessels and ghost preview inside GridShipOverlay', () => {
    render(
      <GridShipOverlay
        ships={[
          { type: 'carrier', origin: { x: 0, y: 0 }, orientation: 'horizontal' },
          { type: 'submarine', origin: { x: 5, y: 2 }, orientation: 'vertical' },
        ]}
        selectedShipType="carrier"
        previewShip={{
          ship: { type: 'destroyer', origin: { x: 2, y: 6 }, orientation: 'horizontal' },
          valid: true,
        }}
      />,
    )

    expect(screen.getByTestId('ship-svg-carrier-horizontal')).toBeDefined()
    expect(screen.getByTestId('ship-svg-submarine-vertical')).toBeDefined()
    expect(screen.getByTestId('ship-svg-destroyer-horizontal')).toBeDefined()
  })
})
