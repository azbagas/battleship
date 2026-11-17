import React from 'react'
import { COLUMN_LABELS, ROW_LABELS } from '../utils/placement'

export interface GridFrameProps {
  /** Responsive `md:` width/height classes for the column header cells. */
  colSizeClass: string
  /** Responsive `md:` height class for the row label cells. */
  rowSizeClass: string
  /** Text color class for the A-J / 1-10 labels. */
  labelColorClass: string
  /** Border and inner-shadow classes for the matrix container. */
  matrixClassName: string
  onMatrixMouseLeave?: () => void
  /** Overlay layers and the 100 cells, rendered inside the 10-column grid. */
  children: React.ReactNode
}

/** Shared 10x10 board chrome: column letters, row numbers and the bordered matrix. */
export const GridFrame: React.FC<GridFrameProps> = ({
  colSizeClass,
  rowSizeClass,
  labelColorClass,
  matrixClassName,
  onMatrixMouseLeave,
  children,
}) => (
  <div className="relative inline-block select-none overflow-x-auto max-w-full">
    <div className="flex gap-0.5 sm:gap-1 ml-6 sm:ml-8 mb-1">
      {COLUMN_LABELS.map((col) => (
        <div
          key={col}
          className={`w-[26px] h-5 sm:w-8 sm:h-6 ${colSizeClass} flex items-center justify-center text-[10px] sm:text-xs font-mono font-bold ${labelColorClass}`}
        >
          {col}
        </div>
      ))}
    </div>

    <div className="flex items-start">
      <div className="flex flex-col gap-0.5 sm:gap-1 mr-1 sm:mr-1.5">
        {ROW_LABELS.map((row) => (
          <div
            key={row}
            className={`w-5 sm:w-6 h-[26px] sm:h-8 ${rowSizeClass} flex items-center justify-end pr-1 text-[10px] sm:text-xs font-mono font-bold ${labelColorClass}`}
          >
            {row}
          </div>
        ))}
      </div>

      <div
        className={`relative p-1 rounded-xl bg-slate-950/90 ${matrixClassName} overflow-hidden`}
      >
        <div
          className="relative grid grid-cols-10 gap-0.5 sm:gap-1"
          onMouseLeave={onMatrixMouseLeave}
        >
          {children}
        </div>
      </div>
    </div>
  </div>
)
