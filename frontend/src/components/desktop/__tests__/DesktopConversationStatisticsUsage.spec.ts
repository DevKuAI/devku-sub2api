import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const api = vi.hoisted(() => ({ getDesktopConversationStatistics: vi.fn() }))
vi.mock('@/api/desktopConversations', () => api)
vi.mock('vue-chartjs', () => ({ Line: { template: '<div data-testid="conversation-trend-chart" />' } }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import DesktopConversationStatistics from '../DesktopConversationStatistics.vue'

const statistics = {
  timezone: 'Asia/Shanghai', as_of: '2026-09-22T04:00:00Z',
  today: { record_count: 3, prompt_count: 5 }, week: { record_count: 12, prompt_count: 20 },
  month: { record_count: 30, prompt_count: 45 }, total: { record_count: 8, prompt_count: 15, duration_record_count: 6, total_duration_ms: 360_000, average_duration_ms: 60_000 },
  last_30_days: { record_count: 30, prompt_count: 45 },
  captured_last_30_days: 25, response_missing_last_30_days: 5,
  workbuddy_last_30_days: 10, chatgpt_codex_last_30_days: 20,
  distinct_members: 4, distinct_sessions: 6, captured: 7, response_missing: 1, workbuddy: 3, chatgpt_codex: 5,
  daily: [{ date: '2026-09-21', record_count: 3, prompt_count: 6 }, { date: '2026-09-22', record_count: 5, prompt_count: 9 }],
  range_start: '2026-09-21', range_end: '2026-09-22',
}

describe('Desktop conversation usage analytics', () => {
  beforeEach(() => { api.getDesktopConversationStatistics.mockReset().mockResolvedValue(statistics) })

  it.each([false, true])('shows selected counts and trend for selfManaged=%s', async selfManaged => {
    const wrapper = mount(DesktopConversationStatistics, { props: { organizationId: 'org_one', selfManaged, filters: {}, range: { days: 30 }, view: 'usage' } })
    await flushPromises()
    expect(api.getDesktopConversationStatistics).toHaveBeenCalledWith('org_one', selfManaged, {}, expect.any(AbortSignal), { days: 30 })
    expect(wrapper.find('[data-testid="statistics-today"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="conversation-usage-analysis"]').text()).toContain('8')
    expect(wrapper.find('[data-testid="conversation-usage-analysis"]').text()).toContain('6')
    expect(wrapper.find('[data-testid="conversation-usage-analysis"]').text()).toContain('6m 0s')
    expect(wrapper.find('[data-testid="conversation-usage-analysis"]').text()).toContain('1m 0s')
    expect(wrapper.find('[data-testid="conversation-trend-chart"]').exists()).toBe(true)
    await wrapper.setProps({ range: { days: 7 } })
    await flushPromises()
    expect(api.getDesktopConversationStatistics).toHaveBeenLastCalledWith('org_one', selfManaged, {}, expect.any(AbortSignal), { days: 7 })
    wrapper.unmount()
  })

  it('shows an empty state without a blank chart', async () => {
    api.getDesktopConversationStatistics.mockResolvedValue({ ...statistics, total: { record_count: 0, prompt_count: 0, duration_record_count: 0, total_duration_ms: 0, average_duration_ms: null }, daily: [{ date: '2026-09-22', record_count: 0, prompt_count: 0 }] })
    const wrapper = mount(DesktopConversationStatistics, { props: { organizationId: 'org_one', selfManaged: false, filters: {}, range: { days: 7 }, view: 'usage' } })
    await flushPromises()
    expect(wrapper.find('[data-testid="conversation-trend-chart"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('statistics.noRecords')
    wrapper.unmount()
  })
})
