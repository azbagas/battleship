import React, { useEffect, useState, useCallback, useMemo } from 'react'
import { ConnectionStatus, ErrorCode, ErrorPayload, HelloOKPayload, PlayerSession } from '../types/protocol'
import { sessionStore } from '../services/storage'
import { wsClient, WebSocketClient } from '../services/wsClient'
import { sanitizeDisplayName } from '../utils/sanitize'
import { SessionContext } from './SessionContext'

export interface SessionProviderProps {
  children: React.ReactNode
  client?: WebSocketClient
}

export const SessionProvider: React.FC<SessionProviderProps> = ({
  children,
  client = wsClient,
}) => {
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>(client.getStatus())
  const [player, setPlayer] = useState<PlayerSession | null>(null)
  const [savedDisplayName, setSavedDisplayName] = useState<string | null>(() => {
    return sessionStore.loadSession().displayName
  })
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [isSessionConflict, setIsSessionConflict] = useState<boolean>(false)
  const [isRestored, setIsRestored] = useState<boolean>(false)

  // Subscriptions to client events
  useEffect(() => {
    const unsubStatus = client.onStatusChange((status, detail) => {
      setConnectionStatus(status)
      if (status === 'connected') {
        setErrorMessage(null)
      } else if (status === 'error' && detail) {
        setErrorMessage(detail)
      }
    })

    const unsubHelloOK = client.onHelloOK((payload: HelloOKPayload) => {
      const newSession: PlayerSession = {
        playerId: payload.playerId,
        sessionToken: payload.sessionToken,
        displayName: payload.displayName,
        restoredState: payload.restoredState,
      }
      setPlayer(newSession)
      setSavedDisplayName(payload.displayName)
      setIsSessionConflict(false)
      setErrorMessage(null)
      sessionStore.saveSession(payload.sessionToken, payload.displayName)
    })

    const unsubError = client.onError((err: ErrorPayload) => {
      if (err.code === ErrorCode.SessionAlreadyActive) {
        setIsSessionConflict(true)
        setErrorMessage('This session is active in another tab. Please close that tab or start a fresh session.')
      } else {
        setErrorMessage(err.message || 'An error occurred during communication with the server')
      }
    })

    return () => {
      unsubStatus()
      unsubHelloOK()
      unsubError()
    }
  }, [client])

  // Automatic state restoration on initial mount
  useEffect(() => {
    const stored = sessionStore.loadSession()
    if (stored.sessionToken && stored.displayName) {
      setIsRestored(true)
      client.connect({
        displayName: stored.displayName,
        sessionToken: stored.sessionToken,
      })
    }
  }, [client])

  const login = useCallback(
    (rawName: string): boolean => {
      const sanitized = sanitizeDisplayName(rawName)
      if (!sanitized.isValid) {
        setErrorMessage(sanitized.error || 'Invalid display name')
        return false
      }

      setErrorMessage(null)
      setIsSessionConflict(false)
      sessionStore.saveDisplayName(sanitized.cleaned)
      setSavedDisplayName(sanitized.cleaned)

      const stored = sessionStore.loadSession()
      client.connect({
        displayName: sanitized.cleaned,
        sessionToken: stored.sessionToken,
      })
      return true
    },
    [client],
  )

  const logout = useCallback(() => {
    sessionStore.clearSession()
    client.disconnect()
    setPlayer(null)
    setIsSessionConflict(false)
    setErrorMessage(null)
    setIsRestored(false)
  }, [client])

  const clearConflictAndReset = useCallback(() => {
    sessionStore.clearSession()
    client.disconnect()
    setIsSessionConflict(false)
    setErrorMessage(null)
    setPlayer(null)
    setIsRestored(false)
  }, [client])

  const retry = useCallback(() => {
    const stored = sessionStore.loadSession()
    const name = player?.displayName || stored.displayName
    if (name) {
      setErrorMessage(null)
      client.connect({
        displayName: name,
        sessionToken: stored.sessionToken,
      })
    }
  }, [client, player])

  const value = useMemo(
    () => ({
      connectionStatus,
      player,
      savedDisplayName,
      errorMessage,
      isSessionConflict,
      isRestored,
      login,
      logout,
      retry,
      clearConflictAndReset,
      client,
    }),
    [
      connectionStatus,
      player,
      savedDisplayName,
      errorMessage,
      isSessionConflict,
      isRestored,
      login,
      logout,
      retry,
      clearConflictAndReset,
      client,
    ],
  )

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

