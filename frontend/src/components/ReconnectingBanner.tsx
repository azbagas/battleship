import React from 'react'
import { ConnectionStatus } from '../types/protocol'

export interface ReconnectingBannerProps {
  status: ConnectionStatus
  message?: string
  className?: string
}

/** Notice shown while this client's own WebSocket link is down or being restored. */
export const ReconnectingBanner: React.FC<ReconnectingBannerProps> = ({
  status,
  message = 'Connection lost. Attempting to restore link to sector... Play will resume once reconnected.',
  className = '',
}) => {
  if (status !== 'connecting' && status !== 'disconnected') return null

  return (
    <div
      role="status"
      className={`${className ? `${className} ` : ''}p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs font-mono flex items-center gap-2`}
    >
      <span className="w-2 h-2 rounded-full bg-amber-400 animate-ping" />
      <span>{message}</span>
    </div>
  )
}
