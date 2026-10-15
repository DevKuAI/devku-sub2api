import type { AccountingInput, AccountingKind } from './types'
const scale = 1_000_000n
function units(raw: string): bigint {
  if (!/^-?\d+(\.\d{0,6})?$/.test(raw)) throw new Error('amount')
  const negative = raw.startsWith('-'),
    [whole, fraction = ''] = raw.replace('-', '').split('.')
  const value = BigInt(whole!) * scale + BigInt(fraction.padEnd(6, '0'))
  return negative ? -value : value
}
function value(raw: bigint): string {
  const negative = raw < 0n,
    absolute = negative ? -raw : raw
  const fraction = String(absolute % scale)
    .padStart(6, '0')
    .replace(/0+$/, '')
  return `${negative ? '-' : ''}${absolute / scale}${fraction ? `.${fraction}` : ''}`
}
export function subtractMoney(a: string, b: string) {
  try {
    return value(units(a) - units(b))
  } catch {
    return ''
  }
}
export function positiveMoney(raw: unknown) {
  try {
    return units(String(raw)) > 0n
  } catch {
    return false
  }
}
export function taxPreview(input: AccountingInput) {
  try {
    const sales = units(String(input.details.sales_amount)),
      rate = units(String(input.details.tax_rate))
    if (sales <= 0n || rate < 0n || rate > scale) return null
    const digits =
      new Intl.NumberFormat('en', {
        style: 'currency',
        currency: input.currency,
      }).resolvedOptions().maximumFractionDigits ?? 2
    const step = 10n ** BigInt(6 - digits)
    const numerator = sales * rate,
      denominator = input.details.tax_inclusive ? scale + rate : scale
    const tax =
      ((numerator + (denominator * step) / 2n) / (denominator * step)) * step
    const net = input.details.tax_inclusive ? sales - tax : sales
    const receivable = input.details.tax_inclusive ? sales : sales + tax
    return {
      tax: value(tax),
      net: value(net),
      receivable: value(receivable),
      difference: subtractMoney(input.amount, value(receivable)),
    }
  } catch {
    return null
  }
}
export const requiredFields: Partial<Record<AccountingKind, string[]>> = {
  transfers: [
    'payer_name',
    'receiving_account',
    'sales_amount',
    'tax_rate',
    'bank_reference',
  ],
  'vcc-cards': ['provider', 'alias'],
  'vcc-transactions': ['merchant', 'provider_transaction_id'],
  contracts: ['name', 'contract_number', 'party_name'],
  invoices: [
    'invoice_code',
    'invoice_number',
    'issuer_name',
    'recipient_name',
    'net_amount',
    'tax_amount',
  ],
  'vendor-invoices': ['vendor', 'period_start', 'period_end'],
  expenses: ['category', 'supplier', 'payment_account'],
  'expense-payments': [
    'payment_account',
    'transaction_number',
    'proof_attachment_id',
  ],
  receivables: ['name', 'reference', 'due_date'],
  'fx-exchanges': [
    'target_currency',
    'target_amount',
    'exchange_rate',
    'fee_amount',
    'reference',
  ],
}
