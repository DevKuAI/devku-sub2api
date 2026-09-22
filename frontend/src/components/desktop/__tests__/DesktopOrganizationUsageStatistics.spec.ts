import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { DesktopOrganizationUsageStatistics as UsageStatistics } from '@/api/desktopOrganizationUsage'

const api = vi.hoisted(() => ({ getDesktopOrganizationUsageStatistics: vi.fn() }))
vi.mock('@/api/desktopOrganizationUsage', () => api)
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import DesktopOrganizationUsageStatistics from '../DesktopOrganizationUsageStatistics.vue'

const statistics: UsageStatistics = {
  timezone: 'Asia/Shanghai', as_of: '2026-09-22T04:00:00Z',
  today: { total_tokens: 1250, actual_cost: 0.125 },
  week: { total_tokens: 2_500_000, actual_cost: 2.5 },
  month: { total_tokens: 8_000_000, actual_cost: 12.75 },
  total: { total_tokens: 4_000_000_000, actual_cost: 150.123456 },
}
const view = (selfManaged = false) => mount(DesktopOrganizationUsageStatistics, { props: { organizationId: 'org_one', selfManaged } })

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
    wrapper.unmount()
  })

  it('shows zero usage as zero and refreshes independently', async () => {
    const empty = { total_tokens: 0, actual_cost: 0 }
    api.getDesktopOrganizationUsageStatistics.mockResolvedValue({ ...statistics, today: empty, week: empty, month: empty, total: empty })
    const wrapper = view()
    await flushPromises()
    expect(wrapper.find('[data-testid="organization-usage-total"]').text()).toContain('$0.0000')
    expect(wrapper.find('[data-testid="organization-usage-total"] [title]').attributes('title')).toBe('0')
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
