import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { i18n, receivingAccount, record, stubs, vendors } from './testing'
import type { AccountingOrder, AccountingRecord, OrderItemKind } from '../types'

const api = vi.hoisted(() => ({
  order: vi.fn(),
  receivingAccounts: vi.fn(),
  vendors: vi.fn(),
  addEntry: vi.fn(),
  updateEntry: vi.fn(),
  mutate: vi.fn(),
  listRecords: vi.fn(),
  upload: vi.fn(),
  attachments: vi.fn(),
  stableRequest: () => Object.assign((path: string) => `key-${path}`, { reset: vi.fn() }),
}))
const route = vi.hoisted(() => ({ params: { orderId: 'order-one' }, fullPath: '/admin/accounting/orders/order-one' }))
vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRoute: () => route, onBeforeRouteLeave: vi.fn() }))

import OrderView from '../OrderView.vue'

enableAutoUnmount(afterEach)

const ConfirmDialog = {
  props: ['show', 'title'],
  emits: ['confirm', 'cancel'],
  template: '<div v-if="show" data-testid="confirm-dialog">{{ title }}<button data-testid="confirm-ok" @click="$emit(\'confirm\')" /></div>',
}

function order(overrides: Partial<AccountingOrder> = {}, items: Partial<Record<OrderItemKind, AccountingRecord[]>> = {}): AccountingOrder {
  return {
    id: 'order-one',
    organization_id: 'org-one',
    organization_name: '上海星海智能科技有限公司',
    name: 'Claude 席位',
    remark: '',
    status: 'OPEN',
    state: 'OPEN',
    close_reason: '',
    created_at: '2026-10-12T02:00:00Z',
    version: 2,
    progress: { receipt: false, invoice: false, purchase: false, ai_invoice: false },
    totals: [],
    items: { transfers: [], invoices: [], 'vcc-cards': [], 'vcc-transactions': [], 'vendor-invoices': [], ...items },
    ...overrides,
  }
}
const receipt = record({ id: 'receipt-one', status: 'RECEIVED', amount: '1000', version: 3, details: { bank_reference: 'B-1', payer_name: '客户', tax_rate: '0.06', sales_amount: '1000', attachment_count: '0' } })

async function render(current: AccountingOrder) {
  api.order.mockResolvedValue(current)
  const wrapper = mount(OrderView, { global: { plugins: [i18n()], stubs: { ...stubs, ConfirmDialog } } })
  await flushPromises()
  return wrapper
}
async function fill(wrapper: VueWrapper, kind: string, values: Record<string, string>) {
  const form = wrapper.get(`[data-testid="entry-form-${kind}"]`)
  for (const [key, value] of Object.entries(values)) await form.get(`[data-field="${key}"] input`).setValue(value)
  await form.trigger('submit')
  await flushPromises()
}

