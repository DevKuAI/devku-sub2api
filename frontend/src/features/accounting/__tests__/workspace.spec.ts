import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import type { AccountingRecord } from '../types'
const api = vi.hoisted(() => ({
  summary: vi.fn(),
  organizations: vi.fn(),
  listRecords: vi.fn(),
  trend: vi.fn(),
  attachments: vi.fn(),
  allocationHistory: vi.fn(),
  mutate: vi.fn(),
  stableRequest: () => Object.assign(() => 'stable-key', { reset: vi.fn() }),
  reconciliation: vi.fn(),
  auditLogs: vi.fn(),
}))
const access = vi.hoisted(() => ({
  refreshDesktopOrganizationAccess: vi.fn(),
}))
vi.mock('../api', () => api)
vi.mock('@/composables/useDesktopOrganizationAccess', () => ({
  useDesktopOrganizationAccess: () => access,
}))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('file-saver', () => ({ saveAs: vi.fn() }))
import Workspace from '../AccountingWorkspace.vue'
enableAutoUnmount(afterEach)
const record: AccountingRecord = {
  id: 'receipt-one',
  organization_id: 'org_one',
  status: 'DRAFT',
  currency: 'CNY',
  amount: '100',
  occurred_at: '2026-10-12T00:00:00Z',
  details: { payer_name: '客户公司', sales_amount: '100' },
  customer_visible: false,
  version: 1,
  created_at: '2026-10-12T00:00:00Z',
}
function render(customer = false) {
  return mount(Workspace, {
    props: { customer },
    global: {
      plugins: [
        createI18n({
          legacy: false,
          locale: 'zh',
          messages: { zh },
          messageCompiler: (message) => () => String(message),
        }),
      ],
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          template:
            '<section v-if="show" class="dialog"><h2>{{ title }}</h2><slot /></section>',
        },
      },
    },
  })
}
describe('accounting workspace', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    access.refreshDesktopOrganizationAccess.mockResolvedValue({
      public_id: 'org_one',
    })
    api.summary.mockResolvedValue([])
    api.organizations.mockResolvedValue([
      { id: 'org_one', name: '客户公司', summary: [] },
    ])
    api.trend.mockResolvedValue([])
    api.listRecords.mockResolvedValue({ items: [record], total: 1 })
    api.attachments.mockResolvedValue([])
    api.allocationHistory.mockResolvedValue([])
    api.mutate.mockResolvedValue(record)
    api.reconciliation.mockResolvedValue({ items: [], total: 0 })
    api.auditLogs.mockResolvedValue({ items: [], total: 0 })
  })
  it('keeps the linked-account view available without financial records', async () => {
    const wrapper = render(true)
    await flushPromises()
    expect(wrapper.text()).toContain('财务账目')
    expect(wrapper.text()).toContain('暂无账目记录')
    expect(wrapper.text()).toContain('企业已挂载')
    expect(api.organizations).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('我方支出')
    expect(wrapper.findAll('select')).toHaveLength(1)
    wrapper.unmount()
  })
  it('renders all customer summary labels without unresolved translation keys', async () => {
    api.summary.mockResolvedValue([
      {
        currency: 'CNY',
        received: '100',
        allocated: '60',
        consumed: '10',
        refunded: '0',
        balance: '50',
        unallocated: '40',
        unmatched: 0,
      },
    ])
    const wrapper = render(true)
    await flushPromises()
    expect(wrapper.text()).toContain('已消费')
    expect(wrapper.text()).not.toContain('accounting.metrics.')
    wrapper.unmount()
  })
  it('offers no write actions in the customer record detail', async () => {
    const wrapper = render(true)
    await flushPromises()
    await wrapper
      .findAll('[role="tab"]')
      .find((b) => b.text() === '到账记录')!
      .trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '记录详情')!
      .trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('确认记录')
    expect(wrapper.text()).not.toContain('编辑草稿')
    expect(wrapper.text()).not.toContain('上传凭证')
    expect(api.mutate).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('filters unexpected private fields even if a server fixture includes them', async () => {
    api.listRecords.mockResolvedValue({
      items: [
        {
          ...record,
          details: {
            payer_name: '客户公司',
            receiving_account: 'PRIVATE-BANK',
            remark: 'PRIVATE-NOTE',
          },
        },
      ],
      total: 1,
    })
    const wrapper = render(true)
    await flushPromises()
    await wrapper
      .findAll('[role="tab"]')
      .find((b) => b.text() === '到账记录')!
      .trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '记录详情')!
      .trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('客户公司')
    expect(wrapper.text()).not.toContain('PRIVATE-BANK')
    expect(wrapper.text()).not.toContain('PRIVATE-NOTE')
    wrapper.unmount()
  })
  it('allows an admin to confirm a selected draft through the real action interface', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper
      .findAll('[role="tab"]')
      .find((b) => b.text() === '到账记录')!
      .trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '记录详情')!
      .trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '确认记录')!
      .trigger('click')
    await flushPromises()
    await wrapper.findAll('form').at(-1)!.trigger('submit')
    await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith(
      'transfers/receipt-one/confirm',
      { reason: '' },
      'stable-key',
      'post',
    )
    wrapper.unmount()
  })
  it('refreshes organization access on window focus and cleans the listener up', async () => {
    const wrapper = render(true)
    await flushPromises()
    access.refreshDesktopOrganizationAccess.mockClear()
    window.dispatchEvent(new Event('focus'))
    await flushPromises()
    expect(access.refreshDesktopOrganizationAccess).toHaveBeenCalledWith(true)
    wrapper.unmount()
    access.refreshDesktopOrganizationAccess.mockClear()
    window.dispatchEvent(new Event('focus'))
    expect(access.refreshDesktopOrganizationAccess).not.toHaveBeenCalled()
  })
  it('shows a recoverable error when data loading fails', async () => {
    api.summary.mockRejectedValue({ reason: 'ACCOUNTING_STORAGE_UNAVAILABLE' })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('存储')
    wrapper.unmount()
  })
})
