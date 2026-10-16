import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, record, stubs } from './testing'
import type { AccountingOrder } from '../types'

const api = vi.hoisted(() => ({
  organizations: vi.fn(),
  order: vi.fn(),
  mutate: vi.fn(),
  stableRequest: () => Object.assign(() => 'stable-key', { reset: vi.fn() }),
}))
const route = vi.hoisted(() => ({
  path: '/admin/accounting/organizations/org-one/orders/order-one',
  params: { organizationId: 'org-one', orderId: 'order-one' },
  query: {} as Record<string, string>,
}))
vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push: vi.fn() }) }))

import OrderView from '../OrderView.vue'

enableAutoUnmount(afterEach)

function order(overrides: Partial<AccountingOrder> = {}): AccountingOrder {
  return {
    id: 'order-one',
    organization_id: 'org-one',
    name: 'Q4 ChatGPT 席位',
    remark: '',
    status: 'OPEN',
    state: 'OPEN',
    close_reason: '',
    created_at: '2026-10-12T02:00:00Z',
    version: 1,
    progress: { receipt: true, invoice: false, purchase: true, ai_invoice: false },
    totals: [
      { currency: 'CNY', received: '1000', invoiced: '0', spent: '0', ai_cost: '0' },
      { currency: 'USD', received: '0', invoiced: '0', spent: '50', ai_cost: '0' },
    ],
    items: {
      transfers: [record({ id: 'receipt-one', status: 'RECEIVED', amount: '1000', details: { payer_name: '上海星海', bank_reference: 'BANK-1' } })],
      invoices: [],
      'vcc-cards': [record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', amount: '0', details: { alias: '席位卡', last_four: '4242', available_balance: '-50' } })],
      'vcc-transactions': [record({ id: 'spend-one', parent_id: 'card-one', status: 'CONFIRMED', currency: 'USD', amount: '50', details: { merchant: 'OpenAI', transaction_type: 'CONSUME' } })],
      'vendor-invoices': [],
    },
    ...overrides,
  }
}
async function render(current: AccountingOrder) {
  api.order.mockResolvedValue(current)
  const wrapper = mount(OrderView, { global: { plugins: [i18n()], stubs: { ...stubs, ConfirmDialog: true } } })
  await flushPromises()
  return wrapper
}
const links = (wrapper: Awaited<ReturnType<typeof render>>) =>
  wrapper.findAllComponents({ name: 'RouterLink' }).map((link) => ({ text: link.text(), to: link.props('to') }))

describe('accounting order page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.organizations.mockResolvedValue([organization])
  })

  it('shows the order as one deal with progress, totals and every record', async () => {
    const wrapper = await render(order())

    expect(wrapper.get('h1').text()).toBe('Q4 ChatGPT 席位')
    const progress = wrapper.findAll('[data-step]').map((step) => [step.attributes('data-step'), step.attributes('data-done')])
    expect(progress).toEqual([
      ['receipt', 'true'],
      ['invoice', 'false'],
      ['purchase', 'true'],
      ['ai_invoice', 'false'],
    ])
    expect(wrapper.get('[data-testid="order-totals"]').text()).toContain('¥1,000.00')
    expect(wrapper.get('[data-testid="order-section-receipt"]').text()).toContain('BANK-1')
    expect(wrapper.get('[data-testid="order-section-purchase"]').text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('请为该卡登记一笔卡充值')
  })

  it('adds every record type from the order and returns here', async () => {
    const wrapper = await render(order())
    const add = links(wrapper).find((link) => link.text.includes('登记卡消费'))!
    expect(add.to).toEqual({
      name: 'AdminAccountingRecordNew',
      params: { organizationId: 'org-one', kind: 'vcc-transactions' },
      query: { order_id: 'order-one', return_to: route.path },
    })
    expect(links(wrapper).find((link) => link.text.includes('开具发票'))!.to).toMatchObject({ params: { kind: 'invoices' } })
    expect(links(wrapper).find((link) => link.text === '卡充值')!.to).toEqual({
      name: 'AdminAccountingRecordAction',
      params: { organizationId: 'org-one', kind: 'vcc-cards', id: 'card-one', action: 'top-up' },
      query: { return_to: route.path },
    })
  })

  it('offers no new records on a closed order, only reopening it', async () => {
    const wrapper = await render(order({ status: 'CLOSED', state: 'CLOSED', close_reason: '客户不需要发票' }))
    expect(wrapper.text()).toContain('已手动结束：客户不需要发票')
    expect(links(wrapper).some((link) => link.text.includes('登记'))).toBe(false)
    expect(wrapper.text()).toContain('重新打开订单')
  })
})
