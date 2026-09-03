import { apiClient } from '../client'
import type { AdminGroup, AdminUser, PaginatedResponse } from '@/types'

export type DesktopStatus = 'active' | 'disabled'
export type DesktopModelTokenStatus = 'active' | 'disabled' | 'missing'
export type DesktopWireAPI = 'responses' | 'chat_completions'
export type DesktopUpdateStatus = 'draft' | 'published' | 'withdrawn'
export type DesktopUpdatePlatform = 'darwin-aarch64' | 'darwin-x86_64' | 'windows-x86_64'

export interface DesktopUpdateArtifact {
  url: string
  signature: string
  object_key: string
  file_name: string
  size: number
  sha256: string
}

export type DesktopUpdateArtifacts = Record<DesktopUpdatePlatform, DesktopUpdateArtifact>

export interface DesktopUpdateRelease {
  public_id: string
  version: string
  notes: string
  artifacts: DesktopUpdateArtifacts
  status: DesktopUpdateStatus
  created_by?: number
  updated_by?: number
  published_by?: number
  withdrawn_by?: number
  published_at?: string
  withdrawn_at?: string
  withdrawal_reason?: string
  created_at: string
  updated_at: string
}

export interface DesktopUpdateDraftRequest {
  version: string
  notes: string
  artifacts: DesktopUpdateArtifacts
}

export interface DesktopGatewayUser {
  id: number
  email: string
  username: string
}

export interface DesktopGroup {
  id: number
  name: string
}

export interface DesktopTarget {
  enabled: boolean
  provider_id: string
  display_name: string
  requested_model: string
  wire_api: DesktopWireAPI
  minimum_app_version?: string | null
  restart_required: boolean
}

export interface DesktopTargetConfig {
  schema_version: 1
  targets: {
    chatgpt_codex?: DesktopTarget
    workbuddy?: DesktopTarget
  }
}

export interface DesktopOrganization {
  public_id: string
  code: string
  name: string
  status: DesktopStatus
  gateway_user: DesktopGatewayUser
  group: DesktopGroup
  member_count: number
  target_config_assigned: boolean
  target_config?: DesktopTargetConfig | null
  created_at: string
  updated_at: string
}

export interface DesktopMember {
	public_id: string
	name: string
	phone: string
  status: DesktopStatus
  model_token_status: DesktopModelTokenStatus
  usage?: DesktopMemberUsage
  created_at: string
  updated_at: string
}

export interface DesktopMemberUsage {
  today_tokens: number
  last_30_days_tokens: number
  total_tokens: number
  today_actual_cost: number
  last_30_days_actual_cost: number
  total_actual_cost: number
}

export interface CreateDesktopOrganizationRequest {
  code: string
  name: string
  gateway_user_id: number
  group_id: number
}

export interface UpdateDesktopOrganizationRequest {
  name?: string
  status?: DesktopStatus
  gateway_user_id?: number
  group_id?: number
}

export interface CreateDesktopMemberRequest {
  name: string
  phone: string
}

export interface UpdateDesktopMemberRequest {
  name?: string
  phone?: string
  status?: DesktopStatus
}

function idempotencyHeaders(prefix: string): Record<string, string> {
  const value = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return { 'Idempotency-Key': `${prefix}-${value}` }
}

export async function listOrganizations(
  page: number,
  pageSize: number,
  filters: { search?: string; status?: DesktopStatus | '' },
  signal?: AbortSignal,
): Promise<PaginatedResponse<DesktopOrganization>> {
  const { data } = await apiClient.get<PaginatedResponse<DesktopOrganization>>('/admin/desktop/organizations', {
    params: { page, page_size: pageSize, ...filters },
    signal,
  })
  return data
}

export async function getOrganization(publicID: string): Promise<DesktopOrganization> {
  const { data } = await apiClient.get<DesktopOrganization>(`/admin/desktop/organizations/${encodeURIComponent(publicID)}`)
  return data
}

export async function createOrganization(input: CreateDesktopOrganizationRequest): Promise<DesktopOrganization> {
  const { data } = await apiClient.post<DesktopOrganization>('/admin/desktop/organizations', input, {
    headers: idempotencyHeaders('desktop-organization-create'),
  })
  return data
}

export async function updateOrganization(publicID: string, input: UpdateDesktopOrganizationRequest): Promise<DesktopOrganization> {
  const { data } = await apiClient.patch<DesktopOrganization>(`/admin/desktop/organizations/${encodeURIComponent(publicID)}`, input)
  return data
}

export async function updateModelConfiguration(publicID: string, targetConfig: DesktopTargetConfig): Promise<DesktopOrganization> {
  const { data } = await apiClient.put<DesktopOrganization>(`/admin/desktop/organizations/${encodeURIComponent(publicID)}/model-configuration`, {
    target_config: targetConfig,
  })
  return data
}

export async function listMembers(
  organizationID: string,
  page: number,
  pageSize: number,
  filters: { search?: string; status?: DesktopStatus | '' },
  signal?: AbortSignal,
): Promise<PaginatedResponse<DesktopMember>> {
  const { data } = await apiClient.get<PaginatedResponse<DesktopMember>>(
    `/admin/desktop/organizations/${encodeURIComponent(organizationID)}/members`,
    { params: { page, page_size: pageSize, ...filters }, signal },
  )
  return data
}

