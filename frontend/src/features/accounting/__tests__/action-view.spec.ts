import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, record, stubs } from './testing'
import type { AccountingKind, AccountingRecord } from '../types'

const api = vi.hoisted(() => ({
  organizations: vi.fn(),
  getRecord: vi.fn(),
  listRecords: vi.fn(),
  attachments: vi.fn(),
  workflow: vi.fn(),
  order: vi.fn(),
  upload: vi.fn(),
  mutate: vi.fn(),
  stableRequest: () => Object.assign(() => 'stable-key', { reset: vi.fn() }),
}))
const route = vi.hoisted(() => ({ params: {} as Record<string, string>, query: {} as Record<string, string> }))
const router = vi.hoisted(() => ({ push: vi.fn() }))
const auth = vi.hoisted(() => ({ isAdmin: true }))
vi.mock('../api', () => api)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router, onBeforeRouteLeave: vi.fn() }))

import AccountingActionView from '../AccountingActionView.vue'

enableAutoUnmount(afterEach)

const cards: AccountingRecord[] = [
  record({ id: 'card-cny', status: 'ACTIVE', details: { alias: '人民币卡', available_balance: '0' } }),
  record({ id: 'card-usd', status: 'ACTIVE', currency: 'USD', details: { alias: '美元卡' } }),
  record({ id: 'card-closed', status: 'CLOSED', details: { alias: '旧卡' } }),
]

async function render(params: Record<string, string>, source?: AccountingRecord) {
  route.params = { organizationId: 'org-one', ...params }
  if (source) api.getRecord.mockResolvedValue(source)
  const wrapper = mount(AccountingActionView, { global: { plugins: [i18n()], stubs } })
  await flushPromises()
  return wrapper
}
async function submit(wrapper: Awaited<ReturnType<typeof render>>) {
  await wrapper.get('form').trigger('submit')
  await flushPromises()
}