describe('order page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.receivingAccounts.mockResolvedValue([receivingAccount])
    api.vendors.mockResolvedValue(vendors)
    api.addEntry.mockResolvedValue(record({ id: 'new-one' }))
    api.updateEntry.mockResolvedValue(receipt)
    api.mutate.mockResolvedValue({})
    api.attachments.mockResolvedValue([])
  })

  it('starts with the receipt and records it on the page', async () => {
    const wrapper = await render(order())
    expect(wrapper.get('[data-testid="order-next-step"]').text()).toBe('登记这笔订单的收款')
    expect(wrapper.text()).toContain('上海星海智能科技有限公司')

    await wrapper.get('[data-testid="order-next-action"]').trigger('click')
    const form = wrapper.get('[data-testid="entry-form-transfers"]')
    expect((form.get('[data-field="payer_name"] input').element as HTMLInputElement).value).toBe('上海星海智能科技有限公司')
    expect((form.get('[data-field="receiving_account_id"] select').element as HTMLSelectElement).value).toBe('account-one')

    await fill(wrapper, 'transfers', { amount: '1000', bank_reference: 'B-9' })
    expect(api.addEntry).toHaveBeenCalledWith(
      'order-one',
      'transfers',
      expect.objectContaining({ currency: 'CNY', amount: '1000', details: expect.objectContaining({ bank_reference: 'B-9', sales_amount: '1000', tax_rate: '0.06', receiving_account_id: 'account-one' }) }),
      'key-transfers',
    )
    expect(api.order).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="entry-form-transfers"]').exists()).toBe(false)
  })

  it('edits an effective entry in place with its version and flags missing proof', async () => {
    const wrapper = await render(order({ progress: { receipt: true, invoice: false, purchase: false, ai_invoice: false } }, { transfers: [receipt] }))
    expect(wrapper.find('[data-testid="entry-missing-file"]').exists()).toBe(true)
    await wrapper.get('[data-testid="entry-edit"]').trigger('click')
    expect(wrapper.find('[data-field="receiving_account_id"]').exists()).toBe(false)
    await fill(wrapper, 'transfers', { amount: '1200' })
    expect(api.updateEntry).toHaveBeenCalledWith('order-one', 'transfers', 'receipt-one', expect.objectContaining({ amount: '1200', version: 3 }), 'key-transfers/receipt-one')
  })

  it('removes an entry only after confirmation', async () => {
    const wrapper = await render(order({}, { transfers: [receipt] }))
    await wrapper.get('[data-testid="entry-remove"]').trigger('click')
    expect(api.mutate).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('orders/order-one/entries/transfers/receipt-one', {}, 'key-delete:orders/order-one/entries/transfers/receipt-one', 'delete')
  })

  it('makes entries left as drafts effective in one click', async () => {
    const draft = record({ id: 'draft-one', status: 'DRAFT', details: { bank_reference: 'B-2' } })
    const wrapper = await render(order({}, { transfers: [draft] }))
    expect(wrapper.get('[data-testid="order-next-step"]').text()).toBe('有一笔收款还没生效')
    await wrapper.get('[data-testid="entry-confirm"]').trigger('click'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('transfers/draft-one/confirm', {}, 'key-post:transfers/draft-one/confirm', 'post')
  })

  it('asks for a card before card spend and picks the only active card', async () => {
    const done = { receipt: true, invoice: true, purchase: false, ai_invoice: false }
    const empty = await render(order({ progress: done }))
    expect(empty.get('[data-testid="order-next-step"]').text()).toBe('添加用于采购的 VCC 卡')
    expect(empty.get('[data-testid="add-vcc-transactions"]').attributes('disabled')).toBeDefined()
    empty.unmount()

    const card = record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', details: { alias: '席位卡', last_four: '4444', provider: 'Airwallex', available_balance: '-20' } })
    const wrapper = await render(order({ progress: done }, { 'vcc-cards': [card] }))
    expect(wrapper.text()).toContain('卡余额为负')
    await wrapper.get('[data-testid="add-vcc-transactions"]').trigger('click')
    expect((wrapper.get('[data-field="parent_id"] select').element as HTMLSelectElement).value).toBe('card-one')
    expect(wrapper.get('[data-field="amount"]').text()).toContain('USD')
  })

  it('shows only the totals an order has and no close action once it is complete', async () => {
    const complete = order({ state: 'COMPLETED', progress: { receipt: true, invoice: true, purchase: true, ai_invoice: true }, totals: [{ currency: 'CNY', received: '1000', invoiced: '1000', spent: '0', ai_cost: '0' }] })
    const wrapper = await render(complete)
    expect(wrapper.get('[data-testid="order-totals"]').text()).toContain('收入')
    expect(wrapper.get('[data-testid="order-totals"]').text()).not.toContain('卡消费')
    expect(wrapper.find('[data-testid="order-close"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="order-next-step"]').text()).toBe('四步都已齐全，订单已完成。')
  })

  it('closes an order with a reason on the page and reopens it after confirmation', async () => {
    const wrapper = await render(order())
    await wrapper.get('[data-testid="order-close"]').trigger('click')
    await wrapper.get('[data-testid="order-close-form"] textarea').setValue('客户取消')
    await wrapper.get('[data-testid="order-close-form"]').trigger('submit'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('orders/order-one/close', { reason: '客户取消' }, 'key-post:orders/order-one/close', 'post')

    wrapper.unmount()
    const closed = await render(order({ status: 'CLOSED', state: 'CLOSED', close_reason: '客户取消' }, { transfers: [receipt] }))
    expect(closed.text()).toContain('客户取消')
    expect(closed.find('[data-testid="entry-edit"]').exists()).toBe(false)
    await closed.get('[data-testid="order-reopen"]').trigger('click')
    await closed.get('[data-testid="confirm-ok"]').trigger('click'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('orders/order-one/reopen', {}, 'key-post:orders/order-one/reopen', 'post')
  })

  it('keeps a saved entry when its file fails to upload and says so', async () => {
    api.upload.mockRejectedValueOnce(new Error('network'))
    const wrapper = await render(order())
    await wrapper.get('[data-testid="add-transfers"]').trigger('click')
    wrapper.findComponent({ name: 'AccountingFileUpload' }).vm.$emit('update:modelValue', new File(['x'], 'slip.pdf', { type: 'application/pdf' }))
    await fill(wrapper, 'transfers', { amount: '1000', bank_reference: 'B-9' })
    expect(api.addEntry).toHaveBeenCalled()
    expect(api.upload).toHaveBeenCalled()
    expect(wrapper.get('[role="status"]').text()).toContain('附件上传失败')
  })
})
