import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, record, stubs } from './testing'

const api = vi.hoisted(() => ({
  listRecords: vi.fn(),
  organizations: vi.fn(),
  mutate: vi.fn(),
  channelAccounts: vi.fn(),
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
const card = record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', details: { alias: '席位卡', last_four: '4444', provider: 'Airwallex', spent_amount: '120', order_id: 'order-one', order_name: 'Claude 席位' } })

const wallet = (name: string, currency: string, balance: string) => ({ wallet: name, currency, opening: '0', balance, lines: 1, first_at: '', last_at: '', gaps: [], negative_spans: [] })
const channel = {
  account: { id: 'channel-one', platform: 'SUNRATE', account_ref: '72778936', name: 'SUNRATE 主账户' },
  wallets: [wallet('PAYMENT', 'USD', '2172.87'), wallet('CARD', 'USD', '1026.50')],
  positions: [],
  fee_costs: [],
  unpaired_transfers: 0,
  missing_proofs: 0,
  cards: 4,
}

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
    api.channelAccounts.mockResolvedValue([channel])
  })

  it('lists active cards with their organization, order and spend', async () => {
    const wrapper = await render()
    expect(api.listRecords).toHaveBeenCalledWith('vcc-cards', false, expect.objectContaining({ status: 'ACTIVE', page: 1 }))
    const row = wrapper.get('tbody tr')
    expect(row.text()).toContain('席位卡 •••• 4444')
    expect(row.text()).toContain('上海星海智能科技有限公司')
    expect(row.text()).toContain('Claude 席位')
    expect(row.text()).toMatch(/(US)?\$120\.00/)
    expect(row.text()).not.toMatch(/-(US)?\$/)
    expect(wrapper.get('thead').text()).toContain('累计消费')
    expect(row.text()).not.toContain('充值')
  })

  it('points to the channel ledger card balance that holds the money', async () => {
    const wrapper = await render()
    const pools = wrapper.get('[data-testid="card-pools"]')
    expect(pools.text()).toContain('卡余额（渠道账）')
    expect(pools.text()).toContain('SUNRATE 主账户')
    expect(pools.text()).toMatch(/(US)?\$1,026\.50/)
    expect(pools.text()).not.toContain('2,172.87')
  })

  it('keeps the card list when the channel ledger cannot be read', async () => {
    api.channelAccounts.mockRejectedValue(new Error('forbidden'))
    const wrapper = await render()
    expect(wrapper.find('[data-testid="card-pools"]').exists()).toBe(false)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('tbody tr').text()).toContain('席位卡 •••• 4444')
  })

  it('closes a card only after confirmation', async () => {
    const wrapper = await render()
    await wrapper.findAll('tbody button').find((button) => button.text() === '停用')!.trigger('click')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('vcc-cards/card-one/close', {}, 'key-vcc-cards/card-one/close')
  })
})
