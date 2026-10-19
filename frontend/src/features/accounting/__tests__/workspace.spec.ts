import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { i18n, organization, record, stubs } from './testing'

const api = vi.hoisted(() => ({
  summary: vi.fn(),
  organizations: vi.fn(),
  workflow: vi.fn(),
  orders: vi.fn(),
  listRecords: vi.fn(),
  attachments: vi.fn(),
  exportCSV: vi.fn(),
  download: vi.fn(),
  mutate: vi.fn(),
  upload: vi.fn(),
  stableRequest: () => Object.assign(() => 'stable-key', { reset: vi.fn() }),
}))
const access = vi.hoisted(() => ({ refreshDesktopOrganizationAccess: vi.fn() }))
const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))
const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }))
vi.mock('../api', () => api)
vi.mock('@/composables/useDesktopOrganizationAccess', () => ({ useDesktopOrganizationAccess: () => access }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router }))
vi.mock('file-saver', () => ({ saveAs: vi.fn() }))

import AccountingWorkspace from '../AccountingWorkspace.vue'

enableAutoUnmount(afterEach)

const DataTable = {
  name: 'DataTable',
  props: ['data', 'columns'],
  emits: ['row-click'],
  template:
    '<table><tr v-for="row in data" :key="row.id" data-testid="row" @click="$emit(\'row-click\', row)"><td v-for="column in columns" :key="column.key"><slot :name="`cell-${column.key}`" :row="row" /></td></tr></table>',
}
const summary = { currency: 'CNY', received: '1000', allocated: '600', consumed: '200', refunded: '0', balance: '400', unallocated: '400', unmatched: 1 }

async function render(customer = false, query: Record<string, string> = {}) {
  Object.assign(route, reactive({ query }))
  const wrapper = mount(AccountingWorkspace, {
    props: { customer, organizationId: customer ? '' : 'org-one', administrator: true },
    global: { plugins: [i18n()], stubs: { ...stubs, DataTable, Pagination: true } },
  })
  await flushPromises()
  return wrapper
}

