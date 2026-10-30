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

// Channel accounts: our own money at payment and card platforms, booked from their statements.
export type ChannelLineType =
  | 'FUNDING_IN'
  | 'FUNDING_OUT'
  | 'TRANSFER_IN'
  | 'TRANSFER_OUT'
  | 'FX_IN'
  | 'FX_OUT'
  | 'CONSUME'
  | 'REFUND'
  | 'FEE'
  | 'ADJUSTMENT'
export interface ChannelAccount {
  id: string
  platform: string
  account_ref: string
  name: string
  remark: string
  created_at: string
}
export interface ChannelAmount {
  currency: string
  amount: string
}
export interface ChannelReported {
  amount: string
  as_of: string
  source: string
  import_id: string
  books: string
  difference: string
  matched: boolean
}
export interface ChannelGap {
  after_external_id: string
  after_at: string
  before_external_id: string
  before_at: string
  expected_balance: string
  reported_balance: string
  difference: string
}
export interface ChannelSpan {
  from: string
  to?: string
  lowest: string
}
export interface ChannelWallet {
  wallet: string
  currency: string
  opening: string
  balance: string
  lines: number
  first_at: string
  last_at: string
  reported?: ChannelReported
  gaps: ChannelGap[]
  negative_spans: ChannelSpan[]
}
export interface ChannelAccountSummary {
  account: ChannelAccount
  wallets: ChannelWallet[]
  positions: ChannelAmount[]
  fee_costs: ChannelAmount[]
  unpaired_transfers: number
  missing_proofs: number
  cards: number
  last_import_at?: string
}
export interface ChannelMonth {
  month: string
  currency: string
  totals: Partial<Record<ChannelLineType, string>>
  fee_cost: string
}
export interface ChannelConversion {
  at: string
  reference: string
  sold: string
  sold_currency: string
  bought: string
  bought_currency: string
  rate: string
}
export interface ChannelCardSpend {
  id: string
  card_ref: string
  card_last4: string
  card_name: string
  currency: string
  spends: number
  spend: string
  refund: string
  last_at?: string
}
export interface ChannelLineRef {
  id: string
  wallet: string
  currency: string
  external_id: string
  amount: string
}
export interface ChannelLine {
  id: string
  wallet: string
  external_id: string
  occurred_at: string
  type: ChannelLineType
  amount: string
  currency: string
  balance_after?: string
  fee_amount?: string
  card_ref?: string
  card_last4?: string
  card_name?: string
  merchant?: string
  related_id?: string
  original_amount?: string
  original_currency?: string
  remark?: string
  booking: string
  import_id: string
  pair?: ChannelLineRef
  proofs: ChannelProof[]
}
// Evidence kept for a channel line, such as a transfer screenshot for a collection.
export interface ChannelProof {
  id: string
  filename: string
  content_type: string
  size: number
  created_by: number
  created_at: string
}
export interface ChannelBalance {
  platform: string
  account_ref: string
  wallet: string
  currency: string
  amount: string
  as_of: string
  source: string
}
export interface ChannelFile {
  name: string
  sha256: string
  size: number
}
export interface ChannelImport {
  id: string
  converter: string
  files: ChannelFile[]
  reported_balances: ChannelBalance[]
  total_rows: number
  new_rows: number
  duplicate_rows: number
  pending_rows: number
  status: 'APPLIED' | 'REVOKED'
  account_ids: string[]
  created_by: number
  created_at: string
  revoked_at?: string
  revoke_reason?: string
}
export interface ChannelAccountDetail extends ChannelAccountSummary {
  months: ChannelMonth[]
  conversions: ChannelConversion[]
  card_spend: ChannelCardSpend[]
  unpaired: ChannelLine[]
  imports: ChannelImport[]
}
export interface ChannelImportResult {
  import: ChannelImport
  accounts: ChannelAccountSummary[]
}
export interface ChannelConversionResult {
  converter: string
  lines: number
  balances: ChannelBalance[]
  csv: string
}
export interface ChannelLineFilter {
  wallet?: string
  currency?: string
  type?: string
  card_id?: string
  from?: string
  to?: string
  search?: string
  page?: number
  page_size?: number
}
// One reason an upload was rejected, at a file row and column when the server could locate it.
export interface ChannelProblem {
  file?: string
  row?: number
  column?: string
  message: string
}
