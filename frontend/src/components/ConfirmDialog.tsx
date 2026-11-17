import React from 'react'

export interface ConfirmDialogProps {
  titleId: string
  title: string
  /** Extra classes for the title color (defaults to white). */
  titleClassName?: string
  confirmLabel: string
  pending?: boolean
  pendingLabel?: string
  onCancel: () => void
  onConfirm: () => void
  children: React.ReactNode
}

/** Modal shell for destructive confirmations such as leaving or forfeiting a match. */
export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  titleId,
  title,
  titleClassName = 'text-white',
  confirmLabel,
  pending = false,
  pendingLabel,
  onCancel,
  onConfirm,
  children,
}) => (
  <div
    role="dialog"
    aria-modal="true"
    aria-labelledby={titleId}
    className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm"
  >
    <div className="w-full max-w-md p-6 rounded-2xl bg-slate-900 border border-slate-800 shadow-2xl text-center space-y-4">
      <div className="w-12 h-12 mx-auto rounded-full bg-rose-500/20 border border-rose-500/40 flex items-center justify-center text-rose-400 text-xl font-bold">
        ⚠️
      </div>
      <h3 id={titleId} className={`text-lg font-mono font-bold uppercase ${titleClassName}`}>
        {title}
      </h3>
      <p className="text-xs text-slate-300 font-mono">{children}</p>
      <div className="flex items-center justify-center gap-3 pt-2">
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-mono font-bold transition-all"
        >
          Cancel
        </button>
        <button
          type="button"
          disabled={pending}
          onClick={onConfirm}
          className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-mono font-bold transition-all disabled:opacity-50"
        >
          {pending && pendingLabel ? pendingLabel : confirmLabel}
        </button>
      </div>
    </div>
  </div>
)
