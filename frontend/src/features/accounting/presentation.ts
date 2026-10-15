import type { AccountingKind, Field } from './types'
import { fields } from './fields'

export const fieldGroups: Record<
  AccountingKind,
  { key: string; fields: Field[] }[]
> = Object.fromEntries(
  Object.entries({
    receivables: [
      ['receivable', 'name reference due_date'],
      ['related', 'contract_id invoice_id'],
      ['notes', 'remark'],
    ],
    'fx-exchanges': [
      [
        'exchange',
        'target_currency target_amount exchange_rate fee_amount reference',
      ],
      ['notes', 'remark'],
    ],
    transfers: [
      ['receipt', 'payer_name receiving_account bank_reference'],
      ['contacts', 'contact_name contact_phone contact_email contact_wechat'],
      ['sales', 'sales_amount tax_inclusive tax_rate difference_reason'],
      ['notes', 'remark'],
    ],
    'vcc-cards': [
      ['card', 'provider alias last_four credit_limit'],
      ['validity', 'active_at inactive_at'],
      ['secrets', 'card_number cvv expiry provider_token'],
      ['notes', 'remark'],
    ],
    'vcc-transactions': [
      [
        'transaction',
        'transaction_type direction merchant provider_transaction_id',
      ],
      ['related', 'original_transaction_id ai_vendor vendor_invoice_id'],
      ['notes', 'remark'],
    ],
    contracts: [
      ['contract', 'name contract_number party_name'],
      ['tax', 'tax_inclusive tax_rate'],
      ['validity', 'signed_at effective_at expires_at'],
      ['notes', 'remark'],
    ],
    invoices: [
      ['invoice', 'invoice_type invoice_code invoice_number issue_date'],
      ['parties', 'issuer_name issuer_tax_id recipient_name recipient_tax_id'],
      ['invoiceAmounts', 'net_amount tax_amount'],
      ['notes', 'remark'],
    ],
    'vendor-invoices': [
      ['vendorBill', 'vendor invoice_number period_start period_end'],
      ['related', 'transaction_id expense_id'],
      ['notes', 'remark'],
    ],
    expenses: [
      ['expense', 'category payment_account supplier contract_number'],
      [
        'related',
        'contract_id ai_vendor vendor_invoice_id invoice_id transaction_id',
      ],
      ['notes', 'remark'],
    ],
    'expense-payments': [
      ['payment', 'payment_account transaction_number proof_attachment_id'],
      ['overpayment', 'override reason'],
      ['notes', 'remark'],
    ],
  }).map(([kind, groups]) => [
    kind,
    groups.map(([key, keys]) => ({
      key,
      fields: keys
        .split(' ')
        .map(
          (name) =>
            fields[kind as AccountingKind].find((field) => field.key === name)!,
        ),
    })),
  ]),
) as Record<AccountingKind, { key: string; fields: Field[] }[]>

export const navigationGroups = [
  { key: 'overview', tabs: ['overview', 'workbench'] },
  {
    key: 'funds',
    tabs: ['transfers', 'vcc-cards', 'vcc-transactions', 'fx-exchanges'],
  },
  {
    key: 'documents',
    tabs: ['receivables', 'contracts', 'invoices', 'vendor-invoices', 'costs'],
  },
  {
    key: 'settlement',
    tabs: [
      'expenses',
      'expense-payments',
      'reconciliation',
      'external',
      'periods',
      'audit',
    ],
  },
]
