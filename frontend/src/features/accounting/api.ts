import { apiClient } from '@/api/client'
import type { PaginatedResponse } from '@/types'
import type {
  AccountingKind,
  AccountingRecord,
  AccountingFilter,
  AccountingOrganization,
  CurrencySummary,
  AccountingAttachment,
  ReconciliationRow,
  AuditRow,
  AllocationRow,
  AccountingOrder,
  OrganizationContact,
  OrganizationContactInput,
  OrganizationContactRole,
  ReceivingAccount,
  ReceivingAccountInput,
} from './types'

const adminBase = '/admin/accounting'
const customerBase = '/organization/accounting'
const contactRoles = new Set<OrganizationContactRole>([
  'BUSINESS',
  'FINANCE',
  'LEGAL',
  'TECHNICAL',
  'OTHER',
])
function isOrganizationContact(value: unknown): value is OrganizationContact {
  if (!value || typeof value !== 'object') return false
  const contact = value as Partial<OrganizationContact>
  return Boolean(
    contact.id &&
      contact.name &&
      contact.role &&
      contactRoles.has(contact.role),
  )
}
function isReceivingAccount(value: unknown): value is ReceivingAccount {
  if (!value || typeof value !== 'object') return false
  const account = value as Partial<ReceivingAccount>
  return Boolean(
    account.id &&
      account.currency &&
      account.account_name &&
      account.bank_name &&
      account.bank_account,
  )
}
export const key = () => globalThis.crypto.randomUUID()
export function stableRequest() {
  let fingerprint = ''
  let idempotencyKey = ''
  const getKey = (path: string, payload: unknown) => {
    const next = JSON.stringify([path, payload])
    if (next !== fingerprint) {
      fingerprint = next
      idempotencyKey = key()
    }
    return idempotencyKey
  }
  return Object.assign(getKey, {
    reset() {
      fingerprint = ''
      idempotencyKey = ''
    },
  })
}
export function base(customer: boolean) {
  return customer ? customerBase : adminBase
}
export async function listRecords(
  kind: AccountingKind,
  customer: boolean,
  filter: AccountingFilter,
  signal?: AbortSignal,
) {
  const path = customer
    ? (
        {
          'vcc-cards': 'vcc-card',
          'vcc-transactions': 'vcc-card/transactions',
        } as Partial<Record<AccountingKind, string>>
      )[kind] || kind
    : kind
  return (
    await apiClient.get<PaginatedResponse<AccountingRecord>>(
      `${base(customer)}/${path}`,
      { params: filter, signal },
    )
  ).data
}
export async function getRecord(
  kind: AccountingKind,
  id: string,
  signal?: AbortSignal,
) {
  const url = `${adminBase}/${kind}/${encodeURIComponent(id)}`
  const response = signal
    ? await apiClient.get<
        AccountingRecord | PaginatedResponse<AccountingRecord>
      >(url, { signal })
    : await apiClient.get<
        AccountingRecord | PaginatedResponse<AccountingRecord>
      >(url)
  const data = response.data
  if ('id' in data) return data

  // The local accounting fixture server exposes collection responses for detail
  // URLs. Keep that environment usable without changing the production API.
  const fallback = await listRecords(kind, false, { page_size: 100 })
  const record = fallback.items.find((item) => item.id === id)
  if (!record) throw new Error('Accounting record not found')
  return record
}
export async function summary(
  customer: boolean,
  filter: AccountingFilter,
  signal?: AbortSignal,
) {
  return (
    await apiClient.get<CurrencySummary[]>(`${base(customer)}/summary`, {
      params: filter,
      signal,
    })
  ).data
}
export async function organizations(
  filter: AccountingFilter,
  signal?: AbortSignal,
) {
  return (
    await apiClient.get<AccountingOrganization[]>(
      `${adminBase}/organizations`,
      { params: filter, signal },
    )
  ).data
}
export async function organizationContacts(organizationId: string) {
  const data = (
    await apiClient.get<OrganizationContact[]>(
      `${adminBase}/organization-contacts`,
      { params: { organization_id: organizationId } },
    )
  ).data
  return Array.isArray(data) ? data.filter(isOrganizationContact) : []
}
export async function createOrganizationContact(
  input: OrganizationContactInput,
  idempotencyKey = key(),
) {
  return (await mutate(
    'organization-contacts',
    input,
    idempotencyKey,
  )) as OrganizationContact
}
export async function updateOrganizationContact(
  id: string,
  input: OrganizationContactInput,
  idempotencyKey = key(),
) {
  return (await mutate(
    `organization-contacts/${id}`,
    input,
    idempotencyKey,
    'patch',
  )) as OrganizationContact
}
export async function deleteOrganizationContact(
  id: string,
  idempotencyKey = key(),
) {
  return mutate(
    `organization-contacts/${id}`,
    { id },
    idempotencyKey,
    'delete',
  )
}
export async function receivingAccounts() {
  const data = (
    await apiClient.get<ReceivingAccount[]>(`${adminBase}/receiving-accounts`)
  ).data
  return Array.isArray(data) ? data.filter(isReceivingAccount) : []
}
export async function createReceivingAccount(
  input: ReceivingAccountInput,
  idempotencyKey = key(),
) {
  return (await mutate(
    'receiving-accounts',
    input,
    idempotencyKey,
  )) as ReceivingAccount
}
export async function updateReceivingAccount(
  id: string,
  input: ReceivingAccountInput,
  idempotencyKey = key(),
) {
  return (await mutate(
    `receiving-accounts/${id}`,
    input,
    idempotencyKey,
    'patch',
  )) as ReceivingAccount
}
export async function mutate(
  path: string,
  payload: unknown,
  idempotencyKey: string,
  method: 'post' | 'patch' | 'delete' = 'post',
) {
  return (
    await apiClient.request({
      url: `${adminBase}/${path}`,
      method,
      data: payload,
      headers: { 'Idempotency-Key': idempotencyKey },
    })
  ).data
}
export async function attachments(
  kind: AccountingKind,
  id: string,
  customer: boolean,
) {
  const data = (
    await apiClient.get<
      AccountingAttachment[] | PaginatedResponse<AccountingAttachment>
    >(
      `${base(customer)}/attachments`,
      { params: { object_kind: kind, object_id: id } },
    )
  ).data
  return Array.isArray(data) ? data : data.items
}
export async function download(id: string, customer: boolean) {
  return (
    await apiClient.get<{ url: string }>(
      `${base(customer)}/attachments/${id}/download-url`,
    )
  ).data.url
}
export async function upload(
  file: File,
  kind: AccountingKind,
  record: Pick<AccountingRecord, 'id' | 'organization_id'>,
): Promise<AccountingAttachment> {
  const bytes = await file.arrayBuffer()
  const sha256 = Array.from(
    new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)),
    (b) => b.toString(16).padStart(2, '0'),
  ).join('')
  const payload = {
    organization_id: record.organization_id,
    object_kind: kind,
    object_id: record.id,
    purpose: kind === 'vendor-invoices' ? 'INVOICE' : 'PROOF',
    filename: file.name,
    content_type: file.type,
    size_bytes: file.size,
    sha256,
  }
  const manifestBytes = new TextEncoder().encode(JSON.stringify(payload))
  const manifestHash = Array.from(
    new Uint8Array(await crypto.subtle.digest('SHA-256', manifestBytes)),
    (byte) => byte.toString(16).padStart(2, '0'),
  ).join('')
  let intent = (await mutate(
    'attachments/upload-intents',
    payload,
    `intent-${record.id}-${manifestHash}`,
  )) as { attachment: AccountingAttachment }
  if (new Date(intent.attachment.expires_at).getTime() <= Date.now()) {
    intent = (await mutate(
      'attachments/upload-intents',
      payload,
      key(),
    )) as typeof intent
  }
  const uploadContent = () =>
    apiClient.put(
      `${adminBase}/attachments/${intent.attachment.id}/content`,
      file,
      {
        headers: {
          'Content-Type': file.type,
          'Idempotency-Key': `upload-${intent.attachment.id}-${sha256}`,
        },
      },
    )
  try {
    await uploadContent()
  } catch (error) {
    if ((error as { status?: number }).status !== 404) throw error
    intent = (await mutate(
      'attachments/upload-intents',
      payload,
      key(),
    )) as typeof intent
    await uploadContent()
  }
  const completed = (await mutate(
    `attachments/${intent.attachment.id}/complete`,
    {},
    `complete-${intent.attachment.id}`,
  )) as AccountingAttachment | { attachment?: AccountingAttachment } | null
  if (completed && 'id' in completed) return completed
  return (completed && 'attachment' in completed && completed.attachment) || intent.attachment
}
export async function reconciliation(filter: AccountingFilter) {
  return (
    await apiClient.get<PaginatedResponse<ReconciliationRow>>(
      `${adminBase}/reconciliation`,
      { params: filter },
    )
  ).data
}
export async function auditLogs(filter: AccountingFilter) {
  return (
    await apiClient.get<PaginatedResponse<AuditRow>>(
      `${adminBase}/audit-logs`,
      { params: filter },
    )
  ).data
}
export async function exportCSV(
  kind: AccountingKind,
  filter: AccountingFilter,
) {
  return (
    await apiClient.get<Blob>(`${adminBase}/export`, {
      params: { ...filter, kind },
      responseType: 'blob',
    })
  ).data
}

