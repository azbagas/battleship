import { describe, it, expect, beforeEach } from 'vitest'
import { sessionStore } from './storage'

describe('sessionStore', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('loads empty session when storage is empty', () => {
    const session = sessionStore.loadSession()
    expect(session.sessionToken).toBeNull()
    expect(session.displayName).toBeNull()
  })

  it('saves and loads session properly', () => {
    sessionStore.saveSession('tok_12345', 'Commander')
    const session = sessionStore.loadSession()
    expect(session.sessionToken).toBe('tok_12345')
    expect(session.displayName).toBe('Commander')
  })

  it('clears session token while preserving display name on clearSession', () => {
    sessionStore.saveSession('tok_12345', 'Commander')
    sessionStore.clearSession()
    const session = sessionStore.loadSession()
    expect(session.sessionToken).toBeNull()
    expect(session.displayName).toBe('Commander')
  })

  it('clears all entries on clearAll', () => {
    sessionStore.saveSession('tok_12345', 'Commander')
    sessionStore.clearAll()
    const session = sessionStore.loadSession()
    expect(session.sessionToken).toBeNull()
    expect(session.displayName).toBeNull()
  })

  it('updates session token independently', () => {
    sessionStore.saveDisplayName('Captain')
    sessionStore.saveSessionToken('tok_new')
    const session = sessionStore.loadSession()
    expect(session.sessionToken).toBe('tok_new')
    expect(session.displayName).toBe('Captain')
  })
})

