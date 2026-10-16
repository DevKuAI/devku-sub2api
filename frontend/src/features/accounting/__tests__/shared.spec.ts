import { describe, expect, it } from 'vitest'
import { i18n, record } from './testing'
import { detailFields, errorMessage, safeReturnPath } from '../shared'

const { t, te } = i18n().global

describe('accounting shared helpers', () => {
  it('maps API errors to one consistent message', () => {
    expect(errorMessage({ reason: 'ACCOUNTING_PERIOD_CLOSED' }, t, te)).toBe('该账期已结账，需管理员重新开账后才能操作。')
    expect(errorMessage({ reason: 'ACCOUNTING_INVALID', metadata: { field: 'difference_reason' } }, t, te)).toContain('差额原因')
    expect(errorMessage({ reason: 'ACCOUNTING_INVALID', metadata: { field: 'bank_reference' } }, t, te)).toBe('“银行流水号”填写有误，请检查。')
    expect(errorMessage({ message: 'network down' }, t, te)).toBe('network down')
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
})