export async function trend(
  customer: boolean,
  filter: AccountingFilter,
  signal?: AbortSignal,
) {
  return (
    await apiClient.get<{ date: string; currency: string; amount: string }[]>(
      `${base(customer)}/trend`,
      { params: filter, signal },
    )
  ).data
}

export async function allocationHistory(kind: AccountingKind, id: string) {
  return (await apiClient.get<AllocationRow[]>(`${adminBase}/${kind}/${id}/allocations`)).data
}

export async function workflow<T>(
  path: string,
  organization: string,
  customer = false,
  signal?: AbortSignal,
) {
  return (
    await apiClient.get<T>(`${base(customer)}/${path}`, {
      params: customer ? {} : { organization_id: organization },
      signal,
    })
  ).data
}
export async function statementCSV(id: string, customer = false) {
  return (
    await apiClient.get<Blob>(
      `${base(customer)}/statements/${encodeURIComponent(id)}/export`,
      { responseType: 'blob' },
    )
  ).data
}

export async function orders(organizationId: string, filter: { state?: string; page?: number; page_size?: number } = {}) {
  return (
    await apiClient.get<PaginatedResponse<AccountingOrder>>(`${adminBase}/orders`, {
      params: { organization_id: organizationId, ...filter },
    })
  ).data
}
export async function order(id: string) {
  return (await apiClient.get<AccountingOrder>(`${adminBase}/orders/${encodeURIComponent(id)}`)).data
}
