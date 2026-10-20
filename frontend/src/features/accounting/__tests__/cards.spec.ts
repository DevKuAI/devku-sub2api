import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, record, stubs } from './testing'

const api = vi.hoisted(() => ({
  listRecords: vi.fn(),
  organizations: vi.fn(),
  mutate: vi.fn(),
  stableRequest: () => Object.assign((path: string) => `key-${path}`, { reset: vi.fn() }),
}))
vi.mock('../api', () => api)

import CardsView from '../CardsView.vue'

enableAutoUnmount(afterEach)

const ConfirmDialog = {
  props: ['show'],
  emits: ['confirm', 'cancel'],
  template: '<div v-if="show"><button data-testid="confirm-ok" @click="$emit(\'confirm\')" /></div>',
}
const card = record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', details: { alias: '席位卡', last_four: '4444', provider: 'Airwallex', available_balance: '-120', order_id: 'order-one', order_name: 'Claude 席位' } })

async function render() {
  const wrapper = mount(CardsView, { global: { plugins: [i18n()], stubs: { ...stubs, ConfirmDialog, Pagination: true } } })
  await flushPromises()
  return wrapper
}

describe('cards page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listRecords.mockResolvedValue({ items: [card], total: 1 })
    api.organizations.mockResolvedValue([organization])
    api.mutate.mockResolvedValue({})
  })

  it('lists active cards with their organization, order and balance', async () => {
    const wrapper = await render()
    expect(api.listRecords).toHaveBeenCalledWith('vcc-cards', false, expect.objectContaining({ status: 'ACTIVE', page: 1 }))
    const row = wrapper.get('tbody tr')
    expect(row.text()).toContain('席位卡 •••• 4444')
    expect(row.text()).toContain('上海星海智能科技有限公司')
    expect(row.text()).toContain('Claude 席位')
    expect(row.text()).toMatch(/-(US)?\$120\.00/)
  })

  it('tops a card up in place, defaulting to what it is short', async () => {
    const wrapper = await render()
    await wrapper.get('[data-testid="card-top-up-open"]').trigger('click')
    const form = wrapper.get('[data-testid="card-top-up"]')
    expect((form.get('input').element as HTMLInputElement).value).toBe('120')
    await form.trigger('submit'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('vcc-cards/card-one/top-ups', expect.objectContaining({ amount: '120', remark: '' }), 'key-vcc-cards/card-one/top-ups')
    expect(api.listRecords).toHaveBeenCalledTimes(2)
  })

  it('closes a card only after confirmation', async () => {
    const wrapper = await render()
    await wrapper.findAll('tbody button').find((button) => button.text() === '停用')!.trigger('click')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('vcc-cards/card-one/close', {}, 'key-vcc-cards/card-one/close')
  })
})
