import React from 'react'
import { Orientation } from '../../types/protocol'

export interface PlacementControlsProps {
  isFleetConfirmed: boolean
  isFleetComplete: boolean
  shipCount: number
  currentOrientation: Orientation
  onRotate: () => void
  onRandomize: () => void
  onReset: () => void
  onConfirm: () => void
}

const SECONDARY_BUTTON =
  'flex items-center justify-center gap-2 p-2.5 rounded-xl bg-slate-800/80 hover:bg-slate-700/80 border border-slate-700 text-white text-xs font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed'

/** Rotate, randomize, reset and confirm actions. */
export const PlacementControls: React.FC<PlacementControlsProps> = ({
  isFleetConfirmed,
  isFleetComplete,
  shipCount,
  currentOrientation,
  onRotate,
  onRandomize,
  onReset,
  onConfirm,
}) => (
  <div className="bg-slate-900/90 backdrop-blur-md rounded-2xl p-4 sm:p-5 shadow-2xl border border-slate-800 space-y-3">
    <h3 className="text-xs font-bold uppercase tracking-wider text-slate-300">Tactical Controls</h3>

    <div className="grid grid-cols-2 gap-2">
      <button
        type="button"
        onClick={onRotate}
        disabled={isFleetConfirmed}
        className={SECONDARY_BUTTON}
        title="Rotate orientation (or press R key)"
      >
        <svg
          className="w-4 h-4 text-cyan-400"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67" />
        </svg>
        <span>Rotate ({currentOrientation === 'horizontal' ? 'H' : 'V'})</span>
      </button>

      <button
        type="button"
        onClick={onRandomize}
        disabled={isFleetConfirmed}
        className={SECONDARY_BUTTON}
      >
        <svg
          className="w-4 h-4 text-amber-400"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <polyline points="16 3 21 3 21 8" />
          <line x1="4" y1="20" x2="21" y2="3" />
          <polyline points="21 16 21 21 16 21" />
          <line x1="15" y1="15" x2="21" y2="21" />
          <line x1="4" y1="4" x2="9" y2="9" />
        </svg>
        <span>Randomize</span>
      </button>
    </div>

    <button
      type="button"
      onClick={onReset}
      disabled={isFleetConfirmed || shipCount === 0}
      className="w-full py-2 rounded-xl bg-slate-950/60 hover:bg-slate-900 border border-slate-800 text-slate-400 hover:text-slate-200 text-xs font-mono transition disabled:opacity-30 disabled:cursor-not-allowed"
    >
      Reset All Ships to Dock
    </button>

    <div className="pt-2 border-t border-slate-800">
      <button
        type="button"
        onClick={onConfirm}
        disabled={isFleetConfirmed || !isFleetComplete}
        className={`w-full py-3.5 px-4 rounded-xl font-bold text-sm tracking-wide transition-all shadow-lg flex items-center justify-center gap-2 ${
          isFleetConfirmed
            ? 'bg-emerald-600 text-white cursor-default shadow-emerald-950/50'
            : isFleetComplete
              ? 'bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-slate-950 shadow-cyan-500/20 active:scale-[0.99] cursor-pointer'
              : 'bg-slate-800 text-slate-500 border border-slate-700/60 cursor-not-allowed'
        }`}
      >
        {isFleetConfirmed ? (
          <>
            <svg className="w-5 h-5 text-emerald-200" viewBox="0 0 20 20" fill="currentColor">
              <path
                fillRule="evenodd"
                d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                clipRule="evenodd"
              />
            </svg>
            <span>Fleet Confirmed</span>
          </>
        ) : (
          <>
            <svg
              className="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
            </svg>
            <span>{isFleetComplete ? 'Confirm Fleet' : `Place All Ships (${shipCount}/5)`}</span>
          </>
        )}
      </button>
    </div>
  </div>
)
