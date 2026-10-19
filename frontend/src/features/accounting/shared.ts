import type { RouteLocationRaw } from 'vue-router'
import { formatMoney } from './format'
import { baseKeys, schemas, publicFields } from './schema'
import type { AccountingKind, AccountingRecord } from './types'

type Translate = (key: string, values?: Record<string, unknown>) => string
type Exists = (key: string) => boolean

const reasonMessages: Record<string, string> = {
  ACCOUNTING_PERIOD_CLOSED: 'errors.periodClosed',
  ACCOUNTING_AUTHORIZATION_REQUIRED: 'errors.adminRequired',
  ACCOUNTING_INVALID: 'errors.validation',
  ACCOUNTING_STATE_CONFLICT: 'errors.stateConflict',
  ACCOUNTING_PROOF_REQUIRED: 'errors.proofRequired',
  ACCOUNTING_STORAGE_UNAVAILABLE: 'errors.storageUnavailable',
  ACCOUNTING_AMOUNT_CONFLICT: 'errors.amountConflict',
  ACCOUNTING_CURRENCY_MISMATCH: 'errors.currencyMismatch',
  IDEMPOTENCY_KEY_CONFLICT: 'errors.keyConflict',
}

// One mapping for every accounting API error so pages show the same wording.
export function errorMessage(cause: unknown, t: Translate, te: Exists) {
  const error = cause as { reason?: string; code?: string; message?: string; metadata?: { field?: string } }
  const field = error.metadata?.field
  if (field && te(`accounting.errors.fields.${field}`)) return t(`accounting.errors.fields.${field}`)
  if (error.reason === 'ACCOUNTING_INVALID' && field && te(`accounting.fields.${field}`))
    return t('accounting.errors.invalidField', { field: t(`accounting.fields.${field}`) })
  if (error.reason && reasonMessages[error.reason]) return t(`accounting.${reasonMessages[error.reason]}`)
  if (['ERR_NETWORK', 'ECONNABORTED', 'ETIMEDOUT'].includes(error.code || '') || error.message === 'Network Error')
    return t('accounting.errors.network')
  return error.message || t('accounting.errors.generic')
}

export function isCanceled(cause: unknown) {
  return Boolean((cause as { code?: string }).code?.includes('CANCELED'))
}

export function localDateTime(value: string | Date = new Date()) {
  const date = new Date(value)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}

export function localDate(value: Date = new Date()) {
  return localDateTime(value).slice(0, 10)
}

export function toISO(local: string) {
  return new Date(local.length === 10 ? `${local}T00:00` : local).toISOString()
}

export function formatDateTime(value: string) {
  return value ? new Date(value).toLocaleString() : ''
}

export function formatDate(value: string) {
  return value ? new Date(value).toLocaleDateString() : ''
}

export function currencyDigits(currency: string) {
  try {
    return new Intl.NumberFormat('en', { style: 'currency', currency }).resolvedOptions().maximumFractionDigits ?? 2
  } catch {
    return 2
  }
}

export function moneyStep(currency: string) {
  const digits = currencyDigits(currency)
  return digits === 0 ? '1' : `0.${'0'.repeat(digits - 1)}1`
}

export function formatPercent(rate: unknown) {
  const value = Number(rate)
  if (rate === '' || rate === undefined || !Number.isFinite(value)) return ''
  return `${Number((value * 100).toFixed(4))}%`
}

export const commonCurrencies = ['CNY', 'USD', 'HKD', 'EUR', 'GBP', 'JPY', 'SGD']

// --- Routes ---------------------------------------------------------------

export function workspaceRoute(organizationId: string, tab?: string): RouteLocationRaw {
  return { name: 'AdminOrganizationAccounting', params: { organizationId }, query: tab ? { tab } : {} }
}

export function recordRoute(organizationId: string, kind: AccountingKind, id: string): RouteLocationRaw {
  return { name: 'AdminAccountingRecord', params: { organizationId, kind, id } }
}

export function createRoute(organizationId: string, kind: AccountingKind, query: Record<string, string> = {}): RouteLocationRaw {
  return { name: 'AdminAccountingRecordNew', params: { organizationId, kind }, query }
}

export function editRoute(organizationId: string, kind: AccountingKind, id: string): RouteLocationRaw {
  return { name: 'AdminAccountingRecordEdit', params: { organizationId, kind, id } }
}

export function recordActionRoute(organizationId: string, kind: AccountingKind, id: string, action: string): RouteLocationRaw {
  return { name: 'AdminAccountingRecordAction', params: { organizationId, kind, id, action } }
}

export function orderRoute(organizationId: string, orderId: string): RouteLocationRaw {
  return { name: 'AdminAccountingOrder', params: { organizationId, orderId } }
}

