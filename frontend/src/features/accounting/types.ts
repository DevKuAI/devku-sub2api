export type AccountingKind =
  | 'transfers'
  | 'vcc-cards'
  | 'vcc-transactions'
  | 'contracts'
  | 'invoices'
  | 'vendor-invoices'
  | 'expenses'
  | 'expense-payments'
export type AccountingDetails = Record<string, string | boolean>
export interface AccountingRecord {
  id: string
  organization_id: string
  parent_id?: string
  status: string
  currency: string
  amount: string
  occurred_at: string
  details: AccountingDetails
  customer_visible: boolean
  version: number
  created_at: string
}
export interface AccountingInput {
  organization_id: string
  parent_id?: string
  currency: string
  amount: string
  occurred_at: string
  details: AccountingDetails
  version?: number
}
export interface CurrencySummary {
  currency: string
  received: string
  allocated: string
  consumed: string
  refunded: string
  balance: string
  unallocated: string
  unmatched: number
  unmatched_amount?: string
  expenses?: string
}
export interface AccountingOrganization {
  id: string
  name: string
  summary: CurrencySummary[]
}
export interface AccountingAttachment {
  id: string
  object_kind: AccountingKind
  object_id: string
  filename: string
  content_type: string
  size_bytes: number
  expires_at: string
  sha256: string
  status: string
  customer_visible: boolean
}
export interface AccountingFilter {
  organization_id?: string
  currency?: string
  status?: string
  search?: string
  from?: string
  to?: string
  page?: number
  page_size?: number
  card_id?: string
}
export interface ReconciliationRow {
  object_kind: AccountingKind
  record: AccountingRecord
  status: string
  reason: string
  reference: string
}
export interface AuditRow {
  id: number
  action: string
  object_kind: string
  object_id: string
  actor_id: number
  request_id: string
  created_at: string
}
export interface Field {
  key: string
  type?: 'attachment' | 'boolean' | 'date' | 'reference' | 'secret' | 'select'
  options?: string[]
  reference?: AccountingKind
  required?: boolean
}
