import { apiClient } from './client'

export interface DesktopUsageMeasurement {
 token_status?: 'ready' | 'partial' | 'unavailable'
 measured_requests?: number
 unmeasured_requests?: number
}
export interface DesktopOrganizationUsagePeriod extends DesktopUsageMeasurement {
  total_tokens: number
  actual_cost: number
}

export interface DesktopUsageDay extends DesktopOrganizationUsagePeriod {
  date: string
}

export interface DesktopUsageRank extends DesktopOrganizationUsagePeriod {
  requests: number
  cost_rank: number
  token_rank: number
}

export interface DesktopAnalyticsRange {
  days?: 7 | 30 | 90
  from?: string
  to?: string
}

export interface DesktopUsageModel extends DesktopUsageRank {
  model: string
}

export interface DesktopUsageMember extends DesktopUsageRank {
  member_id: string
  name: string
  deleted: boolean
}

export interface DesktopUsageMemberModel extends DesktopUsageMeasurement {
  member_id: string
  name: string
  deleted: boolean
  model: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
}

export interface DesktopMemberModelPage extends DesktopUsageMeasurement {
 page: number
 page_size: number
 total_members: number
 total_rows: number
 model_count: number
 total_tokens: number
 models: string[]
}
export interface DesktopUsageQuery { section?: 'summary' | 'insights' | 'member_models'; page?: number; page_size?: number; search?: string; model?: string; sort?: 'tokens' | 'name'; as_of?: string }
export interface DesktopOrganizationUsageStatistics {
 member_models_page?: DesktopMemberModelPage
  analysis?: DesktopOrganizationUsagePeriod
  timezone: string
  as_of: string
  today: DesktopOrganizationUsagePeriod
  week: DesktopOrganizationUsagePeriod
  month: DesktopOrganizationUsagePeriod
  total: DesktopOrganizationUsagePeriod
  last_30_days: DesktopOrganizationUsagePeriod
  selected: DesktopOrganizationUsagePeriod
  previous: DesktopOrganizationUsagePeriod
  range_start: string
  range_end: string
  previous_start: string
  previous_end: string
  daily: DesktopUsageDay[]
  breakdown: {
    input_tokens: number
    output_tokens: number
    cache_creation_tokens: number
    cache_read_tokens: number
  }
  models: DesktopUsageModel[]
  members: DesktopUsageMember[]
  member_models: DesktopUsageMemberModel[]
  observed_members: number
}

export async function getDesktopOrganizationUsageStatistics(organizationID: string, selfManaged: boolean, signal?: AbortSignal, range?: DesktopAnalyticsRange, query?: DesktopUsageQuery): Promise<DesktopOrganizationUsageStatistics> {
  const basePath = selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(organizationID)}`
  const { data } = await apiClient.get<DesktopOrganizationUsageStatistics>(`${basePath}/usage/statistics`, (range || query) ? { signal, params: { ...range, ...query } } : { signal })
  return data
}

export async function exportDesktopMemberUsage(organizationId: string, selfManaged: boolean, filters: Record<string, string | boolean | undefined>, t: (key: string) => string, signal: AbortSignal): Promise<Blob> {
 const { desktopMemberUsageCsvHeaders } = await import('@/utils/desktopMemberUsageCsv')
 const base = selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(organizationId)}`
 const { data } = await apiClient.post<Blob | string>(`${base}/members/usage-export`, { headers: desktopMemberUsageCsvHeaders(t), active: t('common.active'), disabled: t('common.disabled'), token_labels: Object.fromEntries(['active', 'disabled', 'missing'].map(status => [status, t(`admin.desktop.tokenStatus.${status}`)])) }, { params: filters, responseType: 'blob', signal })
 if (data instanceof Blob) return data
 if (typeof data === 'string') return new Blob([data], { type: 'text/csv;charset=utf-8;' })
 throw new Error('Invalid CSV response')
}
