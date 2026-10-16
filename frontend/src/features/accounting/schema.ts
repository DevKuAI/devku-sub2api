import type { AccountingKind, AccountingRecord } from './types'

// Form state shared by the record form and the schema predicates.
export interface FormState {
  currency: string
  amount: string
  occurred_at: string
  parent_id: string
  details: Record<string, string | boolean>
}

export type FieldType =
  | 'text'
  | 'textarea'
  | 'money'
  | 'decimal'
  | 'date'
  | 'month'
  | 'datetime'
  | 'select'
  | 'segmented'
  | 'checkbox'
  | 'secret'
  | 'currency'
  | 'reference'
  | 'parent'
  | 'contact'
  | 'tax'
  | 'receiving-account'

export interface FieldDef {
  // Base keys (currency, amount, occurred_at, parent_id) map to the record; others to details.
  key: string
  type: FieldType
  label?: string
  required?: boolean | ((state: FormState) => boolean)
  show?: (state: FormState) => boolean
  options?: string[]
  reference?: AccountingKind
  filter?: (record: AccountingRecord, state: FormState) => boolean
  wide?: boolean
  hint?: string
  lockedOnEdit?: boolean
  // Required only when the record is first created (e.g. a card's full number, kept encrypted afterwards).
  requiredOnCreate?: boolean
}

export interface KindSchema {
  sections: { key: string; fields: FieldDef[] }[]
  // Kinds whose business time is a detail date rather than a separate input.
  occurredAtFrom?: string
  // Key figures listed first on the detail page.
  metrics: string[]
  // Remaining balance column shown in lists.
  remaining?: string
  subtitle: (record: AccountingRecord) => string[]
}

export const baseKeys = new Set(['currency', 'amount', 'occurred_at', 'parent_id'])

const remark: FieldDef = { key: 'remark', type: 'textarea', wide: true }
const currency: FieldDef = { key: 'currency', type: 'currency', required: true, lockedOnEdit: true }
const amount = (label?: string): FieldDef => ({ key: 'amount', type: 'money', required: true, label })
const occurredAt = (label?: string): FieldDef => ({ key: 'occurred_at', type: 'datetime', required: true, label })
const usable = (record: AccountingRecord) => record.status !== 'VOID'
const sameCurrency = (record: AccountingRecord, state: FormState) => usable(record) && record.currency === state.currency

