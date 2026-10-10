import { apiClient } from './client'

export interface DesktopOrganizationUsagePeriod {
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

export interface DesktopUsageMemberModel {
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

export interface DesktopOrganizationUsageStatistics {
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

export async function getDesktopOrganizationUsageStatistics(organizationID: string, selfManaged: boolean, signal?: AbortSignal, range?: DesktopAnalyticsRange): Promise<DesktopOrganizationUsageStatistics> {
  const basePath = selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(organizationID)}`
  const { data } = await apiClient.get<DesktopOrganizationUsageStatistics>(`${basePath}/usage/statistics`, range ? { signal, params: range } : { signal })
  return data
}
