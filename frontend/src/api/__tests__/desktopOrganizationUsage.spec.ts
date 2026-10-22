import { beforeEach, describe, expect, it, vi } from 'vitest'
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))
import { getDesktopOrganizationUsageStatistics, exportDesktopMemberUsage } from '../desktopOrganizationUsage'

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
  it('passes the selected date range without changing the organization scope', async () => {
    const controller = new AbortController()
    await getDesktopOrganizationUsageStatistics('org/one', false, controller.signal, { from: '2026-09-01', to: '2026-09-07' })
    expect(client.get).toHaveBeenCalledWith('/admin/desktop/organizations/org%2Fone/usage/statistics', {
      signal: controller.signal, params: { from: '2026-09-01', to: '2026-09-07' },
    })
  })
  it('requests the export snapshot with localized labels and cancellation', async () => {
    const controller = new AbortController(), file = new Blob(['CSV'])
    client.post.mockResolvedValueOnce({ data: file })
    expect(await exportDesktopMemberUsage('org/one', false, { unassigned: true }, key => key, controller.signal)).toBe(file)
    expect(client.post).toHaveBeenCalledWith('/admin/desktop/organizations/org%2Fone/members/usage-export', expect.objectContaining({ headers: expect.arrayContaining(['desktopStructure.position']) }), expect.objectContaining({ params: { unassigned: true }, responseType: 'blob', signal: controller.signal }))
    expect(client.post.mock.lastCall![1].headers).toHaveLength(19)
  })

})
