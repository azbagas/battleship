import { describe, it, expect } from 'vitest'
import { sanitizeDisplayName } from './sanitize'

describe('sanitizeDisplayName', () => {
  it('accepts valid alphanumeric names', () => {
    const res = sanitizeDisplayName('AdmiralNimitz')
    expect(res.isValid).toBe(true)
    expect(res.cleaned).toBe('AdmiralNimitz')
    expect(res.runeCount).toBe(13)
    expect(res.error).toBeUndefined()
  })

  it('accepts names with spaces in the middle', () => {
    const res = sanitizeDisplayName('Fleet Commander')
    expect(res.isValid).toBe(true)
    expect(res.cleaned).toBe('Fleet Commander')
    expect(res.runeCount).toBe(15)
  })

  it('trims leading and trailing whitespace', () => {
    const res = sanitizeDisplayName('   Captain Hook   ')
    expect(res.isValid).toBe(true)
    expect(res.cleaned).toBe('Captain Hook')
    expect(res.runeCount).toBe(12)
  })

  it('rejects empty or whitespace-only names', () => {
    expect(sanitizeDisplayName('').isValid).toBe(false)
    expect(sanitizeDisplayName('   ').isValid).toBe(false)
    expect(sanitizeDisplayName('\t\n\r').isValid).toBe(false)
  })

  it('strips ASCII and Unicode control characters', () => {
    // 0x07 (BEL), 0x1B (ESC), 0x00 (NUL), 0x7F (DEL), 0x200B (Zero-width space - format Cf)
    const raw = 'Naval\u0000\u0007Officer\u007F\u200B'
    const res = sanitizeDisplayName(raw)
    expect(res.isValid).toBe(true)
    expect(res.cleaned).toBe('NavalOfficer')
  })

  it('correctly handles multi-byte and unicode characters', () => {
    const res = sanitizeDisplayName('艦隊司令')
    expect(res.isValid).toBe(true)
    expect(res.cleaned).toBe('艦隊司令')
    expect(res.runeCount).toBe(4)
  })

  it('enforces maximum length of 20 characters', () => {
    const exactly20 = '12345678901234567890'
    const res20 = sanitizeDisplayName(exactly20)
    expect(res20.isValid).toBe(true)
    expect(res20.runeCount).toBe(20)

    const over20 = '123456789012345678901'
    const res21 = sanitizeDisplayName(over20)
    expect(res21.isValid).toBe(false)
    expect(res21.runeCount).toBe(21)
    expect(res21.error).toContain('must not exceed 20 characters')
  })
})

