import { describe, expect, it } from 'vitest'
import { formatMoney } from '../format'
describe('financial decimal display', () => {
  it('preserves cents beyond the safe floating-point integer range', () => {
    expect(formatMoney('999999999999999999.99', 'CNY')).toContain(
      '999,999,999,999,999,999.99',
    )
  })
  it('preserves the sign of a negative fractional balance', () => {
    expect(formatMoney('-0.50', 'CNY')).toContain('-')
    expect(formatMoney('-0.50', 'CNY')).toContain('0.50')
  })
  it('supports zero and three decimal currencies', () => {
    expect(formatMoney('100.000000', 'JPY')).not.toContain('.')
    expect(formatMoney('1.123000', 'KWD')).toContain('1.123')
  })
})
