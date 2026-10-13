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
} from './types'

const adminBase = '/admin/accounting'
const customerBase = '/organization/accounting'
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
  return (
    await apiClient.get<AccountingAttachment[]>(
      `${base(customer)}/attachments`,
      { params: { object_kind: kind, object_id: id } },
    )
  ).data
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
  record: AccountingRecord,
) {
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
  await mutate(
    `attachments/${intent.attachment.id}/complete`,
    {},
    `complete-${intent.attachment.id}`,
  )
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
  return (
    await apiClient.get<
      {
        id: string
        transfer_id: string
        card_id: string
        amount: string
        used_amount: string
        released_amount: string
        closed: boolean
      }[]
    >(`${adminBase}/${kind}/${id}/allocations`)
  ).data
}
