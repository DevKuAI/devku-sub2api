import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { ChannelAccountDetail, ChannelAccountSummary } from '../types'
import { i18n, stubs } from './testing'

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
    })
    await wrapper.get('[data-testid="channel-import-submit"]').trigger('click')
    await flushPromises()
    expect(api.importChannel).toHaveBeenLastCalledWith([file], 'request-key')
    expect(wrapper.get('[data-testid="channel-import-result"]').text()).toContain('新增 36 行')
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
