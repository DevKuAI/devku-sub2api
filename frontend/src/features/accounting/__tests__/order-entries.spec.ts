import { describe, expect, it } from 'vitest'
import { entryDefinitions, percent, type EntryContext } from '../orderEntries'
import { receivingAccount, record, vendors } from './testing'
import type { AccountingOrder, AccountingRecord, OrderItemKind } from '../types'

function context(items: Partial<Record<OrderItemKind, AccountingRecord[]>> = {}): EntryContext {
  return {
    order: { id: 'order-one', organization_id: 'org-one', organization_name: '上海星海智能科技有限公司' } as AccountingOrder,
    items: { transfers: [], invoices: [], 'vcc-cards': [], 'vcc-transactions': [], 'vendor-invoices': [], ...items },
    receivingAccounts: [receivingAccount],
    vendors,
  }
}

describe('order entry definitions', () => {
  it('fills a receipt from the organization and the only matching account', () => {
    const values = entryDefinitions.transfers.defaults(context())
    expect(values).toMatchObject({ currency: 'CNY', payer_name: '上海星海智能科技有限公司', tax_rate: '6', receiving_account_id: 'account-one' })
  })

  it('sends a receipt as fully invoiced at its rate and keeps the account fixed when editing', () => {
    const values = { occurred_at: '2026-10-12', currency: 'CNY', amount: '1060', bank_reference: 'B-1', payer_name: '客户', tax_rate: '6', receiving_account_id: 'account-one', remark: '' }
    const created = entryDefinitions.transfers.payload(values, context())
    expect(created.details).toMatchObject({ sales_amount: '1060', tax_inclusive: true, tax_rate: '0.06', receiving_account_id: 'account-one' })
    const existing = record({ status: 'RECEIVED', amount: '1060', details: { sales_amount: '1060' } })
    expect(entryDefinitions.transfers.payload(values, context(), existing).details).not.toHaveProperty('receiving_account_id')
  })

  it('keeps an older receipt’s invoiced amount while its received amount is unchanged', () => {
    const existing = record({ status: 'RECEIVED', amount: '990', details: { sales_amount: '1000', difference_reason: '手续费' } })
    const values = { ...entryDefinitions.transfers.values(existing), remark: '补充' }
    expect(entryDefinitions.transfers.payload(values, context(), existing).details.sales_amount).toBe('1000')
  })

  it('proposes the not yet invoiced amount and splits the invoice tax exactly', () => {
    const receipt = record({ status: 'RECEIVED', amount: '1060', details: { tax_rate: '0.06', receiving_account_name: '我方公司' } })
    const values = entryDefinitions.invoices.defaults(context({ transfers: [receipt] }))
    expect(values).toMatchObject({ amount: '1060', tax_rate: '6', issuer_name: '我方公司' })
    const payload = entryDefinitions.invoices.payload({ ...values, invoice_number: 'INV-1', issue_date: '2026-10-12' }, context())
    expect(payload.details).toMatchObject({ net_amount: '1000', tax_amount: '60', recipient_name: '上海星海智能科技有限公司', invoice_number: 'INV-1' })
    expect(entryDefinitions.invoices.values(record({ amount: '1060', details: { net_amount: '1000', tax_amount: '60' } })).tax_rate).toBe('6')
  })

  it('takes the AI vendor and billing month from the order’s spend, not the card issuer', () => {
    const card = record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', details: { provider: 'Airwallex' } })
    const spend = record({ status: 'CONFIRMED', currency: 'USD', amount: '300', occurred_at: '2026-10-12T02:00:00Z', parent_id: 'card-one', details: { transaction_type: 'CONSUME', merchant: 'Anthropic', vendor_id: 'vendor-anthropic' } })
    const covered = record({ status: 'CONFIRMED', currency: 'USD', amount: '100' })
    const items = { 'vcc-cards': [card], 'vcc-transactions': [spend], 'vendor-invoices': [covered] }
    expect(entryDefinitions['vendor-invoices'].defaults(context(items))).toMatchObject({ vendor_id: 'vendor-anthropic', currency: 'USD', amount: '200', period_start: '2026-10-01', period_end: '2026-10-31' })
    expect(entryDefinitions['vcc-transactions'].defaults(context(items))).toMatchObject({ parent_id: 'card-one', vendor_id: 'vendor-anthropic' })
  })

  it('sends card spend in its card’s currency and identifies cards by card ID and last four digits', () => {
    const card = record({ id: 'card-one', status: 'ACTIVE', currency: 'USD' })
    const spend = entryDefinitions['vcc-transactions'].payload({ parent_id: 'card-one', occurred_at: '2026-10-12', amount: '20', vendor_id: 'vendor-openai', provider_transaction_id: '', transaction_type: 'CONSUME', original_transaction_id: '', remark: '' }, context({ 'vcc-cards': [card] }))
    expect(spend).toMatchObject({ parent_id: 'card-one', currency: 'USD', amount: '20' })
    expect(spend.details).not.toHaveProperty('original_transaction_id')
    expect(spend.details).toMatchObject({ vendor_id: 'vendor-openai', merchant: 'OpenAI' })
    const created = entryDefinitions['vcc-cards'].payload({ alias: '席位卡', provider_id: 'vendor-sunrate', currency: 'USD', provider_card_id: ' 2095346501199990785 ', last_four: '39 26', expiry: '2030-12', remark: '' }, context())
    expect(created.details).toEqual({ alias: '席位卡', provider_id: 'vendor-sunrate', provider_card_id: '2095346501199990785', last_four: '3926', expiry: '2030-12', remark: '' })
    expect(created.details).not.toHaveProperty('card_number')
    const fields = entryDefinitions['vcc-cards'].fields.map((field) => field.key)
    expect(fields).toEqual(expect.arrayContaining(['provider_card_id', 'last_four']))
    expect(fields).not.toContain('card_number')
  })

  it('shows stored rates as whole percentages', () => {
    expect(percent('0.06')).toBe('6')
    expect(percent('0.13')).toBe('13')
  })

  it('defaults a new card to the only active card provider', () => {
    expect(entryDefinitions['vcc-cards'].defaults(context()).provider_id).toBe('vendor-sunrate')
    expect(entryDefinitions['vcc-cards'].fields.find((field) => field.key === 'provider_id')).toMatchObject({ type: 'vendor', vendorKind: 'CARD_PROVIDER', required: true })
    expect(entryDefinitions['vendor-invoices'].fields.find((field) => field.key === 'vendor_id')).toMatchObject({ type: 'vendor', vendorKind: 'AI_VENDOR', required: true })
  })
})
