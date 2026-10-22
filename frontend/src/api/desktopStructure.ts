import { apiClient } from './client'
import type { DesktopMember, DesktopOrganization } from './admin/desktop'
import type { PaginatedResponse } from '@/types'
import * as adminDesktop from './admin/desktop'
import managedDesktop from './desktopOrganization'

export interface StructureRef { public_id: string; name: string }
export interface OrganizationUnit extends StructureRef {
  status: 'active' | 'disabled'
  revision: number
  department?: StructureRef
  member_count: number
}
export type UnitKind = 'departments' | 'positions'
export interface StructureStatus { status: 'complete' | 'needs_assignment'; unassigned_members: number; inconsistent_members: number; constraint_enabled: boolean }
export interface PositionHistory { position: StructureRef; department: StructureRef; position_revision: number; started_at: string; ended_at: string | null; source: string; reason: string }
export interface SaveUnitInput { name?: string; status?: 'active' | 'disabled'; expected_revision?: number; department_id?: string }
export const structureIdempotencyKey = () => `desktop-structure-${crypto.randomUUID()}`

export function desktopStructureError(cause: unknown, t: (key: string) => string): string {
  if (!cause || typeof cause !== 'object') return t('desktopStructure.failed')
  const error = cause as { reason?: string; message?: string }
  const messages: Record<string, string> = {
    ORGANIZATION_UNIT_NAME_EXISTS: 'nameExists',
    ORGANIZATION_STRUCTURE_CONFLICT: 'conflict',
    ORGANIZATION_UNIT_IN_USE: 'inUse',
    ORGANIZATION_UNIT_DISABLED: 'disabledUnit',
    ORGANIZATION_UNIT_NOT_FOUND: 'notFound',
    MEMBER_NOT_FOUND: 'notFound',
  }
  const message = messages[error.reason || '']
  if (message) return t(`desktopStructure.${message}`)
  return typeof error.message === 'string' && error.message ? error.message : t('desktopStructure.failed')
}

export function desktopStructure(organizationId: string, selfManaged = false) {
  const base = selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(organizationId)}`
  const members = selfManaged ? managedDesktop : adminDesktop
  return {
    members,
    organization: () => members.getOrganization(organizationId) as Promise<DesktopOrganization | null>,
    async list(kind: UnitKind, params: Record<string, string | number> = {}, signal?: AbortSignal) {
      const { data } = await apiClient.get<PaginatedResponse<OrganizationUnit>>(`${base}/${kind}`, { params: { page: 1, page_size: 100, ...params }, ...(signal ? { signal } : {}) })
      return data
    },
    // Dropdown options do not need member counts; callers can explicitly opt in.
    async all(kind: UnitKind, params: Record<string, string | number> = {}, signal?: AbortSignal) {
      const result: OrganizationUnit[] = []
      for (let page = 1; ; page++) {
        signal?.throwIfAborted()
        const data = await this.list(kind, { include_member_count: 'false', ...params, page }, signal)
        signal?.throwIfAborted()
        result.push(...data.items)
        if (result.length >= data.total || data.items.length === 0) return result
      }
    },
    async unit(kind: UnitKind, id: string) { return (await apiClient.get<OrganizationUnit>(`${base}/${kind}/${encodeURIComponent(id)}`)).data },
    async save(kind: UnitKind, id: string, input: SaveUnitInput, key: string) {
      if (id) return (await apiClient.patch<OrganizationUnit>(`${base}/${kind}/${encodeURIComponent(id)}`, input)).data
      return (await apiClient.post<OrganizationUnit>(`${base}/${kind}`, input, { headers: { 'Idempotency-Key': key } })).data
    },
    async status() { return (await apiClient.get<StructureStatus>(`${base}/organization-structure/status`)).data },
    async member(id: string) { return (await apiClient.get<DesktopMember>(`${base}/members/${encodeURIComponent(id)}`)).data },
    async assign(id: string, input: { position_id: string; expected_revision: number; reason: string }, key: string) {
      return (await apiClient.put<DesktopMember>(`${base}/members/${encodeURIComponent(id)}/position`, input, { headers: { 'Idempotency-Key': key } })).data
    },
    async history(id: string, page = 1) { return (await apiClient.get<PaginatedResponse<PositionHistory>>(`${base}/members/${encodeURIComponent(id)}/position-history`, { params: { page, page_size: 20 } })).data },
  }
}

export function desktopOrganizationPath(organizationId: string, selfManaged: boolean) {
  return selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(organizationId)}`
}

export function safeDesktopReturn(value: unknown, base: string, fallback = base) {
  if (typeof value !== 'string' || !value.startsWith('/') || value.includes('\\')) return fallback
  try {
    const parsed = new URL(value, 'https://desktop.invalid')
    if (parsed.origin !== 'https://desktop.invalid' || (parsed.pathname !== base && !parsed.pathname.startsWith(`${base}/`))) return fallback
    return parsed.pathname + parsed.search + parsed.hash
  } catch { return fallback }
}
