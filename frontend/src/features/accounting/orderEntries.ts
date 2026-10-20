import { addMoney, compareMoney, subtractMoney, taxPreview } from './money'
import { localDate, toISO } from './shared'
import type { AccountingOrder, AccountingRecord, OrderEntryInput, OrderItemKind, ReceivingAccount } from './types'

// What each block of the order page asks for, how it fills itself in, and what it sends.

export type EntryFieldType = 'text' | 'textarea' | 'date' | 'month' | 'money' | 'currency' | 'rate' | 'card' | 'receiving-account' | 'card-number'
export interface EntryField {
  key: string
  type: EntryFieldType
  required?: boolean
  // Only asked when the entry is created; the backend keeps it fixed afterwards.
  createOnly?: boolean
  wide?: boolean
}
export type EntryValues = Record<string, string>
export interface EntryContext {
  order: AccountingOrder
  items: Record<OrderItemKind, AccountingRecord[]>
  receivingAccounts: ReceivingAccount[]
}
export interface EntryDefinition {
  fields: EntryField[]
  // Kinds that take a supporting file when they are added.
  file: boolean
  defaults(context: EntryContext): EntryValues
  values(record: AccountingRecord): EntryValues
  payload(values: EntryValues, context: EntryContext, record?: AccountingRecord): OrderEntryInput
}

export const taxRates = ['0', '1', '3', '6', '9', '13']
const remark: EntryField = { key: 'remark', type: 'textarea', wide: true }

// Rates are stored as fractions ("0.06") and entered as whole percentages ("6").
export function percent(rate: unknown) {
  const value = Number(rate)
  return Number.isFinite(value) ? String(Math.round(value * 10000) / 100) : ''
}
function fraction(percentValue: string) {
  return String(Number(percentValue || 0) / 100)
}
function day(value: string) {
  return value ? localDate(new Date(value)) : ''
}
function text(record: AccountingRecord, key: string) {
  return String(record.details[key] ?? '')
}
function live(records: AccountingRecord[]) {
  return records.filter((record) => record.status !== 'VOID')
}
function effective(record: AccountingRecord) {
  return String(record.details.effective_amount || record.amount)
}
function sum(records: AccountingRecord[], currency: string, value: (record: AccountingRecord) => string = effective) {
  return records.filter((record) => record.currency === currency).reduce((total, record) => addMoney(total, value(record)) || total, '0')
}
function remainder(total: string, used: string) {
  const left = subtractMoney(total, used)
  return compareMoney(left, '0') > 0 ? left : ''
}
function monthRange(date: Date) {
  const start = new Date(date.getFullYear(), date.getMonth(), 1)
  const end = new Date(date.getFullYear(), date.getMonth() + 1, 0)
  return { period_start: localDate(start), period_end: localDate(end) }
}
// The currency most of the order already uses for this kind of money.
function usualCurrency(records: AccountingRecord[], fallback: string) {
  return live(records)[0]?.currency || fallback
}
// The AI vendor this order buys from, taken from the spend already recorded; the card provider only issues the card.
function lastMerchant(context: EntryContext) {
  return String(live(context.items['vcc-transactions']).at(-1)?.details.merchant || '')
}
export function activeCards(context: EntryContext) {
  return context.items['vcc-cards'].filter((card) => card.status === 'ACTIVE')
}
export function cardLabel(card: AccountingRecord) {
  return [card.details.alias, card.details.last_four ? `•••• ${card.details.last_four}` : ''].filter(Boolean).join(' ')
}
// Whether a receipt is fully described or still carries a difference reason set in the old forms.
function salesAmount(values: EntryValues, record?: AccountingRecord) {
  if (record && text(record, 'difference_reason') && compareMoney(values.amount, effective(record)) === 0) return text(record, 'sales_amount')
  return values.amount
}

