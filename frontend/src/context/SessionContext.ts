import { createContext, useContext } from 'react'
import { ConnectionStatus, PlayerSession } from '../types/protocol'
import { WebSocketClient } from '../services/wsClient'

export interface SessionContextValue {
  connectionStatus: ConnectionStatus
  player: PlayerSession | null
  savedDisplayName: string | null
  errorMessage: string | null
  isSessionConflict: boolean
  isRestored: boolean
  login: (rawName: string) => boolean
  logout: () => void
  retry: () => void
  clearConflictAndReset: () => void
  client: WebSocketClient
}

export const SessionContext = createContext<SessionContextValue | undefined>(undefined)

export function useSession(): SessionContextValue {
  const context = useContext(SessionContext)
  if (!context) {
    throw new Error('useSession must be used within a SessionProvider')
  }
  return context
}

