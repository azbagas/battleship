export interface ValidationResult {
  isValid: boolean
  cleaned: string
  runeCount: number
  error?: string
}

/**
 * Sanitizes and validates a display name according to the Battleship protocol:
 * - Strips ASCII control chars (0x00-0x1F, 0x7F) and Unicode control (Cc) & format (Cf) characters.
 * - Trims leading and trailing whitespace.
 * - Enforces length between 1 and 20 runes.
 */
export function sanitizeDisplayName(raw: string): ValidationResult {
  // Strip ASCII controls and Unicode control (Cc) & format (Cf) characters
  // eslint-disable-next-line no-control-regex
  const withoutControls = raw.replace(/[\u0000-\u001F\u007F\p{Cc}\p{Cf}]/gu, '')
  const cleaned = withoutControls.trim()
  const runeCount = Array.from(cleaned).length

  if (cleaned.length === 0) {
    return {
      isValid: false,
      cleaned: '',
      runeCount: 0,
      error: 'Display name cannot be empty',
    }
  }

  if (runeCount > 20) {
    return {
      isValid: false,
      cleaned,
      runeCount,
      error: 'Display name must not exceed 20 characters',
    }
  }

  return {
    isValid: true,
    cleaned,
    runeCount,
  }
}
