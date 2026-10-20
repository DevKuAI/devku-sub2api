import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, record, stubs } from './testing'
import type { AccountingRecord } from '../types'

const api = vi.hoisted(() => ({
  organizations: vi.fn(),
  getRecord: vi.fn(),
  workflow: vi.fn(),
  allocationHistory: vi.fn(),
  attachments: vi.fn(),
  mutate: vi.fn(),
  upload: vi.fn(),
  download: vi.fn(),
  stableRequest: () => Object.assign(() => 'stable-key', { reset: vi.fn() }),
}))
const route = vi.hoisted(() => ({
  name: 'AdminAccountingRecord',
  fullPath: '/admin/accounting/organizations/org-one/transfers/record-one',
  params: { organizationId: 'org-one', kind: 'transfers', id: 'record-one' } as Record<string, string>,
  query: {} as Record<string, string>,
}))
const router = vi.hoisted(() => ({ push: vi.fn() }))
const auth = vi.hoisted(() => ({ isAdmin: true }))
vi.mock('../api', () => api)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router }))

import AccountingRecordView from '../AccountingRecordView.vue'

enableAutoUnmount(afterEach)

const ConfirmDialog = {
  name: 'ConfirmDialog',
  props: ['show', 'title', 'message'],
  emits: ['confirm', 'cancel'],
  template: '<div v-if="show" data-testid="confirm"><p>{{ message }}</p><button data-testid="confirm-ok" @click="$emit(\'confirm\')" /></div>',
}

async function render(current: AccountingRecord, kind = 'transfers') {
  route.params = { organizationId: 'org-one', kind, id: current.id }
  api.getRecord.mockResolvedValue(current)
  const wrapper = mount(AccountingRecordView, { global: { plugins: [i18n()], stubs: { ...stubs, ConfirmDialog } } })
  await flushPromises()
  return wrapper
}
const actions = (wrapper: Awaited<ReturnType<typeof render>>) => wrapper.findAll('[data-action]').map((button) => button.attributes('data-action'))

describe('accounting record page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    auth.isAdmin = true
    api.organizations.mockResolvedValue([organization])
    api.workflow.mockResolvedValue([])
    api.allocationHistory.mockResolvedValue([])
    api.attachments.mockResolvedValue([])
    api.mutate.mockResolvedValue({})
  })

  it('shows readable details instead of raw stored values', async () => {
    const wrapper = await render(
      record({
        details: {
          payer_name: '上海星海智能科技有限公司',
          bank_reference: 'BANK-001',
          sales_amount: '106',
          tax_inclusive: true,
          tax_rate: '0.06',
          tax_amount: '6',
          receiving_account_id: 'account-one',
          receiving_account: '6225888800001234',
          receiving_account_name: '我方公司',
          card_number: 'secret',
          adjustment_amount: '0',
          unallocated_amount: '100',
        },
      }),
    )
    const text = wrapper.text()
    expect(text).toContain('6% · 含税')
    expect(text).toContain('我方公司 · 6225888800001234')
    expect(text).not.toContain('account-one')
    expect(text).not.toContain('secret')
    expect(text).not.toContain('累计调整')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('confirms a draft in place with a confirmation dialog', async () => {
    const wrapper = await render(record())
    expect(actions(wrapper)).toEqual(['confirm', 'edit', 'void'])

    await wrapper.get('[data-action="confirm"]').trigger('click')
    expect(wrapper.get('[data-testid="confirm"]').text()).toContain('确认后记录正式入账')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()

    expect(api.mutate).toHaveBeenCalledWith('transfers/record-one/confirm', {}, 'stable-key', 'post')
    expect(api.getRecord).toHaveBeenCalledTimes(2)
    expect(router.push).not.toHaveBeenCalled()
  })

  it('offers the next step for a confirmed receipt on its own page', async () => {
    const wrapper = await render(record({ status: 'RECEIVED', details: { unallocated_amount: '40' } }))
    expect(actions(wrapper)).toEqual(['allocate', 'exchange', 'adjust', 'void'])

    await wrapper.get('[data-action="allocate"]').trigger('click')
    expect(router.push).toHaveBeenCalledWith({
      name: 'AdminAccountingRecordAction',
      params: { organizationId: 'org-one', kind: 'transfers', id: 'record-one', action: 'allocate' },
    })
    await wrapper.get('[data-action="exchange"]').trigger('click')
    expect(router.push).toHaveBeenLastCalledWith({
      name: 'AdminAccountingRecordNew',
      params: { organizationId: 'org-one', kind: 'fx-exchanges' },
      query: { parent_id: 'record-one', currency: 'CNY', return_to: route.fullPath },
    })
  })

  it('hides allocation once a receipt is fully allocated', async () => {
    const wrapper = await render(record({ status: 'RECEIVED', details: { unallocated_amount: '0' } }))
    expect(actions(wrapper)).not.toContain('allocate')
  })

  it('only lets administrators overpay a fully paid expense', async () => {
    const paid = record({ status: 'PAID', details: { unpaid_amount: '0' } })
    expect(actions(await render(paid, 'expenses'))).toContain('pay')
    auth.isAdmin = false
    expect(actions(await render(paid, 'expenses'))).not.toContain('pay')
  })

  it('refuses a record from another organization', async () => {
    const wrapper = await render(record({ organization_id: 'org-two' }))
    expect(wrapper.text()).toContain('企业不存在或已不可用')
    expect(wrapper.find('[data-testid="record-actions"]').exists()).toBe(false)
  })

  it('tops cards up from the card and links records to their order', async () => {
    const card = record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', details: { alias: '席位卡', order_id: 'order-one', order_name: 'Q4 席位' } })
    const wrapper = await render(card, 'vcc-cards')
    expect(actions(wrapper)).toEqual(['top-up', 'edit', 'close'])
    expect(wrapper.get('[data-testid="record-order"]').text()).toBe('Q4 席位')
  })

  it('goes back to the order a record belongs to', async () => {
    const wrapper = await render(record({ details: { bank_reference: 'BANK-7', order_id: 'order-one', order_name: 'Q4 席位' } }))
    expect(wrapper.get('h1').text()).toBe('BANK-7')
    const back = wrapper.findAllComponents({ name: 'RouterLink' }).find((link) => link.attributes('data-testid') === 'page-back')!
    expect(back.text()).toBe('Q4 席位')
    expect(back.props('to')).toEqual({ name: 'AdminAccountingOrder', params: { organizationId: 'org-one', orderId: 'order-one' } })
    expect(wrapper.text().match(/Q4 席位/g)!.length).toBe(2)
  })
})
