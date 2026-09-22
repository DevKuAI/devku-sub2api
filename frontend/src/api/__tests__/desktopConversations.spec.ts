import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))
import { getDesktopConversation, getDesktopConversationStatistics, listDesktopConversations } from '../desktopConversations'

describe('Desktop conversation API', () => {
  beforeEach(() => { client.get.mockReset(); client.get.mockResolvedValue({ data: { items: [], total: 0 } }) })

  it('uses the managed route without sending the supplied organization ID', async () => {
    const controller = new AbortController()
    await listDesktopConversations('org_other', true, { page: 1, page_size: 20 }, controller.signal)
    await getDesktopConversation('org_other', true, 'record/one', controller.signal)
    expect(client.get).toHaveBeenNthCalledWith(1, '/desktop/organization/conversation-records', { params: { page: 1, page_size: 20 }, signal: controller.signal })
    expect(client.get).toHaveBeenNthCalledWith(2, '/desktop/organization/conversation-records/record%2Fone', { signal: controller.signal })
  })

  it('escapes identifiers and forwards all thread coordinates for administrators', async () => {
    const query = { page: 2, page_size: 20, member_id: 'mem_one', client: 'workbuddy', installation_id: 'device_one', source_session_id: 'source_one', sort_order: 'asc' as const }
    await listDesktopConversations('org/one', false, query)
    expect(client.get).toHaveBeenCalledWith('/admin/desktop/organizations/org%2Fone/conversation-records', { params: query, signal: undefined })
  })
  it('uses organization-scoped statistics routes and forwards filters and cancellation', async () => {
    const controller = new AbortController()
    const filters = { member_search: 'Member', client: 'workbuddy', received_from: '2026-09-21T16:00:00Z' }
    await getDesktopConversationStatistics('org/one', false, filters, controller.signal)
    await getDesktopConversationStatistics('org_other', true, filters, controller.signal)
    expect(client.get).toHaveBeenNthCalledWith(1, '/admin/desktop/organizations/org%2Fone/conversation-records/statistics', { params: filters, signal: controller.signal })
    expect(client.get).toHaveBeenNthCalledWith(2, '/desktop/organization/conversation-records/statistics', { params: filters, signal: controller.signal })
  })

})
