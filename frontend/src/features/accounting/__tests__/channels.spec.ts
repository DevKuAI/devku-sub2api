import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { ChannelAccountDetail, ChannelAccountSummary } from '../types'
import { i18n, record, stubs } from './testing'

const api = vi.hoisted(() => ({
  channelAccounts: vi.fn(),
  channelAccount: vi.fn(),
  channelLines: vi.fn(),
  importChannel: vi.fn(),
  convertChannel: vi.fn(),
  channelTemplate: vi.fn(),
  revokeChannelImport: vi.fn(),
  channelFileURL: vi.fn(),
  uploadChannelProof: vi.fn(),
  channelProofURL: vi.fn(),
  deleteChannelProof: vi.fn(),
  bindChannelCard: vi.fn(),
  rebookChannelLine: vi.fn(),
  organizations: vi.fn(),
  listRecords: vi.fn(),
  key: () => 'proof-key',
  stableRequest: () => Object.assign(() => 'request-key', { reset: vi.fn() }),
}))
const router = vi.hoisted(() => ({ push: vi.fn() }))
const auth = vi.hoisted(() => ({ isAdmin: true }))

vi.mock('../api', () => api)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-router', () => ({
  useRouter: () => router,
  useRoute: () => ({ params: { accountId: 'account-one' } }),
  onBeforeRouteLeave: vi.fn(),
}))

import ChannelsView from '../ChannelsView.vue'
import ChannelImportView from '../ChannelImportView.vue'
import ChannelAccountView from '../ChannelAccountView.vue'

enableAutoUnmount(afterEach)

const mountOptions = { global: { plugins: [i18n()], stubs: { ...stubs, Pagination: true } } }

function summary(overrides: Partial<ChannelAccountSummary> = {}): ChannelAccountSummary {
  return {
    account: { id: 'account-one', platform: 'SUNRATE', account_ref: '72778936', name: 'Hong Kong Rich Sum Technology Co. Limited', remark: '', created_at: '2026-10-28T00:00:00Z' },
    wallets: [
      { wallet: 'PAYMENT', currency: 'HKD', opening: '0.00', balance: '0.00', lines: 5, first_at: '2026-08-30T16:00:00Z', last_at: '2026-10-27T02:59:06Z', gaps: [], negative_spans: [],
        reported: { amount: '0.00', as_of: '2026-10-28T15:59:59Z', source: 'hkd.pdf', import_id: 'import-one', books: '0.00', difference: '0.00', matched: true } },
      { wallet: 'PAYMENT', currency: 'USD', opening: '0.00', balance: '2172.87', lines: 8, first_at: '2026-09-02T01:42:16Z', last_at: '2026-10-27T02:59:27Z', gaps: [], negative_spans: [],
        reported: { amount: '2172.87', as_of: '2026-10-28T15:59:59Z', source: 'usd.pdf', import_id: 'import-one', books: '2172.87', difference: '0.00', matched: true } },
      { wallet: 'CARD', currency: 'USD', opening: '0.00', balance: '1026.50', lines: 23, first_at: '2026-09-02T01:42:51Z', last_at: '2026-10-27T02:59:27Z', gaps: [],
        negative_spans: [{ from: '2026-10-09T16:19:15Z', to: '2026-10-15T01:41:28Z', lowest: '-1.00' }],
        reported: { amount: '1026.50', as_of: '2026-10-27T02:59:27Z', source: 'card.xlsx', import_id: 'import-one', books: '1026.50', difference: '0.00', matched: true } },
    ],
    positions: [{ currency: 'HKD', amount: '0.00' }, { currency: 'USD', amount: '3199.37' }],
    fee_costs: [{ currency: 'HKD', amount: '34.64' }, { currency: 'USD', amount: '4.25' }],
    unpaired_transfers: 0,
    missing_proofs: 0,
    cards: 4,
    last_import_at: '2026-10-28T10:00:00Z',
    pending_cards: 0,
    pending_orders: 0,
    pending_review: 0,
    ...overrides,
  }
}

