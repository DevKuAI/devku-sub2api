import { describe, expect, it } from 'vitest'
import { positiveMoney, subtractMoney, taxPreview } from '../money'
import type { AccountingInput } from '../types'
const receipt = (
  sales: string,
  rate: string,
  inclusive: boolean,
  currency = 'CNY',
): AccountingInput => ({
  organization_id: 'org',
  currency,
  amount: sales,
  occurred_at: '',
  details: { sales_amount: sales, tax_rate: rate, tax_inclusive: inclusive },
})
describe('accounting amount previews', () => {
  it('keeps cents above the JavaScript integer precision limit', () => {
    expect(subtractMoney('9007199254740993.15', '9007199254740993.01')).toBe(
      '0.14',
    )
    expect(subtractMoney('10', '10.25')).toBe('-0.25')
  })
  it('matches inclusive and exclusive tax rounding', () => {
    expect(taxPreview(receipt('58000', '0.06', true))).toMatchObject({
      tax: '3283.02',
      net: '54716.98',
      receivable: '58000',
    })
    expect(taxPreview(receipt('42000', '0.06', false))).toMatchObject({
      tax: '2520',
      net: '42000',
      receivable: '44520',
    })
    expect(taxPreview(receipt('0.01', '0.06', true))?.tax).toBe('0')
  })
  it('uses each currency precision and preserves unknown amounts', () => {
    expect(taxPreview(receipt('100', '0.055', false, 'JPY'))?.tax).toBe('6')
    expect(taxPreview(receipt('1.005', '0.05', false, 'KWD'))?.tax).toBe('0.05')
    expect(taxPreview(receipt('', '0.06', true))).toBeNull()
    expect(positiveMoney('invalid')).toBe(false)
    expect(subtractMoney('', '1')).toBe('')
  })
})
