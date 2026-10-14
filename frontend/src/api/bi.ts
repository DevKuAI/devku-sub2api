import { apiClient } from './client'
export const biCapabilities = ['analytics:read', 'members:read', 'conversations:read', 'reports:read', 'reports:copy'] as const
export type BICapability = typeof biCapabilities[number]
export interface BIPage<T> { items: T[]; page: number; page_size: number; total: number; has_more: boolean }
export interface BIUserAccess { bi_user_id: string; display_name: string; organization_alias?: string; user_status: string; status: 'active' | 'revoked'; scope: 'organization'; capabilities: BICapability[]; revision: number; last_seen_at: string | null }
export interface BIInvitation { id: string; organization_id: string; organization_name: string; capabilities: BICapability[]; note: string; recipient_name?: string; status: string; expires_at: string; created_at: string; claimed_by: string | null; claimed_at: string | null }
export interface BIIssuedInvitation extends BIInvitation { token: string; path: string }
export function biManagement(organization: string, selfManaged = false) {
  const base = selfManaged ? '/desktop/organization/bi' : `/admin/bi/organizations/${encodeURIComponent(organization)}`
  return {
    async users(page = 1) { return (await apiClient.get<BIPage<BIUserAccess>>(`${base}/users`, { params: { page, page_size: 20 } })).data },
    async invitations(page = 1) { return (await apiClient.get<BIPage<BIInvitation>>(`${base}/invitations`, { params: { page, page_size: 20 } })).data },
    async create(capabilities: BICapability[], note: string, recipientName = '') { return (await apiClient.post<BIIssuedInvitation>(`${base}/invitations`, { capabilities, note, recipient_name: recipientName })).data },
    async revoke(id: string) { await apiClient.post(`${base}/invitations/${encodeURIComponent(id)}/revoke`, {}) },
    async access(user: BIUserAccess, capabilities: BICapability[], status: BIUserAccess['status'], organizationAlias?: string) { return (await apiClient.put<BIUserAccess>(`${base}/users/${encodeURIComponent(user.bi_user_id)}/access`, { capabilities, status, expected_revision: user.revision, ...(organizationAlias !== undefined ? {organization_alias:organizationAlias} : {}) })).data },
  }
}
export function capabilityDependencies(caps: BICapability[]): BICapability[] {
  const next = new Set(caps)
  if (next.has('conversations:read') || next.has('reports:read') || next.has('reports:copy')) next.add('members:read')
  if (next.has('reports:copy')) next.add('reports:read')
  return biCapabilities.filter(cap => next.has(cap))
}
export async function isBIEnabled() {
  const { data } = await apiClient.get<{ bi_enabled?: boolean }>('/settings/public')
  if (typeof data?.bi_enabled !== 'boolean') throw new Error('BI availability missing from public settings')
  return data.bi_enabled
}
