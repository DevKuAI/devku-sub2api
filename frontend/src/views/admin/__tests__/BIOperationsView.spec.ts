import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BIOperationsView from '../BIOperationsView.vue'

const { getOverview, listBindings, listChallenges, listSessions } = vi.hoisted(() => ({
  getOverview: vi.fn(),
  listBindings: vi.fn(),
  listChallenges: vi.fn(),
  listSessions: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { bi: { getOverview, listBindings, listChallenges, listSessions } },
}))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))

describe('BI Operations identity lists', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getOverview.mockResolvedValue({
      config: { enabled: true, appid: '', app_secret_configured: false, jwt_secret_configured: false, identity_secret_configured: false },
      counts: { bindings: 0, pending_challenges: 0, active_sessions: 0, sources: 0, imports_processing: 0, reports_processing: 0 },
      retention: { report_months: 24, fact_months: 24, audit_days: 180, ephemeral_days: 7 },
    })
    listBindings.mockRejectedValueOnce({ reason: 'INTERNAL_ERROR', message: 'BI operation failed', metadata: { request_id: 'bi-request-123' } })
    listBindings.mockResolvedValue({ items: [{ id: 'binding-1', display_name: 'Alice', user_id: 7, status: 'active', session_count: 1 }], total: 1, page: 1, page_size: 20 })
    listChallenges.mockResolvedValue({ items: [{ id: 'challenge-1', status: 'pending', expires_at: '2026-09-28T00:00:00Z' }], total: 1, page: 1, page_size: 20 })
    listSessions.mockResolvedValue({ items: [{ id: 'session-1', status: 'active', expires_at: '2026-09-28T00:00:00Z' }], total: 1, page: 1, page_size: 20 })
  })

  it('keeps healthy identity lists visible and retries only the failed list', async () => {
    const wrapper = shallowMount(BIOperationsView)
    await flushPromises()
    const identityTab = wrapper.findAll('button').find((button) => button.text() === '微信身份')
    expect(identityTab).toBeDefined()
    await identityTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('bi-request-123')
    expect(wrapper.text()).toContain('challenge-1')
    expect(wrapper.text()).toContain('session-1')
    expect(wrapper.findAll('[role="alert"]')).toHaveLength(1)

    const retry = wrapper.findAll('button').find((button) => button.text() === '重试绑定列表')
    expect(retry).toBeDefined()
    await retry!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('binding-1')
    expect(wrapper.text()).not.toContain('bi-request-123')
    expect(listChallenges).toHaveBeenCalledTimes(1)
    expect(listSessions).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
})
