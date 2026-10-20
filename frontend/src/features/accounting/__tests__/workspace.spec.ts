import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { i18n, record, stubs } from './testing'

const api = vi.hoisted(() => ({
  summary: vi.fn(),
  workflow: vi.fn(),
  listRecords: vi.fn(),
  attachments: vi.fn(),
  download: vi.fn(),
  statementCSV: vi.fn(),
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

async function render(query: Record<string, string> = {}) {
  Object.assign(route, reactive({ query }))
  const wrapper = mount(AccountingWorkspace, {
    global: { plugins: [i18n()], stubs: { ...stubs, DataTable, Pagination: true } },
  })
  await flushPromises()
  return wrapper
}

describe('organization accounting workspace', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.summary.mockResolvedValue([summary])
    api.workflow.mockResolvedValue([])
    api.listRecords.mockResolvedValue({ items: [record({ status: 'RECEIVED', details: { payer_name: '客户付款', bank_reference: 'BANK-1', remark: '内部备注' } })], total: 1 })
    api.attachments.mockResolvedValue([])
    access.refreshDesktopOrganizationAccess.mockResolvedValue({ public_id: 'org-one', name: '上海星海智能科技有限公司' })
  })

  it('shows the organization’s own books in read-only sections', async () => {
    const wrapper = await render()
    expect(wrapper.get('h1').text()).toBe('财务账目')
    expect(wrapper.text()).toContain('上海星海智能科技有限公司')
    expect(wrapper.findAll('[role="tab"]').map((tab) => tab.text())).toEqual(['概览', '到账', 'VCC 卡片', '卡交易', '合同', '发票', '对账单'])
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('400.00')
    expect(api.summary).toHaveBeenCalledWith(true, {}, expect.anything())
  })

  it('does not flash an empty financial overview before the summary arrives', async () => {
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = await render()
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="accounting-overview"]').exists()).toBe(false)
    finish([summary]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('400.00')
  })

  it('shows a retryable error instead of an empty balance when the summary fails', async () => {
    api.summary.mockRejectedValueOnce({ message: '资金汇总加载失败' })
    const wrapper = await render()
    expect(wrapper.get('[role="alert"]').text()).toContain('资金汇总加载失败')
    expect(wrapper.find('[data-testid="accounting-overview"]').exists()).toBe(false)
    expect(wrapper.attributes('aria-busy')).toBe('false')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('400.00')
  })

  it('shows the real empty state only after a successful empty summary', async () => {
    api.summary.mockResolvedValueOnce([])
    const wrapper = await render()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('暂无账目')
  })

  it('keeps the current overview mounted when refreshing the same organization', async () => {
    const wrapper = await render()
    const panel = wrapper.get('[data-testid="accounting-overview"]').element
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    window.dispatchEvent(new Event('focus')); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').element).toBe(panel)
    finish([summary]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').element).toBe(panel)
  })

  it('hides the previous balance while a changed organization loads', async () => {
    const wrapper = await render()
    access.refreshDesktopOrganizationAccess.mockResolvedValueOnce({ public_id: 'org-two', name: '另一家企业' })
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    window.dispatchEvent(new Event('focus')); await flushPromises()
    expect(wrapper.text()).toContain('另一家企业')
    expect(wrapper.find('[data-testid="accounting-overview"]').exists()).toBe(false)
    finish([{ ...summary, balance: '900' }]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('900.00')
  })

  it('ignores an older summary that finishes after a newer refresh', async () => {
    const wrapper = await render()
    let finish!: (value: typeof summary[]) => void
    api.summary.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    window.dispatchEvent(new Event('focus')); await flushPromises()
    api.summary.mockResolvedValueOnce([{ ...summary, balance: '900' }])
    window.dispatchEvent(new Event('focus')); await flushPromises()
    finish([{ ...summary, balance: '25' }]); await flushPromises()
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).toContain('900.00')
    expect(wrapper.get('[data-testid="accounting-overview"]').text()).not.toContain('25.00')
  })

  it('opens records in a read-only view with public fields only', async () => {
    const wrapper = await render({ tab: 'transfers' })
    expect(api.listRecords).toHaveBeenCalledWith('transfers', true, expect.objectContaining({ page: 1 }), expect.anything())
    expect(wrapper.text()).not.toContain('导出')
    // Organizations only receive received transfers, so a status filter would offer nothing.
    expect(wrapper.find('[data-testid="record-list"] select').exists()).toBe(false)

    const button = wrapper.get('[data-testid="row"] button')
    expect(button.text()).toBe('客户付款')
    await button.trigger('click'); await flushPromises()
    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).not.toContain('内部备注')
    expect(dialog.text()).not.toContain('BANK-1')
    expect(api.attachments).toHaveBeenCalledWith('transfers', 'record-one', true)
    expect(router.push).not.toHaveBeenCalled()
  })

  it('switches sections through the address', async () => {
    const wrapper = await render()
    await wrapper.findAll('[role="tab"]')[2]!.trigger('click')
    expect(router.replace).toHaveBeenCalledWith({ query: { tab: 'vcc-cards' } })
  })

  it('applies list filters as soon as they change', async () => {
    const wrapper = await render({ tab: 'vcc-cards' })
    api.listRecords.mockClear()
    await wrapper.get('[data-testid="record-list"] select').setValue('ACTIVE')
    await flushPromises()
    expect(api.listRecords).toHaveBeenCalledWith('vcc-cards', true, expect.objectContaining({ status: 'ACTIVE' }), expect.anything())
  })

  it('offers a retry after a list failure without presenting it as empty data', async () => {
    api.listRecords.mockRejectedValueOnce({ code: 'ERR_NETWORK' })
    const wrapper = await render({ tab: 'transfers' })
    expect(wrapper.get('[role="alert"]').text()).toContain('检查网络后重试')
    expect(wrapper.find('[data-testid="row"]').exists()).toBe(false)
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="row"]').exists()).toBe(true)
  })
})
