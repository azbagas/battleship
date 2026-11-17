import React, { useState, useEffect, useCallback } from 'react'
import { useLobby } from '../../context/LobbyContext'
import { useSession } from '../../context/SessionContext'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { OpponentDisconnectBanner } from '../../components/OpponentDisconnectBanner'
import { ReconnectingBanner } from '../../components/ReconnectingBanner'
import { useCountdown } from '../../hooks/useCountdown'
import { isTypingTarget, useGridKeyboardNav } from '../../hooks/useGridKeyboardNav'
import { BattleGameOverModal } from './BattleGameOverModal'
import { BattleHeader } from './BattleHeader'
import { CombatLog } from './CombatLog'
import { DefenseGrid } from './DefenseGrid'
import { FriendlyFleetPanel, HostileFleetPanel } from './FleetStatusPanels'
import { MobileTab, MobileTabSwitcher } from './MobileTabSwitcher'
import { TargetGrid } from './TargetGrid'
import { TurnStatusBanner } from './TurnStatusBanner'
import { useBattleView } from './useBattleView'

const TURN_SECONDS = 30
const URGENT_THRESHOLD_SECONDS = 10

export const BattleScreen: React.FC = () => {
  const {
    activeRoom,
    ownFleet,
    turnDeadline,
    isMyTurn,
    targetShots,
    ownShots,
    opponentSunkShips,
    ownSunkShips,
    recentEvents,
    gameOverResult,
    fireShot,
    leaveRoom,
    forfeitGame,
    isLeavingRoom,
    opponentDisconnectDeadline,
    lobbyError,
    clearLobbyError,
  } = useLobby()
  const { player, connectionStatus, isSessionConflict } = useSession()

  const view = useBattleView({
    activeRoom,
    ownFleet,
    opponentSunkShips,
    ownSunkShips,
    gameOverResult,
    playerId: player?.playerId,
  })
  const { isMatchOver, playerName, opponentName } = view

  const [activeMobileTab, setActiveMobileTab] = useState<MobileTab>('target')
  const [showSurrenderConfirm, setShowSurrenderConfirm] = useState<boolean>(false)

  const secondsRemaining = useCountdown(turnDeadline, TURN_SECONDS, {
    intervalMs: 250,
    active: !isMatchOver,
  })
  const isTimerUrgent = secondsRemaining <= URGENT_THRESHOLD_SECONDS && !isMatchOver
  const timerPercentage = Math.min(100, Math.max(0, (secondsRemaining / TURN_SECONDS) * 100))

  // Bring the radar to the front on mobile whenever it becomes the player's turn.
  useEffect(() => {
    if (isMyTurn) {
      setActiveMobileTab('target')
    }
  }, [isMyTurn])

  const handleTargetCellClick = useCallback(
    (x: number, y: number) => {
      if (!isMyTurn || isMatchOver) return
      if (targetShots.has(`${x},${y}`)) return
      fireShot({ x, y })
    },
    [isMyTurn, isMatchOver, targetShots, fireShot],
  )

  // Arrow keys move the reticle; Enter/Space (below) fires.
  const [focusedTargetCoord, setFocusedTargetCoord] = useGridKeyboardNav({ enabled: !isMatchOver })

  useEffect(() => {
    if (isMatchOver) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (isTypingTarget(e.target)) return
      if (e.key !== 'Enter' && e.key !== ' ') return
      e.preventDefault()
      handleTargetCellClick(focusedTargetCoord.x, focusedTargetCoord.y)
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isMatchOver, focusedTargetCoord, handleTargetCellClick])

  return (
    <div className="w-full max-w-6xl mx-auto px-1 sm:px-4 py-3 sm:py-4 flex flex-col gap-4">
      <ReconnectingBanner status={connectionStatus} />

      {opponentDisconnectDeadline && <OpponentDisconnectBanner opponentName={opponentName} />}

      <BattleHeader
        sectorId={view.sectorId}
        role={view.role}
        playerName={playerName}
        opponentName={opponentName}
        connectionStatus={connectionStatus}
        isSessionConflict={isSessionConflict}
        isMatchOver={isMatchOver}
        onLeave={() => setShowSurrenderConfirm(true)}
      />

      {lobbyError && (
        <div
          role="alert"
          className="flex items-center justify-between p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs sm:text-sm font-mono"
        >
          <div className="flex items-center gap-2">
            <span className="text-rose-400 font-bold">WARNING:</span>
            <span>{lobbyError}</span>
          </div>
          <button
            type="button"
            onClick={clearLobbyError}
            className="text-rose-400 hover:text-rose-200 ml-4 font-bold text-base leading-none"
            aria-label="Dismiss alert"
          >
            &times;
          </button>
        </div>
      )}

      <TurnStatusBanner
        isMatchOver={isMatchOver}
        isMyTurn={isMyTurn}
        opponentName={opponentName}
        secondsRemaining={secondsRemaining}
        isTimerUrgent={isTimerUrgent}
        timerPercentage={timerPercentage}
      />

      <section className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <FriendlyFleetPanel
          playerName={playerName}
          ownFleet={ownFleet}
          ownShots={ownShots}
          sunkTypes={view.ownSunkShipTypes}
          sunkCount={ownSunkShips.length}
        />
        <HostileFleetPanel
          opponentName={opponentName}
          sunkTypes={view.opponentSunkShipTypes}
          sunkCount={opponentSunkShips.length}
        />
      </section>

      <MobileTabSwitcher
        activeTab={activeMobileTab}
        onChange={setActiveMobileTab}
        logCount={recentEvents.length}
      />

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        <TargetGrid
          targetShots={targetShots}
          sunkCellMap={view.opponentSunkCellMap}
          revealedShips={view.revealedOpponentShips}
          focused={focusedTargetCoord}
          onFocusCell={setFocusedTargetCoord}
          isMyTurn={isMyTurn}
          isMatchOver={isMatchOver}
          onCellClick={handleTargetCellClick}
          visibleOnMobile={activeMobileTab === 'target'}
        />

        <div className="lg:col-span-5 flex flex-col gap-4">
          <DefenseGrid
            ownFleet={ownFleet}
            ownShots={ownShots}
            occupiedMap={view.ownOccupiedMap}
            sunkCellMap={view.ownSunkCellMap}
            sunkShipTypes={view.ownSunkShipTypes}
            visibleOnMobile={activeMobileTab === 'defense'}
          />
          <CombatLog
            events={recentEvents}
            playerId={player?.playerId}
            playerName={playerName}
            opponentName={opponentName}
            visibleOnMobile={activeMobileTab === 'log'}
          />
        </div>
      </div>

      {showSurrenderConfirm && (
        <ConfirmDialog
          titleId="surrender-title"
          title="Leave Game / Forfeit Match?"
          confirmLabel="Confirm Forfeit"
          pending={isLeavingRoom}
          pendingLabel="Forfeiting..."
          onCancel={() => setShowSurrenderConfirm(false)}
          onConfirm={() => {
            setShowSurrenderConfirm(false)
            forfeitGame()
          }}
        >
          Leaving will forfeit the match immediately and award the victory to{' '}
          <strong className="text-white">{opponentName}</strong>. This cannot be undone.
        </ConfirmDialog>
      )}

      {isMatchOver && (
        <BattleGameOverModal
          isWinner={view.isWinner}
          winnerName={view.winnerName}
          reason={view.gameOverReason}
          ownSunkCount={ownSunkShips.length}
          opponentSunkCount={opponentSunkShips.length}
          isLeavingRoom={isLeavingRoom}
          onReturnToLobby={leaveRoom}
        />
      )}
    </div>
  )
}