export const entryDefinitions: Record<OrderItemKind, EntryDefinition> = {
  transfers: {
    file: true,
    fields: [
      { key: 'occurred_at', type: 'date', required: true },
      { key: 'currency', type: 'currency', required: true },
      { key: 'amount', type: 'money', required: true },
      { key: 'bank_reference', type: 'text', required: true },
      { key: 'payer_name', type: 'text', required: true },
      { key: 'tax_rate', type: 'rate', required: true },
      { key: 'receiving_account_id', type: 'receiving-account', required: true, createOnly: true, wide: true },
      remark,
    ],
    defaults(context) {
      const previous = live(context.items.transfers)[0]
      const currency = previous?.currency || 'CNY'
      return {
        occurred_at: localDate(),
        currency,
        amount: '',
        bank_reference: '',
        payer_name: context.order.organization_name,
        tax_rate: previous ? percent(previous.details.tax_rate) : '6',
        receiving_account_id: context.receivingAccounts.find((account) => account.currency === currency)?.id || '',
        remark: '',
      }
    },
    values(record) {
      return {
        occurred_at: day(record.occurred_at),
        currency: record.currency,
        amount: effective(record),
        bank_reference: text(record, 'bank_reference'),
        payer_name: text(record, 'payer_name'),
        tax_rate: percent(record.details.tax_rate),
        receiving_account_id: text(record, 'receiving_account_id'),
        remark: text(record, 'remark'),
      }
    },
    payload(values, _context, record) {
      const details: Record<string, string | boolean> = {
        payer_name: values.payer_name,
        bank_reference: values.bank_reference,
        sales_amount: salesAmount(values, record),
        tax_inclusive: true,
        tax_rate: fraction(values.tax_rate),
        remark: values.remark,
      }
      if (!record) details.receiving_account_id = values.receiving_account_id
      return { currency: values.currency, amount: values.amount, occurred_at: toISO(values.occurred_at), details }
    },
  },
  invoices: {
    file: true,
    fields: [
      { key: 'issue_date', type: 'date', required: true },
      { key: 'invoice_number', type: 'text', required: true },
      { key: 'currency', type: 'currency', required: true },
      { key: 'amount', type: 'money', required: true },
      { key: 'tax_rate', type: 'rate', required: true },
      { key: 'invoice_code', type: 'text' },
      { key: 'issuer_name', type: 'text', required: true, wide: true },
      remark,
    ],
    defaults(context) {
      const receipts = live(context.items.transfers)
      const currency = usualCurrency(receipts, 'CNY')
      const invoiced = sum(live(context.items.invoices), currency)
      return {
        issue_date: localDate(),
        invoice_number: '',
        currency,
        amount: remainder(sum(receipts, currency), invoiced),
        tax_rate: receipts[0] ? percent(receipts[0].details.tax_rate) : '6',
        invoice_code: '',
        issuer_name: String(receipts[0]?.details.receiving_account_name || context.receivingAccounts.find((account) => account.currency === currency)?.account_name || ''),
        remark: '',
      }
    },
    values(record) {
      const net = Number(record.details.net_amount)
      const tax = Number(record.details.tax_amount)
      return {
        issue_date: text(record, 'issue_date') || day(record.occurred_at),
        invoice_number: text(record, 'invoice_number'),
        currency: record.currency,
        amount: record.amount,
        tax_rate: net > 0 ? percent(Math.round((tax / net) * 100) / 100) : '0',
        invoice_code: text(record, 'invoice_code'),
        issuer_name: text(record, 'issuer_name'),
        remark: text(record, 'remark'),
      }
    },
    payload(values, context) {
      const split = taxPreview({ currency: values.currency, amount: values.amount, details: { sales_amount: values.amount, tax_rate: fraction(values.tax_rate), tax_inclusive: true } })
      return {
        currency: values.currency,
        amount: values.amount,
        occurred_at: toISO(values.issue_date),
        details: {
          invoice_number: values.invoice_number,
          invoice_code: values.invoice_code,
          issue_date: values.issue_date,
          issuer_name: values.issuer_name,
          recipient_name: context.order.organization_name,
          net_amount: split?.net || values.amount,
          tax_amount: split?.tax || '0',
          remark: values.remark,
        },
      }
    },
  },
  'vcc-cards': {
    file: false,
    fields: [
      { key: 'alias', type: 'text', required: true },
      { key: 'provider', type: 'text', required: true },
      { key: 'currency', type: 'currency', required: true, createOnly: true },
      { key: 'card_number', type: 'card-number', required: true, createOnly: true },
      { key: 'expiry', type: 'month', createOnly: true },
      remark,
    ],
    defaults(context) {
      return { alias: '', provider: '', currency: usualCurrency(context.items['vcc-cards'], 'USD'), card_number: '', expiry: '', remark: '' }
    },
    values(record) {
      return { alias: text(record, 'alias'), provider: text(record, 'provider'), currency: record.currency, card_number: '', expiry: '', remark: text(record, 'remark') }
    },
    payload(values, _context, record) {
      const details: Record<string, string> = { alias: values.alias, provider: values.provider, remark: values.remark }
      if (!record) {
        details.card_number = values.card_number.replace(/\D/g, '')
        if (values.expiry) details.expiry = values.expiry
      }
      return { currency: values.currency, amount: '0', details }
    },
  },
  'vcc-transactions': {
    file: false,
    fields: [
      { key: 'parent_id', type: 'card', required: true },
      { key: 'occurred_at', type: 'date', required: true },
      { key: 'amount', type: 'money', required: true },
      { key: 'merchant', type: 'text', required: true },
      { key: 'provider_transaction_id', type: 'text' },
      remark,
    ],
    defaults(context) {
      const cards = activeCards(context)
      const card = cards.length === 1 ? cards[0] : undefined
      return {
        parent_id: card?.id || '',
        occurred_at: localDate(),
        amount: '',
        merchant: lastMerchant(context),
        provider_transaction_id: '',
        transaction_type: 'CONSUME',
        original_transaction_id: '',
        remark: '',
      }
    },
    values(record) {
      return {
        parent_id: record.parent_id || '',
        occurred_at: day(record.occurred_at),
        amount: record.amount,
        merchant: text(record, 'merchant'),
        provider_transaction_id: text(record, 'provider_transaction_id'),
        transaction_type: text(record, 'transaction_type') || 'CONSUME',
        original_transaction_id: text(record, 'original_transaction_id'),
        remark: text(record, 'remark'),
      }
    },
    payload(values, context) {
      const card = context.items['vcc-cards'].find((item) => item.id === values.parent_id)
      const details: Record<string, string> = {
        transaction_type: values.transaction_type || 'CONSUME',
        merchant: values.merchant,
        provider_transaction_id: values.provider_transaction_id,
        remark: values.remark,
      }
      if (values.original_transaction_id) details.original_transaction_id = values.original_transaction_id
      return { parent_id: values.parent_id, currency: card?.currency, amount: values.amount, occurred_at: toISO(values.occurred_at), details }
    },
  },
  'vendor-invoices': {
    file: true,
    fields: [
      { key: 'vendor', type: 'text', required: true },
      { key: 'invoice_number', type: 'text' },
      { key: 'period_start', type: 'date', required: true },
      { key: 'period_end', type: 'date', required: true },
      { key: 'currency', type: 'currency', required: true },
      { key: 'amount', type: 'money', required: true },
      remark,
    ],
    defaults(context) {
      const spends = live(context.items['vcc-transactions']).filter((record) => record.details.transaction_type === 'CONSUME')
      const currency = usualCurrency(spends, usualCurrency(context.items['vcc-cards'], 'USD'))
      const covered = sum(live(context.items['vendor-invoices']), currency, (record) => record.amount)
      const first = spends[0]
      return {
        vendor: lastMerchant(context),
        invoice_number: '',
        ...monthRange(first ? new Date(first.occurred_at) : new Date()),
        currency,
        amount: remainder(sum(spends, currency, (record) => record.amount), covered),
        remark: '',
      }
    },
    values(record) {
      return {
        vendor: text(record, 'vendor'),
        invoice_number: text(record, 'invoice_number'),
        period_start: text(record, 'period_start'),
        period_end: text(record, 'period_end'),
        currency: record.currency,
        amount: record.amount,
        remark: text(record, 'remark'),
      }
    },
    payload(values, _context, record) {
      return {
        currency: values.currency,
        amount: values.amount,
        occurred_at: record?.occurred_at || new Date().toISOString(),
        details: { vendor: values.vendor, invoice_number: values.invoice_number, period_start: values.period_start, period_end: values.period_end, remark: values.remark },
      }
    },
  },
}
