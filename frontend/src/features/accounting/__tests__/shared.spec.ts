import { describe, expect, it } from 'vitest'
import { i18n, record } from './testing'
import { detailFields, errorMessage, inputMoney, moneyPattern, safeReturnPath } from '../shared'

const { t, te } = i18n().global

describe('accounting shared helpers', () => {
  it('maps API errors to one consistent message', () => {
    expect(errorMessage({ reason: 'ACCOUNTING_PERIOD_CLOSED' }, t, te)).toBe('该账期已结账，需管理员重新开账后才能操作。')
    expect(errorMessage({ reason: 'ACCOUNTING_INVALID', metadata: { field: 'difference_reason' } }, t, te)).toContain('差额原因')
    expect(errorMessage({ reason: 'ACCOUNTING_INVALID', metadata: { field: 'bank_reference' } }, t, te)).toBe('“银行流水号”填写有误，请检查。')
    expect(errorMessage({ reason: 'ACCOUNTING_INVALID', metadata: { field: 'period_end', after: 'period_start' } }, t, te)).toBe('“账单结束”不能早于“账单开始”。')
    expect(errorMessage({ message: 'network down' }, t, te)).toBe('network down')
  })

  it.each([{ code: 'ERR_NETWORK' }, { code: 'ECONNABORTED' }, { code: 'ETIMEDOUT' }, { message: 'Network Error' }])('gives a recoverable localized message for connection failures: %o', cause => {
    expect(errorMessage(cause, t, te)).toBe('网络连接失败。请检查网络后重试。')
  })

  it('shows customers only the public fields of a record', () => {
    const receipt = record({ details: { sales_amount: '106', tax_inclusive: true, tax_rate: '0.06', bank_reference: 'BANK-1', remark: '内部备注' } })
    const admin = detailFields('transfers', receipt, false, t, te).map((field) => field.key)
    const customer = detailFields('transfers', receipt, true, t, te).map((field) => field.key)
    expect(admin).toEqual(expect.arrayContaining(['bank_reference', 'remark', 'tax_rate']))
    expect(customer).toEqual(['sales_amount', 'tax_rate'])
  })

  it('only accepts in-app admin paths as return targets', () => {
    expect(safeReturnPath('/admin/accounting/organizations/org-one/transfers/new')).toBe('/admin/accounting/organizations/org-one/transfers/new')
    expect(safeReturnPath('https://example.com')).toBe('')
    expect(safeReturnPath('//example.com/admin')).toBe('')
    expect(safeReturnPath(['/admin/accounting'])).toBe('')
  })

  it('starts amount inputs with the currency\'s own decimals', () => {
    expect(inputMoney('200.000000', 'USD')).toBe('200.00')
    expect(inputMoney('12.5', 'CNY')).toBe('12.50')
    expect(inputMoney('300', 'USD')).toBe('300.00')
    expect(inputMoney('1500.000000', 'JPY')).toBe('1500')
    expect(inputMoney('0.123400', 'USD')).toBe('0.1234')
    expect(inputMoney('', 'USD')).toBe('')
  })

  it('accepts at most the currency\'s own decimals in amount inputs', () => {
    const accepts = (currency: string, value: string) => new RegExp(`^(?:${moneyPattern(currency)})$`).test(value)
    expect(accepts('USD', '200.00')).toBe(true)
    expect(accepts('USD', '200.000000')).toBe(false)
    expect(accepts('JPY', '1500')).toBe(true)
    expect(accepts('JPY', '1500.5')).toBe(false)
  })
})
