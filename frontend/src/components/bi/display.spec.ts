import { describe, expect, it } from 'vitest'
import { biDate, biNumber, biText } from './display'

describe('BI metadata formatting', () => {
  it('preserves large token counts and exact decimal fees', () => {
    expect(biNumber('9007199254740993')).toBe('9,007,199,254,740,993')
    expect(biNumber('-12345.0000012300')).toBe('-12,345.0000012300')
  })

  it('uses placeholders only for missing values', () => {
    for (const value of [null, undefined, '', '  ']) expect(biText(value)).toBe('-')
    expect(biText(0)).toBe('0')
    expect(biText(false)).toBe('false')
  })

  it('formats Shanghai dates across the UTC day boundary and retains invalid values', () => {
    expect(biDate('2026-10-13T16:00:00Z')).toBe('2026-10-14 00:00:00')
    expect(biDate('invalid')).toBe('invalid')
    expect(biDate(null)).toBe('-')
  })
})
