import { apiClient } from './client'
import type { SubscriptionAccount, SubscriptionAccountUsage } from '@/types'
import type { OpenAIQuotaUsage, OpenAIQuotaResetResult } from './admin/accounts'

type SubscriptionQuotaUsage = Pick<OpenAIQuotaUsage, 'fetched_at' | 'rate_limit_reset_credits'>
export type SubscriptionQuotaRefreshResult = SubscriptionQuotaUsage & { cache_persisted: boolean }
export type SubscriptionQuotaResetResult = Pick<OpenAIQuotaResetResult,
  'code' | 'windows_reset' | 'cache_refreshed' | 'account_state_recovered' | 'warning_code'
> & { quota?: SubscriptionQuotaUsage | null }

export async function list(includeUsage = false): Promise<SubscriptionAccount[]> {
  const { data } = await apiClient.get<SubscriptionAccount[]>('/subscription-accounts', {
    params: { include_usage: includeUsage }
  })
  return data
}

export async function getUsage(id: number): Promise<SubscriptionAccountUsage | null> {
  const { data } = await apiClient.get<SubscriptionAccountUsage | null>(`/subscription-accounts/${id}/usage`)
  return data
}

export async function refreshUsage(id: number): Promise<SubscriptionAccountUsage | null> {
  const { data } = await apiClient.post<SubscriptionAccountUsage | null>(`/subscription-accounts/${id}/usage/refresh`)
  return data
}

export async function refreshQuota(id: number): Promise<SubscriptionQuotaRefreshResult> {
  const { data } = await apiClient.post<SubscriptionQuotaRefreshResult>(`/subscription-accounts/${id}/quota/refresh`)
  return data
}

export async function resetQuota(id: number): Promise<SubscriptionQuotaResetResult> {
  const { data } = await apiClient.post<SubscriptionQuotaResetResult>(
    `/subscription-accounts/${id}/reset-quota`,
    undefined,
    { timeout: 90_000 }
  )
  return data
}

export const subscriptionAccountsAPI = { list, getUsage, refreshUsage, refreshQuota, resetQuota }

export default subscriptionAccountsAPI
