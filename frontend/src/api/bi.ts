import { apiClient } from './client'
import { buildGatewayUrl } from './url'

const baseURL = buildGatewayUrl('/api/bi/v1')

export interface BIPage<T> {
  items: T[]
  next_cursor: string | null
  has_more: boolean
  snapshot_id: string
}

export interface BIBinding {
  id: string
  display_name: string
  created_at: string
  last_login_at: string | null
}

export async function isBIEnabled(): Promise<boolean> {
  try {
    const { data } = await apiClient.get<{ binding_enabled: boolean }>('/bootstrap', { baseURL })
    return data.binding_enabled === true
  } catch (error) {
    if ((error as { status?: number }).status === 404) return false
    throw error
  }
}

export async function listBindings(cursor?: string): Promise<BIPage<BIBinding>> {
  const { data } = await apiClient.get<BIPage<BIBinding>>('/auth/bindings', {
    baseURL, params: { limit: 20, ...(cursor ? { cursor } : {}) },
  })
  return data
}

export async function approveBinding(userCode: string): Promise<void> {
  await apiClient.post('/auth/bindings/approve', { user_code: userCode, confirm_binding: true }, { baseURL })
}

export async function revokeBinding(id: string): Promise<void> {
  await apiClient.delete(`/auth/bindings/${encodeURIComponent(id)}`, { baseURL })
}

export interface BIGrant {
  manager_id: string
  user_id: number
  display_name: string
  role: 'org_admin' | 'team_manager' | 'viewer'
  all_teams: boolean
  team_ids: string[]
  capabilities: string[]
  revision: number
  revoked: boolean
}

export interface BIGrantInput {
  user_id: number
  role: BIGrant['role']
  all_teams: boolean
  team_ids: string[]
  capabilities: string[]
  expected_revision: number
}

export async function listGrants(organizationID: string, page = 1) {
  const { data } = await apiClient.get<{ items: BIGrant[]; page: number; has_more: boolean; capabilities: string[] }>(
    `/admin/bi/organizations/${encodeURIComponent(organizationID)}/grants`, { params: { page } },
  )
  return data
}

export async function saveGrant(organizationID: string, input: BIGrantInput): Promise<BIGrant> {
  const { data } = await apiClient.put<BIGrant>(`/admin/bi/organizations/${encodeURIComponent(organizationID)}/grants`, input)
  return data
}

export async function revokeGrant(organizationID: string, grant: BIGrant): Promise<void> {
  await apiClient.post(`/admin/bi/organizations/${encodeURIComponent(organizationID)}/grants/${encodeURIComponent(grant.manager_id)}/revoke`, {
    expected_revision: grant.revision,
  })
}
