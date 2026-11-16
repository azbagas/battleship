import React from 'react'

export type BrandLogoSize = 'sm' | 'md' | 'lg'

export interface BrandLogoProps {
  size?: BrandLogoSize
  showWordmark?: boolean
  subtitle?: string
  className?: string
  ariaHidden?: boolean
}

const SIZE_CLASSES: Record<BrandLogoSize, string> = {
  sm: 'w-9 h-9',
  md: 'w-10 h-10',
  lg: 'w-14 h-14',
}

/**
 * Reusable brand emblem for Battleship backed by the standalone public/logo.svg asset.
 */
export const BrandLogo: React.FC<BrandLogoProps> = ({
  size = 'sm',
  showWordmark = false,
  subtitle,
  className = '',
  ariaHidden = true,
}) => {
  const mark = (
    <img
      src="/logo.svg"
      alt={ariaHidden ? '' : 'Battleship logo'}
      aria-hidden={ariaHidden}
      data-testid="brand-logo"
      draggable={false}
      className={`${SIZE_CLASSES[size]} shrink-0 rounded-lg select-none`}
    />
  )

  if (!showWordmark) {
    return <span className={`inline-flex items-center justify-center ${className}`}>{mark}</span>
  }

  return (
    <div className={`inline-flex items-center gap-3 ${className}`}>
      {mark}
      <div>
        <span className="block text-xl font-bold tracking-tight text-white">Battleship</span>
        {subtitle && <span className="block text-xs text-slate-400">{subtitle}</span>}
      </div>
    </div>
  )
}