describe('channel pages', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    auth.isAdmin = true
  })

  it('shows every balance against the platform and what needs attention', async () => {
    api.channelAccounts.mockResolvedValue([summary()])
    const wrapper = mount(ChannelsView, mountOptions)
    await flushPromises()

    expect(wrapper.text()).toContain('Hong Kong Rich Sum Technology Co. Limited')
    expect(wrapper.get('[data-testid="channel-wallet-PAYMENT-USD"]').text()).toContain('已对平')
    expect(wrapper.get('[data-testid="channel-wallet-CARD-USD"]').text()).toContain('卡余额')
    expect(wrapper.text()).toContain('3,199.37')
    expect(wrapper.text()).toContain('34.64')
    expect(wrapper.text()).toContain('出现过负余额')
  })

  it('books an upload and lists every rejected row', async () => {
    const wrapper = mount(ChannelImportView, mountOptions)
    const file = new File(['csv'], 'lines.csv', { type: 'text/csv' })
    const input = wrapper.get('[data-testid="channel-file-input"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    expect(wrapper.text()).toContain('已选 1 个文件')

    api.importChannel.mockRejectedValueOnce({
      reason: 'ACCOUNTING_CHANNEL_FILES_INVALID',
      metadata: { problems: JSON.stringify([{ file: 'lines.csv', row: 3, column: 'amount', message: 'sign does not match type CONSUME' }]) },
    })
    await wrapper.get('[data-testid="channel-import-submit"]').trigger('click')
    await flushPromises()
    const problems = wrapper.get('[data-testid="channel-problems"]').text()
    expect(problems).toContain('lines.csv 第 3 行')
    expect(problems).toContain('sign does not match type CONSUME')

    api.importChannel.mockResolvedValueOnce({
      import: { id: 'import-one', converter: 'TEMPLATE', files: [], reported_balances: [], total_rows: 36, new_rows: 36, duplicate_rows: 0, pending_rows: 0, status: 'APPLIED', account_ids: ['account-one'], created_by: 1, created_at: '2026-10-28T10:00:00Z' },
      accounts: [summary()],
      bookings: { BOOKED: 5, PENDING_CARD: 1 },
    })
    await wrapper.get('[data-testid="channel-import-submit"]').trigger('click')
    await flushPromises()
    expect(api.importChannel).toHaveBeenLastCalledWith([file], 'request-key')
    expect(wrapper.get('[data-testid="channel-import-result"]').text()).toContain('新增 36 行')
    expect(wrapper.get('[data-testid="channel-import-bookings"]').text()).toBe('卡消费：5 笔入企业账，1 笔待绑定卡。')
  })

  it('binds statement cards and retries card lines that wait for review', async () => {
    const cardSpend = (overrides: object) => ({ id: 'channel-card-one', card_ref: '2108375553744965633', card_last4: '0956', card_name: '', currency: 'USD', spends: 1, spend: '200.00', refund: '0.00', pending: 1, ...overrides })
    api.channelAccount.mockResolvedValue({
      ...summary({ pending_cards: 1, pending_review: 1 }),
      months: [], conversions: [], unpaired: [], imports: [],
      card_spend: [cardSpend({}), cardSpend({ id: 'channel-card-two', card_ref: '2095346501199990785', card_last4: '3926', card_name: '中新软件', pending: 0, organization_id: 'org-one', organization_name: '中新软件', vcc_card_id: 'card-bound', vcc_card_name: '中新软件', bind_method: 'AUTO_CARD' })],
    })
    const line = (overrides: object) => ({ id: 'line-one', wallet: 'CARD', external_id: 'AJ1', occurred_at: '2026-10-12T02:00:00Z', type: 'REFUND', amount: '60.00', currency: 'USD', import_id: 'import-one', proofs: [], booking: 'PENDING_REVIEW', booking_note: 'amount', ...overrides })
    api.channelLines.mockResolvedValue({ items: [line({}), line({ id: 'line-two', external_id: 'AJ2', type: 'CONSUME', amount: '-200.00', booking: 'BOOKED', booking_note: '', order_id: 'order-one', order_name: 'Claude 席位' })], total: 2, page: 1, page_size: 50, pages: 1 })
    api.rebookChannelLine.mockResolvedValue({ booking: 'BOOKED', booking_note: '' })
    const wrapper = mount(ChannelAccountView, mountOptions)
    await flushPromises()

    expect(wrapper.get('[data-testid="channel-pending"]').text()).toContain('待绑定卡 1 笔')
    await wrapper.get('[data-testid="channel-show-pending"]').trigger('click'); await flushPromises()
    expect(api.channelLines).toHaveBeenLastCalledWith('account-one', expect.objectContaining({ booking: 'PENDING' }), expect.anything())
    const review = wrapper.get('[data-testid="channel-booking-PENDING_REVIEW"]')
    expect(review.text()).toContain('超过原消费可退金额')
    await review.get('[data-testid="channel-rebook"]').trigger('click'); await flushPromises()
    expect(api.rebookChannelLine).toHaveBeenCalledWith('line-one', 'request-key')
    expect(wrapper.get('[data-testid="channel-booking-BOOKED"]').text()).toContain('Claude 席位')

    // An unbound statement card is bound to a card of the chosen enterprise; cards holding another card ID are left out.
    api.organizations.mockResolvedValue([{ id: 'org-one', name: '银通源', summary: [] }])
    api.listRecords.mockResolvedValue({ items: [
      record({ id: 'card-legacy', status: 'ACTIVE', currency: 'USD', details: { alias: '旧卡', last_four: '0956' } }),
      record({ id: 'card-other', status: 'ACTIVE', currency: 'USD', details: { alias: '别的卡', last_four: '1111', provider_card_id: '999' } }),
    ], total: 2 })
    api.bindChannelCard.mockResolvedValue({})
    await wrapper.get('[data-testid="channel-card-2108375553744965633"] [data-testid="channel-bind-open"]').trigger('click'); await flushPromises()
    const form = wrapper.get('[data-testid="channel-bind-form"]')
    await form.get('[data-testid="channel-bind-organization"]').setValue('org-one'); await flushPromises()
    const options = form.findAll('[data-testid="channel-bind-card"] option').map((option) => option.text())
    expect(options).toHaveLength(2)
    expect(options[1]).toContain('旧卡')
    await form.get('[data-testid="channel-bind-card"]').setValue('card-legacy')
    await form.trigger('submit'); await flushPromises()
    expect(api.bindChannelCard).toHaveBeenCalledWith('channel-card-one', 'card-legacy', 'request-key')
  })

  it('lets administrators revoke a batch inline with a reason', async () => {
    const detail: ChannelAccountDetail = {
      ...summary({ wallets: summary().wallets.map((w) => (w.wallet === 'CARD' ? { ...w, gaps: [{ after_external_id: 'b', after_at: '', before_external_id: 'e', before_at: '', expected_balance: '70.00', reported_balance: '90.00', difference: '20.00' }] } : w)) }),
      months: [{ month: '2026-10', currency: 'USD', totals: { FX_IN: '2982.20', TRANSFER_OUT: '-1429.00' }, fee_cost: '2.50' }],
      conversions: [{ at: '2026-10-27T02:59:06Z', reference: 'TXN1', sold: '23439.81', sold_currency: 'HKD', bought: '2982.20', bought_currency: 'USD', rate: '7.8599' }],
      card_spend: [],
      unpaired: [],
      imports: [{ id: 'import-one', converter: 'SUNRATE', files: [{ name: 'card.xlsx', sha256: 'abc', size: 10 }], reported_balances: [], total_rows: 36, new_rows: 36, duplicate_rows: 0, pending_rows: 0, status: 'APPLIED', account_ids: ['account-one'], created_by: 1, created_at: '2026-10-28T10:00:00Z' }],
    }
    api.channelAccount.mockResolvedValue(detail)
    api.channelLines.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 50, pages: 0 })
    api.revokeChannelImport.mockResolvedValue({})
    const wrapper = mount(ChannelAccountView, mountOptions)
    await flushPromises()

    expect(wrapper.get('[data-testid="channel-alerts"]').text()).toContain('b 之后到 e 之间缺少流水')
    expect(wrapper.get('[data-testid="channel-months"]').text()).toContain('购汇买入')
    expect(wrapper.text()).toContain('7.8599')

    await wrapper.get('[data-testid="channel-revoke-open"]').trigger('click')
    await wrapper.get('[data-testid="channel-revoke-form"] textarea').setValue('重新导出')
    await wrapper.get('[data-testid="channel-revoke-form"]').trigger('submit')
    await flushPromises()
    expect(api.revokeChannelImport).toHaveBeenCalledWith('import-one', '重新导出', 'request-key')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('hides revoking from finance users', async () => {
    auth.isAdmin = false
    api.channelAccount.mockResolvedValue({ ...summary(), months: [], conversions: [], card_spend: [], unpaired: [],
      imports: [{ id: 'import-one', converter: 'SUNRATE', files: [], reported_balances: [], total_rows: 1, new_rows: 1, duplicate_rows: 0, pending_rows: 0, status: 'APPLIED', account_ids: [], created_by: 1, created_at: '2026-10-28T10:00:00Z' }] })
    api.channelLines.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 50, pages: 0 })
    const wrapper = mount(ChannelAccountView, mountOptions)
    await flushPromises()
    expect(wrapper.find('[data-testid="channel-revoke-open"]').exists()).toBe(false)
  })

  it('asks for proofs on collections and uploads them inline', async () => {
    const collection = {
      id: 'line-one', wallet: 'PAYMENT', external_id: 'C26A26JRZ9', occurred_at: '2026-10-26T02:07:51Z', type: 'FUNDING_IN' as const,
      amount: '23439.81', currency: 'HKD', balance_after: '23439.81', fee_amount: '23.46', booking: 'INTERNAL', import_id: 'import-one', proofs: [],
    }
    api.channelAccount.mockResolvedValue({ ...summary({ missing_proofs: 2 }), months: [], conversions: [], card_spend: [], unpaired: [], imports: [] })
    api.channelLines.mockResolvedValue({ items: [collection], total: 1, page: 1, page_size: 50, pages: 1 })
    api.uploadChannelProof.mockResolvedValue({ id: 'proof-one', filename: '转账截图.png', content_type: 'image/png', size: 10, created_by: 1, created_at: '2026-10-28T10:00:00Z' })
    const wrapper = mount(ChannelAccountView, mountOptions)
    await flushPromises()

    expect(wrapper.get('[data-testid="channel-alerts"]').text()).toContain('2 笔入金缺凭证')
    expect(wrapper.get('[data-testid="channel-lines"]').text()).toContain('缺凭证')
    await wrapper.get('[data-testid="channel-missing-proofs"]').trigger('click')
    await flushPromises()
    expect(api.channelLines).toHaveBeenLastCalledWith('account-one', expect.objectContaining({ type: 'MISSING_PROOF' }), expect.anything())

    await wrapper.get('[data-testid="channel-proof-open"]').trigger('click')
    const file = new File(['png'], '转账截图.png', { type: 'image/png' })
    const input = wrapper.get('[data-testid="channel-proof-input"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    await flushPromises()
    expect(api.uploadChannelProof).toHaveBeenCalledWith('line-one', file, 'proof-key')
    expect(wrapper.get('[data-testid="channel-lines"]').text()).toContain('转账截图.png')
  })
})
