const KEY_SESSION_TOKEN = 'battleship.sessionToken'
const KEY_DISPLAY_NAME = 'battleship.displayName'

export interface StoredSession {
  sessionToken: string | null
  displayName: string | null
}

/**
 * Safely accesses localStorage with defensive error handling for restricted environments
 * (e.g. private browsing, cross-origin iframes, storage quota exceeded).
 */
export const sessionStore = {
  loadSession(): StoredSession {
    try {
      const sessionToken = localStorage.getItem(KEY_SESSION_TOKEN)
      const displayName = localStorage.getItem(KEY_DISPLAY_NAME)
      return {
        sessionToken: sessionToken && sessionToken.trim() !== '' ? sessionToken : null,
        displayName: displayName && displayName.trim() !== '' ? displayName : null,
      }
    } catch {
      return { sessionToken: null, displayName: null }
    }
  },

  saveSession(sessionToken: string, displayName: string): void {
    try {
      localStorage.setItem(KEY_SESSION_TOKEN, sessionToken)
      localStorage.setItem(KEY_DISPLAY_NAME, displayName)
    } catch {
      // Storage unavailable or disabled
    }
  },

  saveDisplayName(displayName: string): void {
    try {
      localStorage.setItem(KEY_DISPLAY_NAME, displayName)
    } catch {
      // Storage unavailable or disabled
    }
  },

  saveSessionToken(sessionToken: string): void {
    try {
      localStorage.setItem(KEY_SESSION_TOKEN, sessionToken)
    } catch {
      // Storage unavailable or disabled
    }
  },

  clearSession(): void {
    try {
      localStorage.removeItem(KEY_SESSION_TOKEN)
      // Note: we can keep displayName in localStorage so the user doesn't have to retype it,
      // while clearing sessionToken so they start a clean session.
    } catch {
      // Storage unavailable or disabled
    }
  },

  clearAll(): void {
    try {
      localStorage.removeItem(KEY_SESSION_TOKEN)
      localStorage.removeItem(KEY_DISPLAY_NAME)
    } catch {
      // Storage unavailable or disabled
    }
  },
}

