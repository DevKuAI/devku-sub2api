import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { DesktopOrganizationUsageStatistics as UsageStatistics } from '@/api/desktopOrganizationUsage'

const api = vi.hoisted(() => ({ getDesktopOrganizationUsageStatistics: vi.fn() }))
vi.mock('@/api/desktopOrganizationUsage', () => api)
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<div data-testid="usage-trend-chart" />' } }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import DesktopOrganizationUsageStatistics from '../DesktopOrganizationUsageStatistics.vue'

const statistics: UsageStatistics = {
  timezone: 'Asia/Shanghai', as_of: '2026-09-22T04:00:00Z',
  today: { total_tokens: 1250, actual_cost: 0.125 },
  week: { total_tokens: 2_500_000, actual_cost: 2.5 },
  month: { total_tokens: 8_000_000, actual_cost: 12.75 },
  total: { total_tokens: 4_000_000_000, actual_cost: 150.123456 },
  last_30_days: { total_tokens: 3000, actual_cost: 1.5 },
  selected: { total_tokens: 3000, actual_cost: 1.5 },
  previous: { total_tokens: 1500, actual_cost: 0.75 },
  range_start: '2026-08-24', range_end: '2026-09-22', previous_start: '2026-07-25', previous_end: '2026-08-23',
  daily: [{ date: '2026-09-22', total_tokens: 3000, actual_cost: 1.5 }],
  breakdown: { input_tokens: 100, output_tokens: 200, cache_creation_tokens: 300, cache_read_tokens: 2400 },
  models: [
    { model: 'model-one', requests: 2, total_tokens: 1000, actual_cost: 1, cost_rank: 1, token_rank: 2 },
    { model: 'model-two', requests: 1, total_tokens: 2000, actual_cost: 0.5, cost_rank: 2, token_rank: 1 },
  ],
  members: [
    { member_id: 'mem_one', name: 'Member One', deleted: true, requests: 2, total_tokens: 1000, actual_cost: 1, cost_rank: 1, token_rank: 2 },
    { member_id: 'mem_two', name: 'Member Two', deleted: false, requests: 1, total_tokens: 2000, actual_cost: 0.5, cost_rank: 2, token_rank: 1 },
  ],
  observed_members: 2,
}
const view = (selfManaged = false, mode: 'summary' | 'insights' = 'summary') => mount(DesktopOrganizationUsageStatistics, { props: { organizationId: 'org_one', selfManaged, view: mode } })

