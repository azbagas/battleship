import React from 'react'
import { SessionProvider } from './context/SessionProvider'
import { LobbyProvider } from './context/LobbyProvider'
import { useSession } from './context/SessionContext'
import { useLobby } from './context/LobbyContext'
import { HomeScreen } from './components/HomeScreen'
import { LobbyScreen } from './components/LobbyScreen'
import { WaitingRoomScreen } from './components/WaitingRoomScreen'
import { ReadyUpScreen } from './components/ReadyUpScreen'
import { PlacementScreen } from './components/PlacementScreen'
import { BattleScreen } from './components/BattleScreen'
import { GameOverScreen } from './components/GameOverScreen'

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
        <main className="min-h-screen bg-slate-950 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-slate-900 via-slate-950 to-black text-slate-100 flex flex-col items-center justify-center p-4 selection:bg-cyan-500/30 selection:text-cyan-200">
          <AppContent />
        </main>
      </LobbyProvider>
    </SessionProvider>
  )
}

export default App
