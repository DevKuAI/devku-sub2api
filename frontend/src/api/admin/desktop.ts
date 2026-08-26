import { apiClient } from '../client'
import type { AdminGroup, AdminUser, PaginatedResponse } from '@/types'

export type DesktopStatus = 'active' | 'disabled'
export type DesktopModelTokenStatus = 'active' | 'disabled' | 'missing'
export type DesktopWireAPI = 'responses' | 'chat_completions'

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
  created_at: string
  updated_at: string
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
}
