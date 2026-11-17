import React, { useCallback, useState } from 'react'
import { useLobby } from '../../context/LobbyContext'
import { useSession } from '../../context/SessionContext'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { OpponentDisconnectBanner } from '../../components/OpponentDisconnectBanner'
import { ReconnectingBanner } from '../../components/ReconnectingBanner'
import { useCountdown } from '../../hooks/useCountdown'
import { useGridKeyboardNav } from '../../hooks/useGridKeyboardNav'
import { getRoomParticipants } from '../../utils/roomParticipants'
import { FleetDock } from './FleetDock'
import { PlacementControls } from './PlacementControls'
import { PlacementGrid } from './PlacementGrid'
import { PlacementHeader } from './PlacementHeader'
import { usePlacementFleet } from './usePlacementFleet'
import { usePlacementShortcuts } from './usePlacementShortcuts'

const PLACEMENT_SECONDS = 60

export const PlacementScreen: React.FC = () => {
  const {
    activeRoom,
    isFleetConfirmed,
    isOpponentConfirmed,
    ownFleet,
    placementDeadline,
    submitFleet,
    syncDraftFleet,
    forfeitGame,
    isLeavingRoom,
    opponentDisconnectDeadline,
    lobbyError,
    clearLobbyError,
  } = useLobby()
  const { connectionStatus, isSessionConflict } = useSession()

  const [showLeaveModal, setShowLeaveModal] = useState<boolean>(false)
  // The keyboard focus ring and preview only show while the player is navigating by keyboard.
  const [isKeyboardNav, setIsKeyboardNav] = useState<boolean>(false)
  const markKeyboardNav = useCallback(() => setIsKeyboardNav(true), [])

  const secondsRemaining = useCountdown(placementDeadline, PLACEMENT_SECONDS, { intervalMs: 500 })
  const { role, playerName, opponentName } = getRoomParticipants(activeRoom)

  const [focusedCoord, setFocusedCoord] = useGridKeyboardNav({
    enabled: !isFleetConfirmed,
    onArrow: markKeyboardNav,
  })

  const fleet = usePlacementFleet({
    isFleetConfirmed,
    serverFleet: ownFleet,
    submitFleet,
    syncDraftFleet,
    keyboardCoord: isKeyboardNav ? focusedCoord : null,
  })

  usePlacementShortcuts({
    isFleetConfirmed,
    isFleetComplete: fleet.isFleetComplete,
    focusedCoord,
    onRotate: fleet.handleRotate,
    onCellActivate: fleet.handleCellClick,
    onConfirm: fleet.handleConfirm,
    onSelectShip: fleet.setSelectedShipType,
    onKeyboardUse: markKeyboardNav,
  })

  const { setHoveredCoord } = fleet

  return (
    <div className="w-full max-w-5xl mx-auto px-1 sm:px-4 py-3 sm:py-4 space-y-4">
      <ReconnectingBanner status={connectionStatus} />

      {opponentDisconnectDeadline && <OpponentDisconnectBanner opponentName={opponentName} />}

      <PlacementHeader
        sectorId={activeRoom?.roomId.slice(-6).toUpperCase() || 'ALPHA'}
        role={role}
        playerName={playerName}
        opponentName={opponentName}
        isOpponentConfirmed={isOpponentConfirmed}
        secondsRemaining={secondsRemaining}
        connectionStatus={connectionStatus}
        isSessionConflict={isSessionConflict}
        lobbyError={lobbyError}
        onDismissError={clearLobbyError}
        onLeave={() => setShowLeaveModal(true)}
      />

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4 items-start">
        <PlacementGrid
          ships={fleet.ships}
          selectedShipType={fleet.selectedShipType}
          hoverPreview={fleet.hoverPreview}
          previewCellMap={fleet.previewCellMap}
          occupiedCellMap={fleet.occupiedCellMap}
          focusedCoord={focusedCoord}
          isKeyboardNav={isKeyboardNav}
          isFleetConfirmed={isFleetConfirmed}
          onFocusCell={setFocusedCoord}
          onCellClick={(x, y) => {
            setIsKeyboardNav(false)
            fleet.handleCellClick(x, y)
          }}
          onCellHover={(x, y) => {
            if (!isFleetConfirmed) {
              setIsKeyboardNav(false)
              setHoveredCoord({ x, y })
            }
          }}
          onHoverLeave={() => setHoveredCoord(null)}
          onCellDragOver={fleet.handleCellDragOver}
          onCellDrop={fleet.handleDrop}
        />

        <div className="lg:col-span-5 space-y-4">
          <PlacementControls
            isFleetConfirmed={isFleetConfirmed}
            isFleetComplete={fleet.isFleetComplete}
            shipCount={fleet.ships.length}
            currentOrientation={fleet.currentOrientation}
            onRotate={fleet.handleRotate}
            onRandomize={fleet.handleRandomize}
            onReset={fleet.handleReset}
            onConfirm={fleet.handleConfirm}
          />
          <FleetDock
            ships={fleet.ships}
            selectedShipType={fleet.selectedShipType}
            isFleetConfirmed={isFleetConfirmed}
            onDockClick={fleet.handleDockClick}
            onDragStart={fleet.handleDragStart}
            onRemoveShip={fleet.handleRemoveShip}
          />
        </div>
      </div>

      {showLeaveModal && (
        <ConfirmDialog
          titleId="leave-title"
          title="Leave Game / Forfeit Match?"
          titleClassName="text-rose-300"
          confirmLabel="Confirm Forfeit"
          pending={isLeavingRoom}
          pendingLabel="Forfeiting..."
          onCancel={() => setShowLeaveModal(false)}
          onConfirm={() => {
            setShowLeaveModal(false)
            forfeitGame()
          }}
        >
          Leaving during fleet deployment forfeits the match immediately and awards victory to{' '}
          <strong className="text-white">{opponentName}</strong>. This cannot be undone.
        </ConfirmDialog>
      )}
    </div>
  )
}
