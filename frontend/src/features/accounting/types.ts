export type AccountingKind =
  | 'receivables'
  | 'fx-exchanges'
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
export type OrganizationContactRole =
  | 'BUSINESS'
  | 'FINANCE'
  | 'LEGAL'
  | 'TECHNICAL'
  | 'OTHER'
export interface OrganizationContact {
  id: string
  organization_id: string
  name: string
  role: OrganizationContactRole
  phone: string
  email: string
  wechat: string
  remark: string
  is_primary: boolean
  created_at: string
  updated_at: string
}
export interface OrganizationContactInput {
  organization_id: string
  name: string
  role: OrganizationContactRole
  phone: string
  email: string
  wechat: string
  remark: string
  is_primary: boolean
}
export interface ReceivingAccount {
  id: string
  currency: string
  account_name: string
  bank_name: string
  bank_account: string
  bank_branch: string
  remark: string
  created_at: string
  updated_at: string
}
export interface ReceivingAccountInput {
  currency: string
  account_name: string
  bank_name: string
  bank_account: string
  bank_branch: string
  remark: string
}
export interface AllocationRow {
  id: string
  transfer_id: string
  card_id: string
  amount: string
  used_amount: string
  released_amount: string
  closed: boolean
  card_name?: string
  transfer_name?: string
  funding_type?: string
}
export interface RelatedRow {
  kind: AccountingKind
  id: string
  name: string
  amount: string
  currency: string
  status: string
}
export interface ActivityRow {
  id: string
  action: string
  actor_id: number
  request_id: string
  created_at: string
  summary?: Record<string, unknown>
}
export interface SettlementRow {
  id: string
  transfer_id: string
  reference: string
  amount: string
  currency: string
  status: string
}

export interface WorkflowRow {
  [key: string]: unknown
  id: string
}
export interface WorkflowTodo {
  kind: AccountingKind | 'external-actions'
  id: string
  name: string
  reason: string
  amount: string
  currency: string
  status: string
}
export interface BookPosition {
  cash: string
  card: string
}
export interface Workbench {
  items: WorkflowTodo[]
  total: number
  matched_total?: number
  counts: Record<string, number>
  positions: Record<string, BookPosition>
  pending_consumption: { currency: string; amount: string }[]
}
export interface Statement {
  month: string
  covered_from: string
  opening: Record<string, BookPosition> | null
  movement: Record<string, BookPosition>
  closing: Record<string, BookPosition>
  records: { kind: AccountingKind; record: AccountingRecord }[]
}

export type OrderStep = 'receipt' | 'invoice' | 'purchase' | 'ai_invoice'
export type OrderItemKind = 'transfers' | 'invoices' | 'vcc-cards' | 'vcc-transactions' | 'vendor-invoices'
export interface OrderTotal {
  currency: string
  received: string
  invoiced: string
  spent: string
  ai_cost: string
}
export interface AccountingOrder {
  id: string
  organization_id: string
  organization_name: string
  name: string
  remark: string
  status: 'OPEN' | 'CLOSED'
  state: 'OPEN' | 'COMPLETED' | 'CLOSED'
  close_reason: string
  closed_at?: string
  created_at: string
  version: number
  progress: Record<OrderStep, boolean>
  totals: OrderTotal[]
  items?: Partial<Record<OrderItemKind, AccountingRecord[]>>
}
export interface OrderFilter {
  organization_id?: string
  state?: string
  search?: string
  page?: number
  page_size?: number
}
// One order entry as sent to the order page endpoints; the order sets the organization.
export interface OrderEntryInput {
  kind?: OrderItemKind
  parent_id?: string
  currency?: string
  amount?: string
  occurred_at?: string
  details: AccountingDetails
  version?: number
}
