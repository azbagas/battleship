import React from 'react'
import { SessionProvider } from './context/SessionProvider'
import { LobbyProvider } from './context/LobbyProvider'
import { useSession } from './context/SessionContext'
import { useLobby } from './context/LobbyContext'
import { HomeScreen } from './features/home/HomeScreen'
import { LobbyScreen } from './features/lobby/LobbyScreen'
import { WaitingRoomScreen } from './features/lobby/WaitingRoomScreen'
import { ReadyUpScreen } from './features/ready-up/ReadyUpScreen'
import { PlacementScreen } from './features/placement/PlacementScreen'
import { BattleScreen } from './features/battle/BattleScreen'
import { GameOverScreen } from './features/game-over/GameOverScreen'

export const AppContent: React.FC = () => {
  const { player, isSessionConflict } = useSession()
  const { activeRoom } = useLobby()

  // If not authenticated or in a multi-tab conflict, display HomeScreen
  if (!player || isSessionConflict) {
    return <HomeScreen />
  }

  // Active room phases
  if (activeRoom) {
    if (activeRoom.phase === 'waiting' && activeRoom.role === 'host') {
      return <WaitingRoomScreen />
    }
    if (activeRoom.phase === 'ready-up') {
      return <ReadyUpScreen />
    }
    if (activeRoom.phase === 'placement') {
      return <PlacementScreen />
    }
    if (activeRoom.phase === 'battle') {
      return <BattleScreen />
    }
    if (activeRoom.phase === 'finished') {
      return <GameOverScreen />
    }
  }

  // Public Lobby
  return <LobbyScreen />
}

export const App: React.FC = () => {
  return (
    <SessionProvider>
      <LobbyProvider>
        <main className="min-h-screen bg-slate-950 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-slate-900 via-slate-950 to-black text-slate-100 flex flex-col items-center justify-center p-2 sm:p-4 md:p-6 selection:bg-cyan-500/30 selection:text-cyan-200">
          <AppContent />
        </main>
      </LobbyProvider>
    </SessionProvider>
  )
}

export default App
