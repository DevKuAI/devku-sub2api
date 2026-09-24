import { apiClient } from '../client'

export interface BIRetentionPolicy {
  report_months: number
  fact_months: number
  audit_days: number
  ephemeral_days: number
  updated_by?: number | null
  updated_at?: string | null
}

export interface BIConfigStatus {
  enabled: boolean
  appid: string
  min_client_version: string
  app_secret_configured: boolean
  jwt_secret_configured: boolean
  identity_secret_configured: boolean
  report_retention_months: number
}

export interface BIOperationsOverview {
  config: BIConfigStatus
  counts: Record<string, number>
  retention: BIRetentionPolicy
}

export interface BIBindingOperation {
  id: string
  user_id: number
  display_name: string
  status: string
  created_at: string
  last_login_at: string | null
  revoked_at: string | null
  session_count: number
}

export interface BIChallengeOperation {
  id: string
  status: string
  created_at: string
  expires_at: string
  binding_id: string | null
}

export interface BISessionOperation {
  id: string
  binding_id: string
  user_id: number
  status: string
  device_id: string | null
  platform: string | null
  client_version: string | null
  created_at: string
  last_seen_at: string | null
  expires_at: string
  revoked_at: string | null
}

export interface BISourceOperation {
  organization_id: string
  source_id: string
  namespace: string
  allowed_kinds: string[]
  checkpoint: string | null
  last_applied_at: string | null
  complete_through: string | null
  history_start_date: string | null
  initial_backfill_complete: boolean
  credential_count: number
}

export interface BICredentialOperation {
  id: string
  organization_id: string
  source_id: string
  token_prefix: string
  status: string
  created_at: string
  expires_at: string
  revoked_at: string | null
}

export interface BIImportOperation {
  id: string
  organization_id: string
  source_id: string
  status: string
  received_at: string
  applied_at: string | null
  checkpoint: string | null
  data_revision: string | null
  errors: Array<{ record_index: number | null; code: string; message: string }>
  lease_until: string | null
  attempt_count: number
  retry_of: string | null
}

export interface BIQualityOperation {
  organization_id: string
  source_id: string
  status: string
  complete_through: string | null
  history_start_date: string | null
  coverage_start: string | null
  coverage_end: string | null
  latency_seconds: number | null
  missing_dimensions: string[]
  reason: string | null
}

export interface BIReportOperation {
  id: string
  organization_id: string
  manager_id: string
  title: string
  status: string
  created_at: string
  expires_at: string
  failure_code: string | null
  generated_at: string | null
  lease_until: string | null
  attempt_count: number
  archived_at: string | null
  retry_of: string | null
}

export interface BIAuditEvent {
  id: number
  actor_user_id: number | null
  organization_id: string | null
  action: string
  target_id: string
  request_id: string
  metadata: Record<string, unknown>
  created_at: string
}

interface ListResponse<T> { items: T[]; total: number; page: number; page_size: number }
const list = <T>(path: string, params?: Record<string, unknown>) => apiClient.get<ListResponse<T>>(path, { params }).then(({ data }) => data)

const biAdminAPI = {
  getOverview: () => apiClient.get<BIOperationsOverview>('/admin/bi/overview').then(({ data }) => data),
  getRetention: () => apiClient.get<BIRetentionPolicy>('/admin/bi/retention').then(({ data }) => data),
  updateRetention: (input: BIRetentionPolicy) => apiClient.put<BIRetentionPolicy>('/admin/bi/retention', input).then(({ data }) => data),
  listBindings: (params?: Record<string, unknown>) => list<BIBindingOperation>('/admin/bi/identities/bindings', params),
  listChallenges: (params?: Record<string, unknown>) => list<BIChallengeOperation>('/admin/bi/identities/challenges', params),
  listSessions: (params?: Record<string, unknown>) => list<BISessionOperation>('/admin/bi/identities/sessions', params),
  revokeBinding: (id: string) => apiClient.post(`/admin/bi/identities/bindings/${encodeURIComponent(id)}/revoke`).then(({ data }) => data),
  listSources: (params?: Record<string, unknown>) => list<BISourceOperation>('/admin/bi/sources', params),
  listCredentials: (sourceID: string, organizationID?: string) => apiClient.get<{ items: BICredentialOperation[] }>(`/admin/bi/sources/${encodeURIComponent(sourceID)}/credentials`, { params: organizationID ? { organization_id: organizationID } : undefined }).then(({ data }) => data),
  issueCredential: (input: Record<string, unknown>) => apiClient.post<{ credential_id: string; token: string }>('/admin/bi/sources', input).then(({ data }) => data),
  revokeCredential: (id: string) => apiClient.post(`/admin/bi/credentials/${encodeURIComponent(id)}/revoke`).then(({ data }) => data),
  rotateCredential: (id: string, expires_at: string) => apiClient.post(`/admin/bi/credentials/${encodeURIComponent(id)}/rotate`, { expires_at }).then(({ data }) => data),
  listImports: (params?: Record<string, unknown>) => list<BIImportOperation>('/admin/bi/imports', params),
  getImport: (id: string) => apiClient.get<BIImportOperation>(`/admin/bi/imports/${encodeURIComponent(id)}`).then(({ data }) => data),
  retryImport: (id: string) => apiClient.post<BIImportOperation>(`/admin/bi/imports/${encodeURIComponent(id)}/retry`).then(({ data }) => data),
  listQuality: (params?: Record<string, unknown>) => apiClient.get<{ items: BIQualityOperation[]; as_of: string }>('/admin/bi/quality', { params }).then(({ data }) => data),
  listReports: (params?: Record<string, unknown>) => list<BIReportOperation>('/admin/bi/reports', params),
  getReport: (id: string) => apiClient.get<BIReportOperation>(`/admin/bi/reports/${encodeURIComponent(id)}`).then(({ data }) => data),
  retryReport: (id: string) => apiClient.post<BIReportOperation>(`/admin/bi/reports/${encodeURIComponent(id)}/retry`).then(({ data }) => data),
  archiveReport: (id: string) => apiClient.post(`/admin/bi/reports/${encodeURIComponent(id)}/archive`).then(({ data }) => data),
  getCleanup: () => apiClient.get<{ retention: BIRetentionPolicy }>('/admin/bi/cleanup').then(({ data }) => data),
  runCleanup: () => apiClient.post<{ deleted: number }>('/admin/bi/cleanup/run').then(({ data }) => data),
  listAudit: (params?: Record<string, unknown>) => list<BIAuditEvent>('/admin/bi/audit', params),
}

export default biAdminAPI
