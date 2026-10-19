import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, receivingAccount, record, stubs } from './testing'
import type { AccountingKind } from '../types'

const api = vi.hoisted(() => ({
  organizations: vi.fn(),
  receivingAccounts: vi.fn(),
  organizationContacts: vi.fn(),
  listRecords: vi.fn(),
  getRecord: vi.fn(),
  order: vi.fn(),
  mutate: vi.fn(),
  upload: vi.fn(),
  stableRequest: () => Object.assign(() => 'stable-key', { reset: vi.fn() }),
}))
const route = vi.hoisted(() => ({
  name: 'AdminAccountingRecordNew',
  params: {} as Record<string, string>,
  query: {} as Record<string, string>,
}))
const router = vi.hoisted(() => ({ push: vi.fn() }))
vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router, onBeforeRouteLeave: vi.fn() }))

import AccountingRecordFormView from '../AccountingRecordFormView.vue'

enableAutoUnmount(afterEach)

const card = record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', amount: '0', details: { alias: '研发卡', available_balance: '80' } })
const purchase = record({ id: 'tx-one', parent_id: 'card-one', status: 'CONFIRMED', currency: 'USD', amount: '20', details: { merchant: 'OpenAI', transaction_type: 'CONSUME' } })
const references: Partial<Record<AccountingKind, unknown[]>> = { 'vcc-cards': [card], 'vcc-transactions': [purchase] }

async function render(kind: AccountingKind | 'orders', extra: Record<string, string> = {}, query: Record<string, string> = {}) {
  route.name = kind === 'orders' ? 'AdminAccountingOrderNew' : 'AdminAccountingRecordNew'
  route.params = kind === 'orders' ? { organizationId: 'org-one' } : { organizationId: 'org-one', kind, ...extra }
  route.query = query
  const wrapper = mount(AccountingRecordFormView, { attachTo: document.body, global: { plugins: [i18n()], stubs } })
  await flushPromises()
  return wrapper
}
function field(wrapper: Awaited<ReturnType<typeof render>>, key: string) {
  return wrapper.find(`[data-field="${key}"]`)
}
async function submit(wrapper: Awaited<ReturnType<typeof render>>) {
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  return api.mutate.mock.calls.at(-1)
}

