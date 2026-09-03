import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'
import type {
  CreateDesktopMemberRequest,
  DesktopMember,
  DesktopOrganization,
  DesktopStatus,
  DesktopTargetConfig,
  UpdateDesktopMemberRequest,
  UpdateDesktopOrganizationRequest,
} from './admin/desktop'

const basePath = '/desktop/organization'

function idempotencyHeaders(prefix: string): Record<string, string> {
  const value = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
  return { 'Idempotency-Key': `${prefix}-${value}` }
}

export async function getOrganization(_publicID = ''): Promise<DesktopOrganization | null> {
  const { data, status } = await apiClient.get<DesktopOrganization | '' | undefined>(basePath)
  return status === 204 || !data ? null : data
}

export async function updateOrganization(
  _publicID: string,
  input: Pick<UpdateDesktopOrganizationRequest, 'name' | 'status'>,
): Promise<DesktopOrganization> {
  const { data } = await apiClient.patch<DesktopOrganization>(basePath, input)
  return data
}

export async function updateModelConfiguration(
  _publicID: string,
  targetConfig: DesktopTargetConfig,
): Promise<DesktopOrganization> {
  const { data } = await apiClient.put<DesktopOrganization>(`${basePath}/model-configuration`, {
    target_config: targetConfig,
  })
  return data
}

export async function listMembers(
  _organizationID: string,
  page: number,
  pageSize: number,
  filters: { search?: string; status?: DesktopStatus | '' },
  signal?: AbortSignal,
): Promise<PaginatedResponse<DesktopMember>> {
  const { data } = await apiClient.get<PaginatedResponse<DesktopMember>>(`${basePath}/members`, {
    params: { page, page_size: pageSize, ...filters },
    signal,
  })
  return data
}

export async function createMember(
  _organizationID: string,
  input: CreateDesktopMemberRequest,
): Promise<DesktopMember> {
  const { data } = await apiClient.post<DesktopMember>(`${basePath}/members`, input, {
    headers: idempotencyHeaders('desktop-managed-member-create'),
  })
  return data
}

export async function updateMember(
  _organizationID: string,
  memberID: string,
  input: UpdateDesktopMemberRequest,
): Promise<DesktopMember> {
  const { data } = await apiClient.patch<DesktopMember>(
    `${basePath}/members/${encodeURIComponent(memberID)}`,
    input,
  )
  return data
}

export async function deleteMember(
  _organizationID: string,
  memberID: string,
): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(
    `${basePath}/members/${encodeURIComponent(memberID)}`,
  )
  return data
}

export async function rotateModelToken(
  _organizationID: string,
  memberID: string,
): Promise<DesktopMember> {
  const { data } = await apiClient.post<DesktopMember>(
    `${basePath}/members/${encodeURIComponent(memberID)}/model-token/rotate`,
    undefined,
    { headers: idempotencyHeaders('desktop-managed-model-token-rotate') },
  )
  return data
}

export default {
  getOrganization,
  updateOrganization,
  updateModelConfiguration,
  listMembers,
  createMember,
  updateMember,
  deleteMember,
  rotateModelToken,
}
