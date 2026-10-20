import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, stubs } from './testing'

const api = vi.hoisted(() => ({ orders: vi.fn(), organizations: vi.fn() }))
const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))
const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }))
vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router }))

import OrdersHomeView from '../OrdersHomeView.vue'

enableAutoUnmount(afterEach)

const order = {
  id: 'order-one',
  organization_id: 'org-one',
  organization_name: '上海星海智能科技有限公司',
  name: 'Claude 席位',
  status: 'OPEN',
  state: 'OPEN',
  created_at: '2026-10-12T02:00:00Z',
  progress: { receipt: true, invoice: false, purchase: false, ai_invoice: false },
  totals: [{ currency: 'CNY', received: '1000', invoiced: '0', spent: '0', ai_cost: '0' }],
}

async function render(query: Record<string, string> = {}) {
  route.query = query
  const wrapper = mount(OrdersHomeView, { global: { plugins: [i18n()], stubs: { ...stubs, Pagination: true } } })
  await flushPromises()
  return wrapper
}

describe('orders home', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.orders.mockResolvedValue({ items: [order], total: 1 })
    api.organizations.mockResolvedValue([organization])
  })

  it('lists every organization’s orders with their progress', async () => {
    const wrapper = await render()
    expect(api.orders).toHaveBeenCalledWith(expect.objectContaining({ organization_id: undefined, page: 1 }), expect.anything())
    const row = wrapper.get('tbody tr')
    expect(row.text()).toContain('Claude 席位')
    expect(row.text()).toContain('上海星海智能科技有限公司')
    expect(row.findAll('[data-done="true"]').map((step) => step.text())).toEqual(['收款'])
    await row.trigger('click')
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminAccountingOrder', params: { orderId: 'order-one' } })
  })

  it('filters by organization and state from the address and keeps them there', async () => {
    const wrapper = await render({ organization: 'org-one', state: 'OPEN' })
    expect(api.orders).toHaveBeenCalledWith(expect.objectContaining({ organization_id: 'org-one', state: 'OPEN' }), expect.anything())
    expect(wrapper.get('[data-testid="orders-new"]').findComponent({ name: 'RouterLink' }).props('to')).toEqual({ name: 'AdminAccountingOrderNew', query: { organization: 'org-one' } })

    await wrapper.get('[data-testid="orders-organization"]').setValue('')
    await flushPromises()
    expect(router.replace).toHaveBeenLastCalledWith({ query: { state: 'OPEN' } })
    expect(api.orders).toHaveBeenLastCalledWith(expect.objectContaining({ organization_id: undefined, state: 'OPEN' }), expect.anything())
  })

  it('explains how to start when there are no orders', async () => {
    api.orders.mockResolvedValue({ items: [], total: 0 })
    const wrapper = await render()
    expect(wrapper.text()).toContain('还没有订单')
  })
})
