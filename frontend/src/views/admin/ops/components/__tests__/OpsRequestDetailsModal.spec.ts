import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { OpsDashboardOverview } from '@/api/admin/ops'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import OpsDashboardHeader from '../OpsDashboardHeader.vue'
import OpsRequestDetailsModal from '../OpsRequestDetailsModal.vue'

const { listRequestDetails, getBIOverview, viewport } = vi.hoisted(() => ({
  listRequestDetails: vi.fn(),
  getBIOverview: vi.fn(),
  viewport: { desktop: true },
}))

vi.mock('@vueuse/core', () => ({ useMediaQuery: () => ref(viewport.desktop) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { listRequestDetails } }))
vi.mock('@/api', () => ({ adminAPI: { groups: { getAll: vi.fn().mockResolvedValue([]) }, bi: { getOverview: getBIOverview } } }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
  useAdminSettingsStore: () => ({ opsRealtimeMonitoringEnabled: false }),
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

async function openDetails(sort: 'duration_desc' | 'ttft_desc') {
  const wrapper = mount(OpsRequestDetailsModal, {
    props: { modelValue: false, timeRange: '1h', preset: { title: 'Details', sort } },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Pagination: true } },
  })
  await wrapper.setProps({ modelValue: true })
  await flushPromises()
  return wrapper
}

const routerLinkStub = { props: ['to'], template: '<a :href="to"><slot /></a>' }

describe('Ops request latency details', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    viewport.desktop = true
    getBIOverview.mockResolvedValue({})
    listRequestDetails.mockResolvedValue({
      items: [
        { kind: 'success', created_at: '2026-09-10T00:00:00Z', duration_ms: 12000, first_token_ms: 800 },
        { kind: 'success', created_at: '2026-09-10T00:00:01Z', duration_ms: 9000, first_token_ms: 0 },
        { kind: 'success', created_at: '2026-09-10T00:00:02Z', duration_ms: 5000, first_token_ms: null },
      ],
      total: 3,
    })
  })

  it('shows the BI Operations link only when BI is available', async () => {
    const props = { platform: '', groupId: null, timeRange: '1h', queryMode: 'auto', loading: false, lastUpdated: null }
    const wrapper = shallowMount(OpsDashboardHeader, { props, global: { stubs: { RouterLink: routerLinkStub } } })
    await flushPromises()
    const link = wrapper.find('a[href="/admin/bi"]')
    expect(link.exists()).toBe(true)
    expect(link.element.previousElementSibling).toBe(wrapper.find('h1').element)
    expect(link.find('svg').exists()).toBe(false)
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    await wrapper.setProps({ fullscreen: true })
    expect(wrapper.find('a[href="/admin/bi"]').exists()).toBe(false)
    wrapper.unmount()

    getBIOverview.mockRejectedValueOnce(new Error('BI unavailable'))
    const unavailable = shallowMount(OpsDashboardHeader, { props, global: { stubs: { RouterLink: routerLinkStub } } })
    await flushPromises()
    expect(unavailable.find('a[href="/admin/bi"]').exists()).toBe(false)
    unavailable.unmount()
  })

  it('opens the TTFT card with first-token sorting and successful requests', async () => {
    const wrapper = shallowMount(OpsDashboardHeader, {
      props: { overview: {} as OpsDashboardOverview, platform: '', groupId: null, timeRange: '1h', queryMode: 'auto', loading: false, lastUpdated: null },
      global: { stubs: { RouterLink: routerLinkStub } },
    })
    await flushPromises()
    const button = wrapper.findAll('button').find((item) =>
      item.text() === 'admin.ops.requestDetails.details' &&
      item.element.parentElement?.textContent?.includes('TTFT'),
    )
    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('openRequestDetails')).toEqual([[
      { title: 'admin.ops.ttftLabel', kind: 'success', sort: 'ttft_desc' },
    ]])
    wrapper.unmount()
  })

  it.each([true, false])('shows TTFT rather than total duration (desktop: %s)', async (desktop) => {
    viewport.desktop = desktop
    const wrapper = await openDetails('ttft_desc')
    expect(listRequestDetails).toHaveBeenCalledWith(expect.objectContaining({ sort: 'ttft_desc' }))
    expect(wrapper.text()).toContain('admin.ops.ttftLabel')
    expect(wrapper.text()).toContain('800 ms')
    expect(wrapper.text()).toContain('0 ms')
    expect(wrapper.text()).not.toContain('12000 ms')
    expect(wrapper.text()).not.toContain('9000 ms')
    expect(wrapper.text()).not.toContain('5000 ms')
    if (desktop) expect(wrapper.findAll('tbody tr')[2].findAll('td')[4].text()).toBe('-')
    else expect(wrapper.text()).toContain('admin.ops.ttftLabel: -')
    wrapper.unmount()
  })

  it('keeps total duration for duration details', async () => {
    const wrapper = await openDetails('duration_desc')
    expect(listRequestDetails).toHaveBeenCalledWith(expect.objectContaining({ sort: 'duration_desc' }))
    expect(wrapper.text()).toContain('admin.ops.requestDetails.table.duration')
    expect(wrapper.text()).toContain('12000 ms')
    expect(wrapper.text()).not.toContain('800 ms')
    wrapper.unmount()
  })
})