describe('accounting workspace', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.organizations.mockResolvedValue([organization])
    api.summary.mockResolvedValue([summary])
    api.workflow.mockResolvedValue({ items: [{ kind: 'transfers', id: 'record-one', name: '客户付款', reason: 'draft', amount: '100', currency: 'CNY', status: 'DRAFT' }], total: 1, counts: { draft: 1 }, positions: {}, pending_consumption: [] })
    api.listRecords.mockResolvedValue({ items: [record({ details: { payer_name: '客户付款', bank_reference: 'BANK-1', remark: '内部备注' } })], total: 1 })
    api.attachments.mockResolvedValue([])
    api.orders.mockResolvedValue({ items: [], total: 0 })
    access.refreshDesktopOrganizationAccess.mockResolvedValue({ public_id: 'org-one', name: '上海星海智能科技有限公司' })
  })

  it('keeps the workspace shell visible without flashing the internal organization ID while loading', async () => {
    let finish!: (value: typeof organization[]) => void
    api.organizations.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = await render()
    expect(wrapper.get('h1').text()).not.toContain('org-one')
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(7)
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
    expect(api.workflow).not.toHaveBeenCalled()
    finish([organization]); await flushPromises()
    expect(wrapper.get('h1').text()).toBe(organization.name)
  })

  it('does not flash an empty financial overview before the summary arrives', async () => {
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = await render()
    expect(wrapper.text()).not.toContain('还没有已确认的资金记录。')
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="accounting-overview"]').exists()).toBe(false)
    finish([summary]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('可用资金')
  })

  it('shows a retryable error instead of an empty balance when the summary fails', async () => {
    api.summary.mockRejectedValueOnce({ message: '资金汇总加载失败' })
    const wrapper = await render()
    expect(wrapper.get('[role="alert"]').text()).toContain('资金汇总加载失败')
    expect(wrapper.find('[data-testid="accounting-overview"]').exists()).toBe(false)
    expect(wrapper.attributes('aria-busy')).toBe('false')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('可用资金')
  })

  it('shows the real empty state only after a successful empty summary', async () => {
    api.summary.mockResolvedValueOnce([])
    const wrapper = await render()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('还没有已确认的资金记录。')
    expect(wrapper.attributes('aria-busy')).toBe('false')
  })

  it('keeps the current customer overview mounted when refreshing the same organization', async () => {
    const wrapper = await render(true)
    const panel = wrapper.get('[data-testid="accounting-overview"]').element
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    window.dispatchEvent(new Event('focus')); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').element).toBe(panel)
    finish([summary]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').element).toBe(panel)
    expect(wrapper.attributes('aria-busy')).toBe('false')
  })

  it('hides the previous customer balance while a changed organization loads', async () => {
    const wrapper = await render(true)
    access.refreshDesktopOrganizationAccess.mockResolvedValueOnce({ public_id: 'org-two', name: '另一家企业' })
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    window.dispatchEvent(new Event('focus')); await flushPromises()
    expect(wrapper.text()).toContain('另一家企业')
    expect(wrapper.find('[data-testid="accounting-overview"]').exists()).toBe(false)
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
    finish([{ ...summary, balance: '900' }]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('900.00')
  })

  it('ignores an older customer summary that finishes after a newer refresh', async () => {
    const wrapper = await render(true)
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    window.dispatchEvent(new Event('focus')); await flushPromises()
    api.summary.mockResolvedValueOnce([{ ...summary, balance: '900' }])
    window.dispatchEvent(new Event('focus')); await flushPromises()
    finish([{ ...summary, balance: '25' }]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('900.00')
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).not.toContain('25.00')
    expect(wrapper.attributes('aria-busy')).toBe('false')
  })

  it('groups an organization’s books into sections with balances, orders and to-dos first', async () => {
    const wrapper = await render()

    expect(wrapper.get('h1').text()).toBe('上海星海智能科技有限公司')
    expect(wrapper.findAll('[role="tab"]').map((tab) => tab.text())).toEqual(['概览', '订单', '收款', '用卡', '合同票据', '支出', '核对结账'])
    expect(wrapper.text()).toContain('可用资金')
    expect(wrapper.get('[data-testid="accounting-todos"]').text()).toContain('客户付款')
    expect(api.workflow).toHaveBeenCalledWith('workbench', 'org-one')
    expect(api.orders).toHaveBeenCalledWith('org-one', expect.objectContaining({ state: 'OPEN' }))
  })

  it('opens a record type from the URL inside its section', async () => {
    const wrapper = await render(false, { tab: 'transfers' })

    expect(wrapper.get('[role="tab"][aria-selected="true"]').text()).toBe('收款')
    expect(wrapper.get('[aria-pressed="true"]').text()).toBe('到账')
    expect(api.listRecords).toHaveBeenCalledWith('transfers', false, expect.objectContaining({ organization_id: 'org-one', page: 1 }), expect.anything())

    await wrapper.get('[data-testid="row"]').trigger('click')
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminAccountingRecord', params: { organizationId: 'org-one', kind: 'transfers', id: 'record-one' } })

    await wrapper.findAll('[role="tab"]')[3]!.trigger('click')
    expect(router.replace).toHaveBeenCalledWith({ query: { tab: 'vcc-cards' } })
  })

  it('maps the old workbench tab to the overview', async () => {
    const wrapper = await render(false, { tab: 'workbench' })
    expect(wrapper.get('[role="tab"][aria-selected="true"]').text()).toBe('概览')
  })

  it('applies list filters as soon as they change', async () => {
    const wrapper = await render(false, { tab: 'transfers' })
    api.listRecords.mockClear()
    await wrapper.get('[data-testid="record-list"] select').setValue('RECEIVED')
    await flushPromises()
    expect(api.listRecords).toHaveBeenCalledWith('transfers', false, expect.objectContaining({ status: 'RECEIVED' }), expect.anything())
  })

  it('does not fall back to other organizations when the organization is missing', async () => {
    api.organizations.mockResolvedValue([])
    const wrapper = await render()
    expect(wrapper.text()).toContain('企业不存在或已不可用')
    expect(wrapper.find('[role="tab"]').exists()).toBe(false)
  })

  it('gives customers a read-only view of their own shared records', async () => {
    const wrapper = await render(true, { tab: 'transfers' })

    expect(wrapper.findAll('[role="tab"]').map((tab) => tab.text())).toEqual(['概览', '到账', 'VCC 卡片', '卡交易', '合同', '发票', '对账单'])
    expect(wrapper.text()).not.toContain('新增')
    expect(wrapper.text()).not.toContain('导出')
    expect(api.listRecords).toHaveBeenCalledWith('transfers', true, expect.objectContaining({ organization_id: undefined }), expect.anything())
    // Customers only receive received transfers, so a status filter would offer nothing.
    expect(wrapper.find('[data-testid="record-list"] select').exists()).toBe(false)

    await wrapper.get('[data-testid="row"]').trigger('click')
    await flushPromises()
    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).not.toContain('内部备注')
    expect(dialog.text()).not.toContain('BANK-1')
    expect(api.attachments).toHaveBeenCalledWith('transfers', 'record-one', true)
    expect(router.push).not.toHaveBeenCalled()
  })
})