describe('accounting record form', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.organizations.mockResolvedValue([organization])
    api.receivingAccounts.mockResolvedValue([receivingAccount])
    api.organizationContacts.mockResolvedValue([])
    api.listRecords.mockImplementation(async (kind: AccountingKind) => ({ items: references[kind] || [], total: 0 }))
    api.mutate.mockResolvedValue({ id: 'saved-one', organization_id: 'org-one' })
    api.order.mockResolvedValue({ id: 'order-one', name: 'Q4 席位', organization_id: 'org-one' })
  })

  it('associates the receiving account selector with its visible label', async () => {
    const wrapper = await render('transfers')
    const input = wrapper.get('[data-testid="receiving-account-select"]')
    expect(wrapper.get(`label[for="${input.attributes('id')}"]`).text()).toBe('收款账户')
  })

  it('announces and focuses rejected fields, then clears their error when edited', async () => {
    api.mutate.mockRejectedValueOnce({ reason: 'ACCOUNTING_INVALID', metadata: { field: 'bank_reference' } })
    const wrapper = await render('transfers')
    await wrapper.get('[data-testid="amount-input"]').setValue('106')
    await wrapper.get('[data-testid="detail-bank_reference"]').setValue('INVALID')
    await wrapper.get('[data-testid="detail-sales_amount"]').setValue('106')
    await wrapper.get('[data-testid="tax-rate-input"]').setValue('6')
    await submit(wrapper)
    const input = wrapper.get('[data-testid="detail-bank_reference"]')
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(wrapper.get(`[id="${input.attributes('aria-describedby')}"]`).text()).toContain('银行流水号')
    expect(document.activeElement).toBe(input.element)
    await input.setValue('BANK-001')
    expect(input.attributes('aria-invalid')).toBeUndefined()
    expect(input.attributes('aria-describedby')).toBeUndefined()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('focuses the error summary when the server cannot identify an editable field', async () => {
    api.mutate.mockRejectedValueOnce({ code: 'ERR_NETWORK' })
    const wrapper = await render('transfers')
    await wrapper.get('[data-testid="amount-input"]').setValue('106')
    await wrapper.get('[data-testid="detail-bank_reference"]').setValue('BANK-001')
    await wrapper.get('[data-testid="detail-sales_amount"]').setValue('106')
    await wrapper.get('[data-testid="tax-rate-input"]').setValue('6')
    await submit(wrapper)
    expect(document.activeElement).toBe(wrapper.get('[role="alert"]').element)
    expect(wrapper.get('[role="alert"]').text()).toContain('检查网络后重试')
  })

  it('records a receipt for the current organization without asking for it again', async () => {
    const wrapper = await render('transfers')

    expect(wrapper.text()).toContain('上海星海智能科技有限公司')
    expect(wrapper.find('select option[value="org-one"]').exists()).toBe(false)
    expect((wrapper.get('[data-testid="receiving-account-select"]').element as HTMLSelectElement).value).toBe('account-one')
    expect(field(wrapper, 'difference_reason').exists()).toBe(false)

    const proof = new File(['proof'], '到账凭证.pdf', { type: 'application/pdf' })
    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', { value: [proof] })
    await fileInput.trigger('change')
    expect(wrapper.text()).toContain(proof.name)

    await wrapper.get('[data-testid="amount-input"]').setValue('106')
    await wrapper.get('[data-testid="detail-bank_reference"]').setValue('BANK-001')
    await wrapper.get('[data-testid="detail-sales_amount"]').setValue('106')
    await wrapper.get('[data-testid="tax-rate-input"]').setValue('6')
    expect(wrapper.get('[data-testid="transfer-calculation"]').text()).toContain('6.00')
    expect(field(wrapper, 'difference_reason').exists()).toBe(false)

    const [path, payload, , method] = (await submit(wrapper))!
    expect(path).toBe('transfers')
    expect(method).toBe('post')
    expect(api.upload).toHaveBeenCalledWith(proof, 'transfers', expect.objectContaining({ id: 'saved-one', organization_id: 'org-one' }))
    expect(payload).toMatchObject({
      organization_id: 'org-one',
      currency: 'CNY',
      amount: '106',
      details: {
        payer_name: '上海星海智能科技有限公司',
        receiving_account_id: 'account-one',
        bank_reference: 'BANK-001',
        sales_amount: '106',
        tax_inclusive: true,
        tax_rate: '0.06',
        difference_reason: '',
      },
    })
    expect(router.push).toHaveBeenCalledWith({
      name: 'AdminAccountingRecord',
      params: { organizationId: 'org-one', kind: 'transfers', id: 'saved-one' },
      query: {},
    })
  })

  it('asks for a reason only when the receipt differs from the receivable', async () => {
    const wrapper = await render('transfers')
    await wrapper.get('[data-testid="amount-input"]').setValue('100')
    await wrapper.get('[data-testid="detail-sales_amount"]').setValue('106')
    await wrapper.get('[data-testid="tax-basis-exclusive"]').trigger('click')
    await wrapper.get('[data-testid="tax-rate-input"]').setValue('6')

    const reason = field(wrapper, 'difference_reason').get('textarea')
    expect(reason.attributes('required')).toBeDefined()
    expect(wrapper.get('[data-testid="transfer-calculation"]').text()).toContain('112.36')
  })

  it('requires an explicit receiving account when several match the currency', async () => {
    api.receivingAccounts.mockResolvedValue([receivingAccount, { ...receivingAccount, id: 'account-two', bank_account: '6225888800005678' }])
    const wrapper = await render('transfers')
    expect((wrapper.get('[data-testid="receiving-account-select"]').element as HTMLSelectElement).value).toBe('')
  })

  it('shows card transaction fields only for the chosen transaction type', async () => {
    const wrapper = await render('vcc-transactions', {}, { parent_id: 'card-one', order_id: 'order-one' })

    expect((field(wrapper, 'parent_id').get('select').element as HTMLSelectElement).value).toBe('card-one')
    expect(field(wrapper, 'direction').exists()).toBe(false)
    expect(field(wrapper, 'original_transaction_id').exists()).toBe(false)
    expect(field(wrapper, 'vendor_invoice_id').exists()).toBe(false)

    await wrapper.get('[data-testid="transaction_type-REFUND"]').trigger('click')
    expect(field(wrapper, 'original_transaction_id').get('select').attributes('required')).toBeDefined()
    expect(field(wrapper, 'original_transaction_id').text()).toContain('OpenAI')

    await wrapper.get('[data-testid="transaction_type-ADJUSTMENT"]').trigger('click')
    expect(field(wrapper, 'direction').exists()).toBe(true)
    expect(field(wrapper, 'original_transaction_id').exists()).toBe(false)
    await wrapper.get('[data-testid="direction-CREDIT"]').trigger('click')
    expect(field(wrapper, 'original_transaction_id').get('select').attributes('required')).toBeUndefined()

    await wrapper.get('[data-testid="transaction_type-CONSUME"]').trigger('click')
    await field(wrapper, 'ai_vendor').get('input').setValue(true)
    expect(field(wrapper, 'vendor_invoice_id').exists()).toBe(true)
    await wrapper.get('[data-testid="amount-input"]').setValue('12.5')
    await wrapper.get('[data-testid="detail-merchant"]').setValue('Anthropic')
    await wrapper.get('[data-testid="detail-provider_transaction_id"]').setValue('TX-9')

    const [path, payload] = (await submit(wrapper))!
    expect(path).toBe('vcc-transactions')
    expect(payload).toMatchObject({
      parent_id: 'card-one',
      order_id: 'order-one',
      currency: 'USD',
      details: { transaction_type: 'CONSUME', direction: '', original_transaction_id: '', ai_vendor: true },
    })
  })

  it('links an AI invoice only for AI vendor expenses', async () => {
    const wrapper = await render('expenses')
    expect(field(wrapper, 'vendor_invoice_id').exists()).toBe(false)
    await field(wrapper, 'category').get('select').setValue('AI_VENDOR')
    expect(field(wrapper, 'vendor_invoice_id').exists()).toBe(true)
  })

  it('totals invoice amounts and uses the issue date as business time', async () => {
    const wrapper = await render('invoices')
    expect((wrapper.get('[data-testid="detail-recipient_name"]').element as HTMLInputElement).value).toBe('上海星海智能科技有限公司')
    await wrapper.get('[data-testid="detail-invoice_code"]').setValue('0310')
    await wrapper.get('[data-testid="detail-invoice_number"]').setValue('INV-1')
    await wrapper.get('[data-testid="detail-issue_date"]').setValue('2026-10-01')
    await wrapper.get('[data-testid="detail-issuer_name"]').setValue('我方公司')
    await wrapper.get('[data-testid="detail-net_amount"]').setValue('100')
    await wrapper.get('[data-testid="detail-tax_amount"]').setValue('6.5')
    expect(wrapper.get('[data-testid="invoice-total"]').text()).toContain('106.50')

    const [, payload] = (await submit(wrapper))!
    expect(payload).toMatchObject({ amount: '106.5', occurred_at: new Date('2026-10-01T00:00').toISOString() })
  })

  it('fills the contract end date from a term shortcut', async () => {
    const wrapper = await render('contracts')
    await wrapper.get('[data-testid="detail-signed_at"]').setValue('2026-01-31')
    await wrapper.get('[data-testid="detail-effective_at"]').setValue('2026-01-31')
    const twelve = wrapper.get('[data-testid="contract-period-shortcuts"]').findAll('button')[1]!
    await twelve.trigger('click')
    expect((wrapper.get('[data-testid="detail-expires_at"]').element as HTMLInputElement).value).toBe('2027-01-30')
  })

  it('keeps card secrets unless new values are entered when editing', async () => {
    api.getRecord.mockResolvedValue(record({ id: 'card-one', status: 'ACTIVE', currency: 'USD', amount: '0', version: 4, details: { alias: '研发卡', provider: 'Airwallex', last_four: '4242' } }))
    const wrapper = await render('vcc-cards', { id: 'card-one' })

    expect(wrapper.text()).toContain('留空表示保持原值')
    expect(wrapper.find('[data-testid="amount-input"]').exists()).toBe(false)
    const [path, payload, , method] = (await submit(wrapper))!
    expect(path).toBe('vcc-cards/card-one')
    expect(method).toBe('patch')
    expect(payload.version).toBe(4)
    expect(payload.amount).toBe('0')
    expect(payload.details).not.toHaveProperty('card_number')
    expect(payload.details).not.toHaveProperty('expiry')
    expect(payload.details).toMatchObject({ alias: '研发卡', provider: 'Airwallex' })
    expect(payload).not.toHaveProperty('order_id')
  })

  it('returns to the page that started the form after saving', async () => {
    const wrapper = await render('vcc-transactions', {}, { parent_id: 'card-one', order_id: 'order-one', return_to: '/admin/accounting/organizations/org-one/vcc-cards/card-one' })
    await wrapper.get('[data-testid="amount-input"]').setValue('5')
    await wrapper.get('[data-testid="detail-merchant"]').setValue('OpenAI')
    await wrapper.get('[data-testid="detail-provider_transaction_id"]').setValue('TX-1')
    await submit(wrapper)
    expect(router.push).toHaveBeenCalledWith('/admin/accounting/organizations/org-one/vcc-cards/card-one')
  })

  it('only records cards inside an order', async () => {
    const wrapper = await render('vcc-cards')
    expect(wrapper.get('[data-testid="order-required"]').text()).toContain('只能在订单里登记')
    expect(wrapper.find('[data-field="card_number"]').exists()).toBe(false)
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()

    const inOrder = await render('vcc-cards', {}, { order_id: 'order-one' })
    expect(inOrder.text()).toContain('Q4 席位')
    expect(inOrder.get('[data-field="card_number"] input').attributes('required')).toBeDefined()
    expect(inOrder.find('[data-field="last_four"]').exists()).toBe(false)
  })

  it('raises sales invoices only, when issued from an order', async () => {
    const wrapper = await render('invoices', {}, { order_id: 'order-one' })
    expect(wrapper.get('[data-testid="invoice_type-SALES"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-testid="invoice_type-PURCHASE"]').attributes('disabled')).toBeDefined()
  })

  it('creates an order together with its first payment', async () => {
    api.mutate.mockResolvedValue({ id: 'order-new', name: 'x' })
    const wrapper = await render('orders')
    expect((wrapper.get('[data-testid="order-name"]').element as HTMLInputElement).value).toMatch(/^上海星海智能科技有限公司 \d{4}-\d{2}-\d{2}$/)
    await wrapper.get('[data-testid="order-name"]').setValue('Q4 ChatGPT 席位')
    await wrapper.get('[data-testid="amount-input"]').setValue('1000')
    await wrapper.get('[data-testid="detail-bank_reference"]').setValue('BANK-9')
    await wrapper.get('[data-testid="detail-sales_amount"]').setValue('1000')
    await wrapper.get('[data-testid="tax-rate-input"]').setValue('6')

    const [path, body] = (await submit(wrapper))!
    expect(path).toBe('orders')
    expect(body).toMatchObject({
      organization_id: 'org-one',
      name: 'Q4 ChatGPT 席位',
      transfer: { amount: '1000', details: { bank_reference: 'BANK-9', receiving_account_id: 'account-one', payer_name: '上海星海智能科技有限公司' } },
    })
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminAccountingOrder', params: { organizationId: 'org-one', orderId: 'order-new' }, query: {} })
  })

  it('can start an order before the payment arrives', async () => {
    api.mutate.mockResolvedValue({ id: 'order-new', name: 'x' })
    const wrapper = await render('orders')
    await wrapper.get('[data-testid="order-with-receipt"]').setValue(false)
    expect(wrapper.find('[data-testid="form-section-receipt"]').exists()).toBe(false)
    const [, body] = (await submit(wrapper))!
    expect(body).not.toHaveProperty('transfer')
  })
})