export async function createMember(organizationID: string, input: CreateDesktopMemberRequest): Promise<DesktopMember> {
  const { data } = await apiClient.post<DesktopMember>(
    `/admin/desktop/organizations/${encodeURIComponent(organizationID)}/members`,
    input,
    { headers: idempotencyHeaders('desktop-member-create') },
  )
  return data
}

export async function updateMember(organizationID: string, memberID: string, input: UpdateDesktopMemberRequest): Promise<DesktopMember> {
  const { data } = await apiClient.patch<DesktopMember>(
    `/admin/desktop/organizations/${encodeURIComponent(organizationID)}/members/${encodeURIComponent(memberID)}`,
    input,
  )
  return data
}

export async function deleteMember(organizationID: string, memberID: string): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(
    `/admin/desktop/organizations/${encodeURIComponent(organizationID)}/members/${encodeURIComponent(memberID)}`,
  )
  return data
}

export async function rotateModelToken(organizationID: string, memberID: string): Promise<DesktopMember> {
  const { data } = await apiClient.post<DesktopMember>(
    `/admin/desktop/organizations/${encodeURIComponent(organizationID)}/members/${encodeURIComponent(memberID)}/model-token/rotate`,
    undefined,
    { headers: idempotencyHeaders('desktop-model-token-rotate') },
  )
  return data
}

export async function listAvailableGatewayUsers(search = '', signal?: AbortSignal): Promise<PaginatedResponse<AdminUser>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminUser>>('/admin/users', {
    params: { page: 1, page_size: 30, search, available_for_desktop: true },
    signal,
  })
  return data
}

export async function getGatewayUser(id: number): Promise<AdminUser> {
  const { data } = await apiClient.get<AdminUser>(`/admin/users/${id}`)
  return data
}

export async function listActiveGroups(): Promise<AdminGroup[]> {
  const { data } = await apiClient.get<AdminGroup[]>('/admin/groups/all')
  return data.filter((group) => group.status === 'active')
}

export async function listUpdateReleases(
  page: number,
  pageSize: number,
  status: DesktopUpdateStatus | '',
  signal?: AbortSignal,
): Promise<PaginatedResponse<DesktopUpdateRelease>> {
  const { data } = await apiClient.get<PaginatedResponse<DesktopUpdateRelease>>('/admin/desktop/updates', {
    params: { page, page_size: pageSize, status },
    signal,
  })
  return data
}

export async function getUpdateRelease(publicID: string): Promise<DesktopUpdateRelease> {
  const { data } = await apiClient.get<DesktopUpdateRelease>(`/admin/desktop/updates/${encodeURIComponent(publicID)}`)
  return data
}

export async function createUpdateRelease(input: DesktopUpdateDraftRequest): Promise<DesktopUpdateRelease> {
  const { data } = await apiClient.post<DesktopUpdateRelease>('/admin/desktop/updates', input, {
    headers: idempotencyHeaders('desktop-update-create'),
  })
  return data
}

export async function updateUpdateRelease(publicID: string, input: DesktopUpdateDraftRequest): Promise<DesktopUpdateRelease> {
  const { data } = await apiClient.patch<DesktopUpdateRelease>(`/admin/desktop/updates/${encodeURIComponent(publicID)}`, input)
  return data
}

export async function uploadUpdateArtifact(
  publicID: string,
  platform: DesktopUpdatePlatform,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<DesktopUpdateArtifact> {
  const form = new FormData()
  form.append('file', file)
  const { data } = await apiClient.post<DesktopUpdateArtifact>(
    `/admin/desktop/updates/${encodeURIComponent(publicID)}/artifacts/${encodeURIComponent(platform)}`,
    form,
    {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 10 * 60 * 1000,
      onUploadProgress: (event) => {
        if (!event.total || !onProgress) return
        onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
      },
    },
  )
  return data
}

export async function publishUpdateRelease(publicID: string): Promise<DesktopUpdateRelease> {
  const { data } = await apiClient.post<DesktopUpdateRelease>(
    `/admin/desktop/updates/${encodeURIComponent(publicID)}/publish`,
    undefined,
    { headers: idempotencyHeaders('desktop-update-publish') },
  )
  return data
}

export async function withdrawUpdateRelease(publicID: string, reason: string): Promise<DesktopUpdateRelease> {
  const { data } = await apiClient.post<DesktopUpdateRelease>(
    `/admin/desktop/updates/${encodeURIComponent(publicID)}/withdraw`,
    { reason },
    { headers: idempotencyHeaders('desktop-update-withdraw') },
  )
  return data
}

export default {
  listOrganizations,
  getOrganization,
  createOrganization,
  updateOrganization,
  updateModelConfiguration,
  listMembers,
  createMember,
  updateMember,
  deleteMember,
  rotateModelToken,
  listAvailableGatewayUsers,
  getGatewayUser,
  listActiveGroups,
  listUpdateReleases,
  getUpdateRelease,
  createUpdateRelease,
  updateUpdateRelease,
  uploadUpdateArtifact,
  publishUpdateRelease,
  withdrawUpdateRelease,
}
