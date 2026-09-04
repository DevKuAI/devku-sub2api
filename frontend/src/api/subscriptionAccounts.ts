import { apiClient } from './client'
import type { SubscriptionAccount } from '@/types'

export async function list(includeUsage = false): Promise<SubscriptionAccount[]> {
  const { data } = await apiClient.get<SubscriptionAccount[]>('/subscription-accounts', {
    params: { include_usage: includeUsage }
  })
  return data
}

export const subscriptionAccountsAPI = { list }

export default subscriptionAccountsAPI
