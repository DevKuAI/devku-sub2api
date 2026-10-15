import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import { adminKinds, fields } from '../fields'
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
  exportCSV: vi.fn(),
  upload: vi.fn(),
}))
const access = vi.hoisted(() => ({
  refreshDesktopOrganizationAccess: vi.fn(),
}))
vi.mock('../api', () => api)
vi.mock('@/composables/useDesktopOrganizationAccess', () => ({
  useDesktopOrganizationAccess: () => access,
}))
const route = { query: {} as Record<string, string> }
const router = { replace: vi.fn() }
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => router,
}))
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
function render(customer = false, organizationId = customer ? '' : 'org_one') {
  return mount(Workspace, {
    props: { customer, organizationId },
    global: {
      plugins: [
        createI18n({
          legacy: false,
          locale: 'zh',
          messages: { zh },
          messageCompiler: (message) => (context) =>
            String(message).replace(/\{(\w+)\}/g, (_match, key: string) =>
              String(context.named(key)),
            ),
        }),
      ],
      stubs: {
        RouterLink: { props: ['to'], template: '<a><slot /></a>' },
        BaseDialog: {
          props: ['show', 'title'],
          template:
            '<section v-if="show" class="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>',
        },
      },
    },
  })
}
describe('accounting workspace', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.query = {}
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
    expect(wrapper.findAll('form select')).toHaveLength(1)
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
  it('starts at an enterprise directory without global record tabs or create actions', async () => {
    const wrapper = render(false, '')
    await flushPromises()
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(0)
    expect(
      wrapper
        .findAll('button')
        .some((button) => button.text().startsWith('新增')),
    ).toBe(false)
    expect(
      wrapper.findAllComponents({ name: 'AccountingWorkspace' }),
    ).toHaveLength(0)
    expect(wrapper.find('.dialog').exists()).toBe(false)
    expect(api.listRecords).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('暂无账目记录')
    expect(wrapper.get('a').text()).toBe('客户公司')
    wrapper.unmount()
  })

  it.each(adminKinds.filter((kind) => kind !== 'expense-payments'))(
    'preserves every %s input within the current enterprise',
    async (kind) => {
      const wrapper = render()
      await flushPromises()
      const label = zh.accounting.kinds[kind]
      await wrapper
        .findAll('[role="tab"]')
        .find((button) => button.text() === label)!
        .trigger('click')
      await flushPromises()
      await wrapper
        .findAll('button')
        .find((button) => button.text().startsWith('新增'))!
        .trigger('click')
      await flushPromises()
      const dialog = wrapper.findAll('.dialog').at(-1)!
      expect(dialog.text()).toContain('客户公司')
      const rendered = dialog
        .findAll('[data-field]')
        .map((field) => field.attributes('data-field'))
        .sort()
      expect(rendered).toEqual(fields[kind].map((field) => field.key).sort())
      expect(new Set(rendered).size).toBe(rendered.length)
      expect(
        dialog.findAll('option').some((option) => option.text() === '客户公司'),
      ).toBe(false)
      if (kind === 'vcc-cards') {
        expect(dialog.findAll('[type="password"]')).toHaveLength(4)
        expect(dialog.get('[data-field-group="secrets"]').text()).toContain(
          '卡片敏感信息',
        )
      }
      wrapper.unmount()
    },
  )

  it('saves a card under the current enterprise without asking for it again', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper
      .findAll('[role="tab"]')
      .find((button) => button.text() === 'VCC 卡片')!
      .trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '新增VCC 卡片')!
      .trigger('click')
    await flushPromises()
    const dialog = wrapper.findAll('.dialog').at(-1)!
    await dialog.get('[data-field="alias"] input').setValue('专用卡')
    await dialog
      .get('[data-field="remark"] textarea')
      .setValue('第一行\n第二行')
    await dialog.get('form').trigger('submit')
    await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith(
      'vcc-cards',
      expect.objectContaining({
        organization_id: 'org_one',
        currency: 'CNY',
        amount: '0',
        details: expect.objectContaining({
          alias: '专用卡',
          remark: '第一行\n第二行',
        }),
      }),
      'stable-key',
      'post',
    )
    wrapper.unmount()
  })

  it('keeps lookups, filters, reconciliation, audit and export inside the enterprise', async () => {
    api.listRecords.mockResolvedValue({
      items: [
        record,
        {
          ...record,
          id: 'other',
          organization_id: 'org_other',
          details: { name: 'OTHER_COMPANY_RECORD' },
        },
      ],
      total: 2,
    })
    const wrapper = render()
    await flushPromises()
    for (const label of ['对账', '审计日志', '我方支出']) {
      await wrapper
        .findAll('[role="tab"]')
        .find((button) => button.text() === label)!
        .trigger('click')
      await flushPromises()
    }
    expect(api.reconciliation).toHaveBeenCalledWith(
      expect.objectContaining({ organization_id: 'org_one' }),
    )
    expect(api.auditLogs).toHaveBeenCalledWith(
      expect.objectContaining({ organization_id: 'org_one' }),
    )
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '导出 CSV')!
      .trigger('click')
    await flushPromises()
    expect(api.exportCSV).toHaveBeenCalledWith(
      'expenses',
      expect.objectContaining({ organization_id: 'org_one' }),
    )
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '新增我方支出')!
      .trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.dialog').at(-1)!.text()).not.toContain(
      'OTHER_COMPANY_RECORD',
    )
    expect(
      api.listRecords.mock.calls.every(
        (call) => call[2].organization_id === 'org_one',
      ),
    ).toBe(true)
    wrapper.unmount()
  })

  it('does not fall back to another enterprise when a direct URL is unavailable', async () => {
    const wrapper = render(false, 'org_missing')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('未找到当前企业')
    expect(api.summary).toHaveBeenCalledWith(
      false,
      expect.objectContaining({ organization_id: 'org_missing' }),
      expect.anything(),
    )
    expect(api.listRecords).not.toHaveBeenCalled()
    expect(api.mutate).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('keeps the payment fields and initiates payments from the selected enterprise expense', async () => {
    api.listRecords.mockResolvedValue({
      items: [
        { ...record, status: 'PENDING', details: { supplier: '示例供应商' } },
      ],
      total: 1,
    })
    const wrapper = render()
    await flushPromises()
    await wrapper
      .findAll('[role="tab"]')
      .find((button) => button.text() === '我方支出')!
      .trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '记录详情')!
      .trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '记录付款')!
      .trigger('click')
    await flushPromises()
    const dialog = wrapper.findAll('.dialog').at(-1)!
    expect(
      dialog
        .findAll('[data-field]')
        .map((field) => field.attributes('data-field'))
        .sort(),
    ).toEqual(fields['expense-payments'].map((field) => field.key).sort())
    expect(api.attachments).toHaveBeenCalledWith(
      'expenses',
      'receipt-one',
      false,
    )
    expect(dialog.text()).toContain('客户公司')
    wrapper.unmount()
  })

  it('lets the compact navigation open the same scoped records and persist the section', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper
      .get('.accounting-mobile-navigation select')
      .setValue('invoices')
    await flushPromises()
    expect(api.listRecords).toHaveBeenLastCalledWith(
      'invoices',
      false,
      expect.objectContaining({ organization_id: 'org_one' }),
      expect.anything(),
    )
    expect(router.replace).toHaveBeenCalledWith({ query: { tab: 'invoices' } })
    wrapper.unmount()
  })
})