describe('Desktop organization usage statistics', () => {
  beforeEach(() => { api.getDesktopOrganizationUsageStatistics.mockReset().mockResolvedValue(statistics) })

  it.each([false, true])('loads whole-organization usage for selfManaged=%s with precise costs and token tooltips', async (selfManaged) => {
    const wrapper = view(selfManaged)
    await flushPromises()
    expect(api.getDesktopOrganizationUsageStatistics).toHaveBeenCalledWith('org_one', selfManaged, expect.any(AbortSignal))
    expect(wrapper.find('[data-testid="organization-usage-today"]').text()).toContain('$0.1250')
    expect(wrapper.find('[data-testid="organization-usage-week"]').text()).toContain('2.5M')
    expect(wrapper.find('[data-testid="organization-usage-month"]').text()).toContain('$12.7500')
    expect(wrapper.find('[data-testid="organization-usage-total"]').text()).toContain('$150.1235')
    expect(wrapper.find('[data-testid="organization-usage-total"] [title]').attributes('title')).toBe((4_000_000_000).toLocaleString())
    expect(wrapper.find('[data-testid="usage-trend-chart"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each([false, true])('shows trends and rankings only in the usage tab for selfManaged=%s', async (selfManaged) => {
    const wrapper = view(selfManaged, 'insights')
    await flushPromises()
    expect(api.getDesktopOrganizationUsageStatistics).toHaveBeenCalledWith('org_one', selfManaged, expect.any(AbortSignal), undefined)
    expect(wrapper.find('[data-testid="organization-usage-today"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="usage-trend-chart"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="organization-last-30-days"]').text()).toContain('$1.5000')
    expect(wrapper.find('[data-testid="organization-model-ranking"] li').text()).toContain('model-one')
    expect(wrapper.find('[data-testid="organization-member-ranking"]').text()).toContain('Member One')
    await wrapper.find('[aria-pressed="false"]').trigger('click')
    expect(wrapper.find('[data-testid="organization-model-ranking"] li').text()).toContain('model-two')
    expect(wrapper.find('[data-testid="organization-model-ranking"] li').text()).toContain('2.0K')
    expect(wrapper.find('[data-testid="organization-member-ranking"] li').text()).toContain('Member Two')
    await wrapper.setProps({ range: { days: 7 } })
    await flushPromises()
    expect(api.getDesktopOrganizationUsageStatistics).toHaveBeenLastCalledWith('org_one', selfManaged, expect.any(AbortSignal), { days: 7 })
    wrapper.unmount()
  })

  it('shows zero usage as zero and refreshes independently', async () => {
    const empty = { total_tokens: 0, actual_cost: 0 }
    api.getDesktopOrganizationUsageStatistics.mockResolvedValue({ ...statistics, today: empty, week: empty, month: empty, total: empty, last_30_days: empty, selected: empty, daily: [], models: [], members: [], observed_members: 0 })
    const wrapper = view()
    await flushPromises()
    expect(wrapper.find('[data-testid="organization-usage-total"]').text()).toContain('$0.0000')
    expect(wrapper.find('[data-testid="organization-usage-total"] [title]').attributes('title')).toBe('0')
    expect(wrapper.find('[data-testid="usage-trend-chart"]').exists()).toBe(false)
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(api.getDesktopOrganizationUsageStatistics).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('clears stale totals during organization and role changes and aborts on unmount', async () => {
    let resolve!: (value: UsageStatistics) => void
    api.getDesktopOrganizationUsageStatistics.mockReturnValueOnce(new Promise<UsageStatistics>(done => { resolve = done }))
    const wrapper = view()
    expect(wrapper.find('[data-testid="organization-usage-total"]').text()).toContain('—')
    const firstSignal = api.getDesktopOrganizationUsageStatistics.mock.calls[0][2] as AbortSignal
    api.getDesktopOrganizationUsageStatistics.mockResolvedValue({ ...statistics, total: { total_tokens: 100, actual_cost: 9 } })
    await wrapper.setProps({ organizationId: 'org_two', selfManaged: true })
    await flushPromises()
    expect(firstSignal.aborted).toBe(true)
    resolve(statistics)
    await flushPromises()
    expect(wrapper.find('[data-testid="organization-usage-total"]').text()).toContain('$9.0000')
    expect(wrapper.text()).not.toContain('$150.1235')
    expect(api.getDesktopOrganizationUsageStatistics).toHaveBeenLastCalledWith('org_two', true, expect.any(AbortSignal))
    wrapper.unmount()
    expect((api.getDesktopOrganizationUsageStatistics.mock.lastCall?.[2] as AbortSignal).aborted).toBe(true)
  })

  it('shows an error instead of zero charges and supports retry', async () => {
    api.getDesktopOrganizationUsageStatistics.mockRejectedValueOnce(new Error('offline'))
    const wrapper = view()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('usageStatistics.loadFailed')
    expect(wrapper.find('[data-testid="organization-usage-total"]').exists()).toBe(false)
    await wrapper.find('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('$150.1235')
    wrapper.unmount()
  })

  it('waits for an organization identity before loading', async () => {
    const wrapper = mount(DesktopOrganizationUsageStatistics, { props: { organizationId: '', selfManaged: true } })
    await flushPromises()
    expect(api.getDesktopOrganizationUsageStatistics).not.toHaveBeenCalled()
    await wrapper.setProps({ organizationId: 'org_one' })
    await flushPromises()
    expect(api.getDesktopOrganizationUsageStatistics).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