describe('accounting action page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.query = {}
    auth.isAdmin = true
    api.organizations.mockResolvedValue([organization])
    api.listRecords.mockImplementation(async (kind: AccountingKind) => ({ items: kind === 'vcc-cards' ? cards : [], total: 0 }))
    api.attachments.mockResolvedValue([])
    api.mutate.mockResolvedValue({})
  })

  it('allocates a receipt to an active card in its currency, defaulting to the remaining amount', async () => {
    const wrapper = await render({ kind: 'transfers', id: 'receipt-one', action: 'allocate' }, record({ id: 'receipt-one', status: 'RECEIVED', details: { unallocated_amount: '40' } }))

    const options = wrapper.findAll('select option').map((option) => option.attributes('value'))
    expect(options).toEqual(['', 'card-cny'])
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('card-cny')
    expect((wrapper.get('[data-testid="action-amount"]').element as HTMLInputElement).value).toBe('40')

    await submit(wrapper)
    expect(api.mutate).toHaveBeenCalledWith('transfers/receipt-one/allocations', { card_id: 'card-cny', amount: '40' }, 'stable-key', 'post')
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminAccountingRecord', params: { organizationId: 'org-one', kind: 'transfers', id: 'receipt-one' } })
  })

  it('uploads payment proof inline and records the payment', async () => {
    const proof = new File(['proof'], 'proof.pdf', { type: 'application/pdf' })
    api.upload.mockResolvedValue({ id: 'file-one', filename: 'proof.pdf', status: 'COMPLETE' })
    const wrapper = await render({ kind: 'expenses', id: 'expense-one', action: 'pay' }, record({ id: 'expense-one', status: 'UNPAID', details: { payment_account: '招商银行 1234', unpaid_amount: '80' } }))

    expect((wrapper.get('[data-testid="action-amount"]').element as HTMLInputElement).value).toBe('80')
    expect(wrapper.find('textarea[required]').exists()).toBe(false)
    const fileInput = wrapper.get('[data-testid="action-proof"] input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', { value: [proof] })
    await fileInput.trigger('change')
    expect(wrapper.get('[data-testid="action-proof"]').text()).toContain(proof.name)
    const inputs = wrapper.findAll('input:not([type])')
    await inputs[1]!.setValue('PAY-1')
    await submit(wrapper)

    expect(api.upload).toHaveBeenCalledWith(proof, 'expenses', { id: 'expense-one', organization_id: 'org-one' })
    expect(api.mutate).toHaveBeenCalledWith(
      'expenses/expense-one/payments',
      expect.objectContaining({
        amount: '80',
        details: expect.objectContaining({ payment_account: '招商银行 1234', transaction_number: 'PAY-1', proof_attachment_id: 'file-one', override: false }),
      }),
      'stable-key',
      'post',
    )
  })

  it('treats a payment above the unpaid amount as an overpayment that needs a reason', async () => {
    const wrapper = await render({ kind: 'expenses', id: 'expense-one', action: 'pay' }, record({ id: 'expense-one', status: 'PENDING', details: { unpaid_amount: '80' } }))
    await wrapper.get('[data-testid="action-amount"]').setValue('100')
    expect(wrapper.text()).toContain('将作为超额付款登记')
    expect(wrapper.find('textarea[required]').exists()).toBe(true)
  })

  it('requires a reason to void and posts to the record lifecycle endpoint', async () => {
    const wrapper = await render({ kind: 'contracts', id: 'contract-one', action: 'void' }, record({ id: 'contract-one', status: 'SIGNED', details: { name: '年度合同' } }))
    const reason = wrapper.get('textarea[required]')
    await reason.setValue('录入错误')
    await submit(wrapper)
    expect(api.mutate).toHaveBeenCalledWith('contracts/contract-one/void', { reason: '录入错误' }, 'stable-key', 'post')
  })

  it('closes the previous month by default and asks for a reason only for partial coverage', async () => {
    const wrapper = await render({ action: 'close' })
    const month = wrapper.get('input[type="month"]').element as HTMLInputElement
    expect(month.value).toBe(month.max)
    expect(wrapper.find('textarea[required]').exists()).toBe(false)
    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.find('textarea[required]').exists()).toBe(true)
    await wrapper.get('textarea').setValue('首期启用')
    await submit(wrapper)
    expect(api.mutate).toHaveBeenCalledWith('periods/close', { organization_id: 'org-one', month: month.max, accept_partial: true, reason: '首期启用' }, 'stable-key', 'post')
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminOrganizationAccounting', params: { organizationId: 'org-one' }, query: { tab: 'periods' } })
  })

  it('tops a card up with our own funds, defaulting to its shortfall', async () => {
    const wrapper = await render({ kind: 'vcc-cards', id: 'card-usd', action: 'top-up' }, record({ id: 'card-usd', status: 'ACTIVE', currency: 'USD', details: { alias: '美元卡', available_balance: '-50' } }))
    expect((wrapper.get('[data-testid="action-amount"]').element as HTMLInputElement).value).toBe('50')
    await submit(wrapper)
    expect(api.mutate).toHaveBeenCalledWith('vcc-cards/card-usd/top-ups', expect.objectContaining({ amount: '50', reference: '', remark: '' }), 'stable-key', 'post')
  })

  it('links only records that are outside every order', async () => {
    route.query = { kind: 'invoices' }
    api.order.mockResolvedValue({ id: 'order-one', organization_id: 'org-one', name: 'Q4', remark: '', state: 'OPEN', version: 2 })
    api.listRecords.mockResolvedValue({
      total: 3,
      items: [
        record({ id: 'free', status: 'VERIFIED', details: { invoice_number: 'INV-FREE', invoice_type: 'SALES' } }),
        record({ id: 'taken', status: 'VERIFIED', details: { invoice_number: 'INV-TAKEN', invoice_type: 'SALES', order_id: 'other' } }),
        record({ id: 'purchase', status: 'VERIFIED', details: { invoice_number: 'INV-IN', invoice_type: 'PURCHASE' } }),
      ],
    })
    const wrapper = await render({ action: 'order-link', targetId: 'order-one' })
    expect(wrapper.findAll('select option').map((option) => option.attributes('value'))).toEqual(['', 'free'])
    await wrapper.get('select').setValue('free')
    await submit(wrapper)
    expect(api.mutate).toHaveBeenCalledWith('orders/order-one/links', { object_kind: 'invoices', object_id: 'free' }, 'stable-key', 'post')
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminAccountingOrder', params: { organizationId: 'org-one', orderId: 'order-one' } })
  })

  it('renames an order with its current version', async () => {
    api.order.mockResolvedValue({ id: 'order-one', organization_id: 'org-one', name: 'Q4', remark: '', state: 'OPEN', version: 2 })
    const wrapper = await render({ action: 'order-edit', targetId: 'order-one' })
    await wrapper.get('input[required]').setValue('Q4 席位')
    await submit(wrapper)
    expect(api.mutate).toHaveBeenCalledWith('orders/order-one', { name: 'Q4 席位', remark: '', version: 2 }, 'stable-key', 'patch')
  })
})
