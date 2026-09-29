import { describe, expect, it } from 'vitest'
import { formatDesktopConversationDuration } from '../desktopConversationDuration'

describe('desktop conversation duration', () => {
  it.each([
    [null, '—'], [0, '0ms'], [250, '250ms'], [1250, '1.3s'],
    [60_000, '1m 0s'], [3_723_000, '1h 2m'],
  ])('formats %s milliseconds as %s', (value, expected) => {
    expect(formatDesktopConversationDuration(value)).toBe(expected)
  })
})
