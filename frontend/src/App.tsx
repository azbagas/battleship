import React from 'react'
import { SessionProvider } from './context/SessionProvider'
import { HomeScreen } from './components/HomeScreen'

export const App: React.FC = () => {
  return (
    <SessionProvider>
      <main className="min-h-screen bg-slate-950 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-slate-900 via-slate-950 to-black text-slate-100 flex flex-col items-center justify-center p-4 selection:bg-cyan-500/30 selection:text-cyan-200">
        <HomeScreen />
      </main>
    </SessionProvider>
  )
}

export default App
