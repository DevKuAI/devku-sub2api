import { beforeEach, describe, expect, it, vi } from 'vitest'
const client = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))
import { getDesktopOrganizationUsageStatistics } from '../desktopOrganizationUsage'

describe('Desktop organization usage API', () => {
  beforeEach(() => { client.get.mockReset().mockResolvedValue({ data: { total: { total_tokens: 42, actual_cost: 1.5 } } }) })
  it('uses the administrator organization scope without pagination or member filters', async () => {
    const controller = new AbortController()
    const result = await getDesktopOrganizationUsageStatistics('org/one', false, controller.signal)
    expect(client.get).toHaveBeenCalledWith('/admin/desktop/organizations/org%2Fone/usage/statistics', { signal: controller.signal })
    expect(result.total).toEqual({ total_tokens: 42, actual_cost: 1.5 })
  })
  it('uses the authenticated organization scope for enterprise users', async () => {
    await getDesktopOrganizationUsageStatistics('org_untrusted', true)
    expect(client.get).toHaveBeenCalledWith('/desktop/organization/usage/statistics', { signal: undefined })
  })
})
