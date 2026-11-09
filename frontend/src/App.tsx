import React from 'react'

export const App: React.FC = () => {
  return (
    <main className="min-h-screen flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-md bg-slate-800 rounded-xl p-6 shadow-xl border border-slate-700 text-center">
        <h1 className="text-3xl font-bold tracking-tight text-white mb-2">Battleship</h1>
        <p className="text-sm text-slate-400 mb-6">
          Online, real-time, two-player naval combat
        </p>
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-medium">
          <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          System Ready
        </div>
      </div>
    </main>
  )
}

export default App

