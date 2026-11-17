import React from 'react'

export type MobileTab = 'target' | 'defense' | 'log'

export interface MobileTabSwitcherProps {
  activeTab: MobileTab
  onChange: (tab: MobileTab) => void
  logCount: number
}

/** Tab bar that swaps between radar, own fleet and log below the `lg` breakpoint. */
export const MobileTabSwitcher: React.FC<MobileTabSwitcherProps> = ({
  activeTab,
  onChange,
  logCount,
}) => {
  const tabs: { id: MobileTab; label: string }[] = [
    { id: 'target', label: '🎯 Target Radar' },
    { id: 'defense', label: '🛡️ Own Fleet' },
    { id: 'log', label: `📜 Log (${logCount})` },
  ]

  return (
    <div className="flex lg:hidden rounded-lg bg-slate-900 p-1 border border-slate-800 gap-1">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={() => onChange(tab.id)}
          className={`flex-1 py-2 rounded text-xs font-mono font-bold uppercase transition-all ${
            activeTab === tab.id
              ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30'
              : 'text-slate-400 hover:text-white'
          }`}
        >
          {tab.label}
        </button>
      ))}
    </div>
  )
}
