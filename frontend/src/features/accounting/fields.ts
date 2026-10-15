import type { AccountingKind, Field } from './types'
const plain = (keys: string): Field[] => keys.split(' ').map((key) => ({ key }))
const ref = (key: string, reference: AccountingKind): Field => ({
  key,
  type: 'reference',
  reference,
})
export const fields: Record<AccountingKind, Field[]> = {
  receivables: [
    ...plain('name reference'),
    { key: 'due_date', type: 'date' },
    ref('contract_id', 'contracts'),
    ref('invoice_id', 'invoices'),
    { key: 'remark' },
  ],
  'fx-exchanges': [
    ...plain(
      'target_currency target_amount exchange_rate fee_amount reference remark',
    ),
  ],
  transfers: [
    ...plain(
      'payer_name contact_name contact_phone contact_email contact_wechat receiving_account sales_amount',
    ),
    { key: 'tax_inclusive', type: 'boolean' },
    { key: 'tax_rate' },
    ...plain('difference_reason bank_reference remark'),
  ],
  'vcc-cards': [
    ...plain('provider alias last_four credit_limit'),
    { key: 'active_at', type: 'date' },
    { key: 'inactive_at', type: 'date' },
    ...['card_number', 'cvv', 'expiry', 'provider_token'].map((key) => ({
      key,
      type: 'secret' as const,
    })),
    { key: 'remark' },
  ],
  'vcc-transactions': [
    {
      key: 'transaction_type',
      type: 'select',
      options: ['CONSUME', 'REFUND', 'FEE', 'ADJUSTMENT'],
    },
    { key: 'direction', type: 'select', options: ['DEBIT', 'CREDIT'] },
    ...plain('merchant provider_transaction_id'),
    ref('original_transaction_id', 'vcc-transactions'),
    { key: 'ai_vendor', type: 'boolean' },
    ref('vendor_invoice_id', 'vendor-invoices'),
    { key: 'remark' },
  ],
  contracts: [
    ...plain('name contract_number party_name'),
    { key: 'tax_inclusive', type: 'boolean' },
    { key: 'tax_rate' },
    ...['signed_at', 'effective_at', 'expires_at'].map((key) => ({
      key,
      type: 'date' as const,
    })),
    { key: 'remark' },
  ],
  invoices: [
    { key: 'invoice_type', type: 'select', options: ['SALES', 'PURCHASE'] },
    ...plain('invoice_code invoice_number'),
    { key: 'issue_date', type: 'date' },
    ...plain(
      'issuer_name issuer_tax_id recipient_name recipient_tax_id net_amount tax_amount remark',
    ),
  ],
  'vendor-invoices': [
    { key: 'vendor' },
    { key: 'period_start', type: 'date' },
    { key: 'period_end', type: 'date' },
    { key: 'invoice_number' },
    ref('transaction_id', 'vcc-transactions'),
    ref('expense_id', 'expenses'),
    { key: 'remark' },
  ],
  expenses: [
    ...plain('category payment_account supplier contract_number'),
    ref('contract_id', 'contracts'),
    { key: 'ai_vendor', type: 'boolean' },
    ref('vendor_invoice_id', 'vendor-invoices'),
    ref('invoice_id', 'invoices'),
    ref('transaction_id', 'vcc-transactions'),
    { key: 'remark' },
  ],
  'expense-payments': [
    ...plain('payment_account transaction_number'),
    { key: 'proof_attachment_id', type: 'attachment' },
    { key: 'override', type: 'boolean' },
    ...plain('reason remark'),
  ],
}
export const adminKinds = Object.keys(fields) as AccountingKind[]
export const customerKinds: AccountingKind[] = [
  'transfers',
  'vcc-cards',
  'vcc-transactions',
  'contracts',
  'invoices',
]
export function recordLabel(
  details: Record<string, string | boolean>,
  fallback: string,
) {
  return String(
    details.alias ||
      details.name ||
      details.payer_name ||
      details.merchant ||
      details.vendor ||
      details.supplier ||
      details.invoice_number ||
      details.reference ||
      fallback,
  )
}
export const publicFields: Record<string, string[]> = {
  transfers: [
    'payer_name',
    'sales_amount',
    'tax_inclusive',
    'tax_rate',
    'tax_amount',
    'net_sales_amount',
    'receivable_amount',
    'received_amount',
    'original_amount',
    'adjustment_amount',
  ],
  'vcc-cards': ['alias', 'last_four', 'allocated_amount', 'available_balance'],
  'vcc-transactions': [
    'transaction_type',
    'merchant',
    'original_transaction_id',
  ],
  contracts: [
    'name',
    'contract_number',
    'party_name',
    'tax_inclusive',
    'tax_rate',
    'signed_at',
    'effective_at',
    'expires_at',
  ],
  invoices: [
    'invoice_type',
    'invoice_code',
    'invoice_number',
    'issue_date',
    'issuer_name',
    'issuer_tax_id',
    'recipient_name',
    'recipient_tax_id',
    'net_amount',
    'tax_amount',
    'total_amount',
  ],
}
