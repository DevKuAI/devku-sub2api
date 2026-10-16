import { positiveMoney } from './money'
import type { AccountingKind, AccountingRecord } from './types'

// "form" actions open their own page; "confirm" actions only need a confirmation dialog.
export interface RecordAction {
  key: string
  mode: 'form' | 'confirm' | 'create' | 'edit' | 'share'
  tone: 'primary' | 'secondary' | 'danger'
  createKind?: AccountingKind
}

const confirmPaths: Record<string, string> = { confirm: 'confirm', close: 'close', expire: 'expire' }
export function confirmPath(kind: AccountingKind, id: string, action: string) {
  return `${kind}/${id}/${confirmPaths[action]}`
}

export function recordActions(kind: AccountingKind, record: AccountingRecord, administrator: boolean): RecordAction[] {
  const { status, details } = record
  // System-created funding (FX target funds, card top-ups) is not a customer receipt and cannot be reworked.
  const internalFunding = Boolean(details.funding_type)
  const cardMapped = kind === 'expenses' && Boolean(details.transaction_id)
  const actions: RecordAction[] = []
  const add = (key: string, mode: RecordAction['mode'], tone: RecordAction['tone'] = 'secondary', createKind?: AccountingKind) =>
    actions.push({ key, mode, tone, createKind })

  if (status === 'DRAFT' && kind !== 'expense-payments') add('confirm', 'confirm', 'primary')
  if (kind === 'transfers' && status === 'RECEIVED' && positiveMoney(details.unallocated_amount)) {
    add('allocate', 'form', 'primary')
    if (!internalFunding) add('exchange', 'create', 'secondary', 'fx-exchanges')
  }
  if (kind === 'receivables' && status === 'CONFIRMED' && positiveMoney(details.outstanding_amount)) add('settle', 'form', 'primary')
  if (kind === 'fx-exchanges' && status === 'CONFIRMED' && positiveMoney(details.target_available)) add('fx-allocate', 'form', 'primary')
  if (kind === 'vendor-invoices' && status === 'CONFIRMED' && positiveMoney(details.unallocated_cost)) add('cost', 'form', 'primary')
  if (kind === 'expenses' && !cardMapped && (['UNPAID', 'PENDING'].includes(status) || (administrator && status === 'PAID')))
    add('pay', 'form', status === 'PAID' ? 'secondary' : 'primary')
  // Card spend is recorded inside orders; on the card itself we only load our own funds.
  if (kind === 'vcc-cards' && status === 'ACTIVE') add('top-up', 'form', 'primary')

  if ((status === 'DRAFT' && kind !== 'expense-payments') || (kind === 'vcc-cards' && status === 'ACTIVE')) add('edit', 'edit')
  if (kind === 'contracts' || (kind === 'invoices' && details.invoice_type === 'SALES')) add(record.customer_visible ? 'unshare' : 'share', 'share')
  if (['transfers', 'expenses', 'receivables'].includes(kind) && !['DRAFT', 'VOID'].includes(status) && !internalFunding && !cardMapped)
    add('adjust', 'form')
  if (kind === 'contracts' && status === 'SIGNED') add('expire', 'confirm')

  if (kind === 'vcc-cards' && status === 'ACTIVE') add('close', 'confirm', 'danger')
  if (status !== 'VOID' && kind !== 'vcc-cards' && !internalFunding) add('void', 'form', 'danger')
  return actions
}

export function statusTone(status: string) {
  if (['RECEIVED', 'CONFIRMED', 'SIGNED', 'VERIFIED', 'PAID', 'ACTIVE', 'MATCHED'].includes(status)) return 'badge-success'
  if (['UNPAID', 'PENDING', 'DIFFERENCE', 'UNMATCHED'].includes(status)) return 'badge-warning'
  if (status === 'VOID') return 'badge-danger'
  return 'badge-gray'
}