export function workflowRoute(organizationId: string, action: string, targetId?: string, query: Record<string, string> = {}): RouteLocationRaw {
  return { name: 'AdminAccountingWorkflow', params: { organizationId, action, ...(targetId ? { targetId } : {}) }, query }
}

// Only same-app accounting paths are accepted as return targets.
export function safeReturnPath(value: unknown) {
  return typeof value === 'string' && value.startsWith('/admin/') && !value.startsWith('//') ? value : ''
}

// --- Detail display -------------------------------------------------------

const moneyKeys = new Set([
  'sales_amount', 'tax_amount', 'net_sales_amount', 'receivable_amount', 'received_amount', 'credit_limit',
  'fee_amount', 'net_amount', 'total_amount', 'unallocated_amount', 'allocated_amount', 'available_balance',
  'paid_amount', 'unpaid_amount', 'settled_amount', 'outstanding_amount', 'allocated_cost', 'unallocated_cost',
  'original_amount', 'adjustment_amount', 'effective_amount', 'overpaid_amount',
])
const targetMoneyKeys = new Set(['target_amount', 'target_available'])
const enumKeys = new Set(['transaction_type', 'direction', 'invoice_type', 'category', 'funding_type'])
const hiddenKeys = new Set([
  'card_number', 'cvv', 'expiry', 'provider_token', 'payer_name', 'tax_inclusive', 'party_name',
  'contact_id', 'receiving_account_id', 'receiving_account', 'receiving_account_name', 'receiving_bank_name',
  'receiving_bank_branch', 'contract_id', 'invoice_id', 'vendor_invoice_id', 'transaction_id', 'expense_id',
  'original_transaction_id', 'target_transfer_id', 'source_exchange_id', 'proof_attachment_id', 'document_type',
  'received_amount', 'effective_amount', 'contact_phone', 'contact_email', 'contact_wechat', 'has_difference',
  'order_id', 'order_name', 'card_id',
  'reconciliation_status',
])
// Adjustment bookkeeping is only worth showing once an adjustment exists.
const zeroHidden = new Set(['adjustment_amount', 'overpaid_amount', 'original_amount'])

export interface DisplayField {
  key: string
  label: string
  value: string
  wide?: boolean
}

export function formatDetail(key: string, value: unknown, record: AccountingRecord, t: Translate, te: Exists) {
  if (value === '' || value === undefined || value === null) return ''
  if (typeof value === 'boolean') return t(value ? 'common.yes' : 'common.no')
  if (moneyKeys.has(key)) return formatMoney(String(value), record.currency)
  if (targetMoneyKeys.has(key)) return formatMoney(String(value), String(record.details.target_currency || record.currency))
  if (key === 'tax_rate') {
    const basis = record.details.tax_inclusive === true ? 'taxInclusive' : 'taxExclusive'
    return `${formatPercent(value)} · ${t(`accounting.form.${basis}`)}`
  }
  if (enumKeys.has(key) && te(`accounting.options.${key}.${value}`)) return t(`accounting.options.${key}.${value}`)
  return String(value)
}

// Notes read best after the figures they explain.
const trailingKeys = ['difference_reason', 'void_reason', 'remark']

// Ordered, translated details for a record; customers only receive their public subset.
export function detailFields(kind: AccountingKind, record: AccountingRecord, customer: boolean, t: Translate, te: Exists): DisplayField[] {
  const schema = schemas[kind]
  const ordered = [
    ...schema.metrics,
    ...schema.sections.flatMap((section) => section.fields.map((field) => field.key)),
    'contact_name',
    'contact_phone',
    'contact_email',
    ...Object.keys(record.details),
  ].filter((key) => !trailingKeys.includes(key))
  const seen = new Set<string>()
  const fields: DisplayField[] = []
  for (const key of [...ordered, ...trailingKeys]) {
    if (seen.has(key)) continue
    seen.add(key)
    if (key === 'receiving_account_id' && !customer && record.details.receiving_account) {
      fields.push({
        key: 'receiving_account',
        label: t('accounting.fields.receiving_account'),
        value: [record.details.receiving_account_name, record.details.receiving_bank_name, record.details.receiving_account].filter(Boolean).join(' · '),
        wide: true,
      })
      continue
    }
    if (baseKeys.has(key) || hiddenKeys.has(key)) continue
    if (customer && !publicFields[kind]?.includes(key)) continue
    if (zeroHidden.has(key) && !Number(record.details[key === 'original_amount' ? 'adjustment_amount' : key])) continue
    const value = formatDetail(key, record.details[key], record, t, te)
    if (!value || !te(`accounting.fields.${key}`)) continue
    fields.push({ key, label: t(`accounting.fields.${key}`), value, wide: trailingKeys.includes(key) })
  }
  return fields
}