export const schemas: Record<AccountingKind, KindSchema> = {
  transfers: {
    sections: [
      {
        key: 'receipt',
        fields: [
          occurredAt('fields.received_at'),
          currency,
          amount('fields.received_amount'),
          { key: 'bank_reference', type: 'text', required: true },
          { key: 'receiving_account_id', type: 'receiving-account', required: true, wide: true, lockedOnEdit: true },
        ],
      },
      {
        key: 'sales',
        fields: [
          { key: 'sales_amount', type: 'money', required: true },
          { key: 'tax_rate', type: 'tax', required: true, wide: true },
          { key: 'difference_reason', type: 'textarea', wide: true },
        ],
      },
      {
        key: 'relations',
        fields: [
          { key: 'contact_id', type: 'contact' },
          { key: 'contract_id', type: 'reference', reference: 'contracts', filter: usable },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: ['unallocated_amount', 'sales_amount', 'tax_rate', 'tax_amount', 'net_sales_amount', 'receivable_amount'],
    remaining: 'unallocated_amount',
    subtitle: (r) => [String(r.details.bank_reference || '')],
  },
  receivables: {
    sections: [
      {
        key: 'receivable',
        fields: [
          { key: 'name', type: 'text', required: true },
          { key: 'reference', type: 'text', required: true },
          currency,
          amount(),
          { key: 'due_date', type: 'date', required: true },
          occurredAt(),
        ],
      },
      {
        key: 'relations',
        fields: [
          { key: 'contract_id', type: 'reference', reference: 'contracts', filter: usable },
          { key: 'invoice_id', type: 'reference', reference: 'invoices', filter: usable },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: ['settled_amount', 'outstanding_amount'],
    remaining: 'outstanding_amount',
    subtitle: (r) => [String(r.details.reference || ''), String(r.details.due_date || '')],
  },
  'fx-exchanges': {
    sections: [
      {
        key: 'exchange',
        fields: [
          {
            key: 'parent_id',
            type: 'parent',
            reference: 'transfers',
            required: true,
            wide: true,
            label: 'fields.source_transfer',
            filter: (r) => r.status === 'RECEIVED' && r.details.funding_type !== 'FX',
          },
          amount('fields.source_amount'),
          { key: 'fee_amount', type: 'money', required: true },
          { key: 'target_currency', type: 'currency', required: true },
          { key: 'target_amount', type: 'decimal', required: true },
          { key: 'exchange_rate', type: 'decimal', required: true, hint: 'hints.exchangeRate' },
          { key: 'reference', type: 'text', required: true },
          occurredAt(),
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: ['target_amount', 'exchange_rate', 'fee_amount', 'target_available'],
    subtitle: (r) => [`→ ${r.details.target_currency || ''}`, String(r.details.reference || '')],
  },
  'vcc-cards': {
    sections: [
      {
        key: 'card',
        fields: [
          { key: 'alias', type: 'text', required: true },
          { key: 'provider', type: 'text', required: true },
          currency,
          { key: 'credit_limit', type: 'money' },
          { key: 'active_at', type: 'date' },
          { key: 'inactive_at', type: 'date' },
        ],
      },
      {
        key: 'secrets',
        fields: [
          { key: 'card_number', type: 'secret', wide: true, requiredOnCreate: true, hint: 'hints.cardNumber' },
          { key: 'cvv', type: 'secret' },
          { key: 'expiry', type: 'month' },
          { key: 'provider_token', type: 'secret', wide: true },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: ['allocated_amount', 'available_balance', 'credit_limit'],
    remaining: 'available_balance',
    subtitle: (r) => [`•••• ${r.details.last_four || ''}`, String(r.details.provider || '')],
  },
  'vcc-transactions': {
    sections: [
      {
        key: 'transaction',
        fields: [
          {
            key: 'parent_id',
            type: 'parent',
            reference: 'vcc-cards',
            required: true,
            wide: true,
            label: 'kinds.vcc-cards',
            lockedOnEdit: true,
            filter: (r, s) => r.status === 'ACTIVE' || (r.status === 'CLOSED' && ['REFUND', 'ADJUSTMENT'].includes(String(s.details.transaction_type))),
          },
          { key: 'transaction_type', type: 'segmented', options: ['CONSUME', 'REFUND', 'FEE', 'ADJUSTMENT'], required: true, wide: true },
          { key: 'direction', type: 'segmented', options: ['DEBIT', 'CREDIT'], required: true, show: (s) => s.details.transaction_type === 'ADJUSTMENT' },
          {
            key: 'original_transaction_id',
            type: 'reference',
            reference: 'vcc-transactions',
            wide: true,
            required: (s) => s.details.transaction_type === 'REFUND',
            show: (s) => s.details.transaction_type === 'REFUND' || (s.details.transaction_type === 'ADJUSTMENT' && s.details.direction === 'CREDIT'),
            filter: (r, s) => r.status === 'CONFIRMED' && r.parent_id === s.parent_id && r.details.transaction_type === 'CONSUME',
          },
          amount(),
          occurredAt(),
          { key: 'merchant', type: 'text', required: true },
          { key: 'provider_transaction_id', type: 'text', required: true },
        ],
      },
      {
        key: 'aiCost',
        fields: [
          { key: 'ai_vendor', type: 'checkbox', wide: true },
          { key: 'vendor_invoice_id', type: 'reference', reference: 'vendor-invoices', wide: true, show: (s) => s.details.ai_vendor === true, filter: usable },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: [],
    subtitle: (r) => [String(r.details.provider_transaction_id || '')],
  },
  contracts: {
    occurredAtFrom: 'signed_at',
    sections: [
      {
        key: 'contract',
        fields: [
          { key: 'name', type: 'text', required: true, wide: true },
          { key: 'contract_number', type: 'text', required: true },
          { key: 'contact_id', type: 'contact' },
          currency,
          amount('fields.contract_amount'),
          { key: 'tax_rate', type: 'tax', wide: true },
        ],
      },
      {
        key: 'validity',
        fields: [
          { key: 'signed_at', type: 'date', required: true },
          { key: 'effective_at', type: 'date' },
          { key: 'expires_at', type: 'date' },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: [],
    subtitle: (r) => [String(r.details.contract_number || ''), [r.details.effective_at, r.details.expires_at].filter(Boolean).join(' ~ ')],
  },
  invoices: {
    occurredAtFrom: 'issue_date',
    sections: [
      {
        key: 'invoice',
        fields: [
          { key: 'invoice_type', type: 'segmented', options: ['SALES', 'PURCHASE'], required: true, wide: true, lockedOnEdit: true },
          { key: 'invoice_code', type: 'text', required: true },
          { key: 'invoice_number', type: 'text', required: true },
          { key: 'issue_date', type: 'date', required: true },
          currency,
        ],
      },
      {
        key: 'parties',
        fields: [
          { key: 'issuer_name', type: 'text', required: true },
          { key: 'issuer_tax_id', type: 'text' },
          { key: 'recipient_name', type: 'text', required: true },
          { key: 'recipient_tax_id', type: 'text' },
        ],
      },
      {
        key: 'invoiceAmounts',
        fields: [
          { key: 'net_amount', type: 'money', required: true },
          { key: 'tax_amount', type: 'money', required: true },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: ['net_amount', 'tax_amount'],
    subtitle: (r) => [String(r.details.invoice_number || '')],
  },
  'vendor-invoices': {
    sections: [
      {
        key: 'vendorBill',
        fields: [
          { key: 'vendor', type: 'text', required: true },
          { key: 'invoice_number', type: 'text' },
          { key: 'period_start', type: 'date', required: true },
          { key: 'period_end', type: 'date', required: true },
          currency,
          amount(),
          occurredAt('fields.bill_date'),
        ],
      },
      {
        key: 'relations',
        fields: [
          { key: 'transaction_id', type: 'reference', reference: 'vcc-transactions', filter: (r) => r.status === 'CONFIRMED' && r.details.transaction_type === 'CONSUME' },
          { key: 'expense_id', type: 'reference', reference: 'expenses', filter: usable },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: ['allocated_cost', 'unallocated_cost'],
    remaining: 'unallocated_cost',
    subtitle: (r) => [[r.details.period_start, r.details.period_end].filter(Boolean).join(' ~ ')],
  },
  expenses: {
    sections: [
      {
        key: 'expense',
        fields: [
          { key: 'category', type: 'select', options: ['AI_VENDOR', 'CLOUD', 'SOFTWARE', 'SERVICE', 'OTHER'], required: true },
          { key: 'supplier', type: 'text', required: true },
          currency,
          amount(),
          occurredAt('fields.expense_date'),
          { key: 'payment_account', type: 'text', required: true },
        ],
      },
      {
        key: 'relations',
        fields: [
          {
            key: 'vendor_invoice_id',
            type: 'reference',
            reference: 'vendor-invoices',
            show: (s) => s.details.category === 'AI_VENDOR',
            hint: 'hints.aiInvoice',
            filter: usable,
          },
          { key: 'contract_id', type: 'reference', reference: 'contracts', filter: usable },
          { key: 'invoice_id', type: 'reference', reference: 'invoices', filter: (r) => usable(r) && r.details.invoice_type === 'PURCHASE' },
          {
            key: 'transaction_id',
            type: 'reference',
            reference: 'vcc-transactions',
            hint: 'hints.cardExpense',
            filter: (r, s) => sameCurrency(r, s) && r.status === 'CONFIRMED' && r.details.transaction_type === 'CONSUME',
          },
        ],
      },
      { key: 'notes', fields: [remark] },
    ],
    metrics: ['paid_amount', 'unpaid_amount'],
    remaining: 'unpaid_amount',
    subtitle: (r) => [String(r.details.supplier || '')],
  },
  'expense-payments': {
    sections: [],
    metrics: [],
    subtitle: (r) => [String(r.details.transaction_number || ''), String(r.details.payment_account || '')],
  },
}

export const creatableKinds: AccountingKind[] = [
  'transfers',
  'receivables',
  'fx-exchanges',
  'vcc-cards',
  'vcc-transactions',
  'contracts',
  'invoices',
  'vendor-invoices',
  'expenses',
]

// Records that can belong to an order; cards and card transactions can only be created inside one.
export const orderKinds: AccountingKind[] = ['transfers', 'invoices', 'vcc-cards', 'vcc-transactions', 'vendor-invoices']
export const orderOnlyKinds: AccountingKind[] = ['vcc-cards', 'vcc-transactions']

export const recordKinds = Object.keys(schemas) as AccountingKind[]

export function isRecordKind(value: unknown): value is AccountingKind {
  return recordKinds.includes(value as AccountingKind)
}

export const statusOptions: Record<AccountingKind, string[]> = {
  transfers: ['DRAFT', 'RECEIVED', 'VOID'],
  receivables: ['DRAFT', 'CONFIRMED', 'VOID'],
  'fx-exchanges': ['DRAFT', 'CONFIRMED', 'VOID'],
  'vcc-cards': ['ACTIVE', 'CLOSED'],
  'vcc-transactions': ['DRAFT', 'CONFIRMED', 'VOID'],
  contracts: ['DRAFT', 'SIGNED', 'EXPIRED', 'VOID'],
  invoices: ['DRAFT', 'VERIFIED', 'VOID'],
  'vendor-invoices': ['DRAFT', 'CONFIRMED', 'VOID'],
  expenses: ['DRAFT', 'UNPAID', 'PENDING', 'PAID', 'VOID'],
  'expense-payments': ['PAID', 'VOID'],
}

// Organization accounts only receive confirmed records (see customerStatuses in the backend).
export const customerStatusOptions: Partial<Record<AccountingKind, string[]>> = {
  transfers: ['RECEIVED'],
  'vcc-cards': ['ACTIVE', 'CLOSED'],
  'vcc-transactions': ['CONFIRMED'],
  contracts: ['SIGNED', 'EXPIRED'],
  invoices: ['VERIFIED'],
}

// Fields the organization-facing API returns; the admin UI never shows more to customers.
export const customerKinds: AccountingKind[] = ['transfers', 'vcc-cards', 'vcc-transactions', 'contracts', 'invoices']
export const publicFields: Partial<Record<AccountingKind, string[]>> = {
  transfers: ['sales_amount', 'tax_inclusive', 'tax_rate', 'tax_amount', 'net_sales_amount', 'receivable_amount', 'received_amount', 'original_amount', 'adjustment_amount'],
  'vcc-cards': ['alias', 'last_four', 'allocated_amount', 'available_balance'],
  'vcc-transactions': ['transaction_type', 'merchant'],
  contracts: ['name', 'contract_number', 'contact_name', 'tax_inclusive', 'tax_rate', 'signed_at', 'effective_at', 'expires_at'],
  invoices: ['invoice_type', 'invoice_code', 'invoice_number', 'issue_date', 'issuer_name', 'issuer_tax_id', 'recipient_name', 'recipient_tax_id', 'net_amount', 'tax_amount', 'total_amount'],
}

export function recordLabel(details: Record<string, unknown>, fallback: string) {
  return String(
    details.alias ||
      details.name ||
      details.payer_name ||
      details.merchant ||
      details.vendor ||
      details.supplier ||
      details.invoice_number ||
      details.transaction_number ||
      details.reference ||
      fallback,
  )
}

// Secondary line under a record's name; parts repeating the name itself are dropped.
export function recordSubtitle(kind: AccountingKind, record: AccountingRecord) {
  const label = recordLabel(record.details, record.id)
  return schemas[kind].subtitle(record).filter((part) => part && part !== label)
}

export function effectiveAmount(record: AccountingRecord) {
  return String(record.details.effective_amount || record.amount)
}

// Primary list/detail amount: cards show their balance, everything else the effective amount.
export function displayAmount(kind: AccountingKind, record: AccountingRecord) {
  return kind === 'vcc-cards' ? String(record.details.available_balance || '0') : effectiveAmount(record)
}
