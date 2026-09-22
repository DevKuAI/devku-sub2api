import { apiClient } from './client'

export interface DesktopOrganizationUsagePeriod {
  total_tokens: number
  actual_cost: number
}

export interface DesktopOrganizationUsageStatistics {
  timezone: string
  as_of: string
  today: DesktopOrganizationUsagePeriod
  week: DesktopOrganizationUsagePeriod
  month: DesktopOrganizationUsagePeriod
  total: DesktopOrganizationUsagePeriod
}

export async function getDesktopOrganizationUsageStatistics(organizationID: string, selfManaged: boolean, signal?: AbortSignal): Promise<DesktopOrganizationUsageStatistics> {
  const basePath = selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(organizationID)}`
  const { data } = await apiClient.get<DesktopOrganizationUsageStatistics>(`${basePath}/usage/statistics`, { signal })
  return data
}
