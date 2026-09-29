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
}

export interface DesktopUsageModel extends DesktopUsageRank {
  model: string
}

export interface DesktopUsageMember extends DesktopUsageRank {
  member_id: string
  name: string
  deleted: boolean
}

export interface DesktopOrganizationUsageStatistics {
  timezone: string
  as_of: string
  today: DesktopOrganizationUsagePeriod
  week: DesktopOrganizationUsagePeriod
  month: DesktopOrganizationUsagePeriod
  total: DesktopOrganizationUsagePeriod
  last_30_days: DesktopOrganizationUsagePeriod
  daily: DesktopUsageDay[]
  breakdown: {
    input_tokens: number
    output_tokens: number
    cache_creation_tokens: number
    cache_read_tokens: number
  }
  models: DesktopUsageModel[]
  members: DesktopUsageMember[]
  observed_members: number
}

export async function getDesktopOrganizationUsageStatistics(organizationID: string, selfManaged: boolean, signal?: AbortSignal): Promise<DesktopOrganizationUsageStatistics> {
  const basePath = selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(organizationID)}`
  const { data } = await apiClient.get<DesktopOrganizationUsageStatistics>(`${basePath}/usage/statistics`, { signal })
  return data
}
