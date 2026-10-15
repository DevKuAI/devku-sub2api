import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import type { AccountingRecord } from '../types'
const api = vi.hoisted(() => ({
  workflow: vi.fn(),
  mutate: vi.fn(),
  listRecords: vi.fn(),
  attachments: vi.fn(),
  statementCSV: vi.fn(),
  stableRequest: () => Object.assign(() => 'workflow-key', { reset: vi.fn() }),
}))
vi.mock('../api', () => api)
vi.mock('file-saver', () => ({ saveAs: vi.fn() }))
import Operations from '../AccountingOperations.vue'
enableAutoUnmount(afterEach)
const record: AccountingRecord = {
  id: 'ar-one',
  organization_id: 'org_one',
  currency: 'CNY',
  amount: '100',
  status: 'CONFIRMED',
  details: { name: '服务费', outstanding_amount: '60' },
  occurred_at: '2026-01-01T00:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
  customer_visible: false,
  version: 1,
}
function render(
  props: Partial<InstanceType<typeof Operations>['$props']> = {},
) {
  return mount(Operations, {
    props: {
      organizationId: 'org_one',
      organizationName: '示例企业',
      ...props,
    },
    global: {
      plugins: [
        createI18n({
          legacy: false,
          locale: 'zh',
          messages: { zh },
          messageCompiler: (message) => (context) =>
            String(message).replace(/\{(\w+)\}/g, (_m, key: string) =>
              String(context.named(key)),
            ),
        }),
      ],
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          template:
            '<div v-if="show" class="dialog"><h2>{{title}}</h2><slot/><slot name="footer"/></div>',
        },
      },
    },
  })
}
beforeEach(() => {
  vi.clearAllMocks()
  api.workflow.mockResolvedValue([])
  api.mutate.mockResolvedValue({})
  api.listRecords.mockResolvedValue({ items: [], total: 0 })
  api.attachments.mockResolvedValue([])
})
describe('finance workflow UI', () => {
  it('shows aggregate workbench counts and opens the selected business record', async () => {
    api.workflow.mockResolvedValue({
      items: [
        {
          kind: 'expenses',
          id: 'expense-one',
          name: '供应商费用',
          reason: 'unpaid',
          amount: '60',
          currency: 'CNY',
          status: 'UNPAID',
        },
      ],
      total: 1,
      counts: { unpaid: 1 },
      positions: { CNY: { cash: '100', card: '20' } },
      pending_consumption: [{ currency: 'CNY', amount: '5' }],
    })
    const w = render()
    await flushPromises()
    expect(w.text()).toContain('待确认消费')
    await w
      .findAll('button')
      .find((b) => b.text().includes('供应商费用'))!
      .trigger('click')
    expect(w.emitted('open-record')?.[0]).toEqual(['expenses', 'expense-one'])
  })
  it('settles only eligible receipts for the current organization and currency', async () => {
    api.listRecords.mockResolvedValue({
      items: [
        {
          ...record,
          id: 'receipt-one',
          status: 'RECEIVED',
          details: { payer_name: '本企业到账' },
        },
        {
          ...record,
          id: 'other',
          organization_id: 'org_other',
          status: 'RECEIVED',
          details: { payer_name: '外部企业' },
        },
        {
          ...record,
          id: 'wrong-currency',
          currency: 'USD',
          status: 'RECEIVED',
          details: { payer_name: '美元到账' },
        },
      ],
      total: 3,
    })
    const w = render({ record, kind: 'receivables' })
    await flushPromises()
    await w
      .findAll('button')
      .find((b) => b.text() === '核销收款')!
      .trigger('click')
    await flushPromises()
    expect(w.get('.dialog').text()).not.toContain('外部企业')
    expect(w.get('.dialog').text()).not.toContain('美元到账')
    await w.get('.dialog select').setValue('receipt-one')
    await w.get('.dialog form').trigger('submit')
    await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith(
      'receivables/ar-one/settlements',
      expect.objectContaining({
        organization_id: 'org_one',
        transfer_id: 'receipt-one',
        amount: '60',
      }),
      'workflow-key',
    )
  })
  it('keeps an idempotent cost request retryable after failure', async () => {
    api.mutate.mockRejectedValueOnce({
      reason: 'ACCOUNTING_AMOUNT_CONFLICT',
      message: '金额超过剩余可分摊额度',
    })
    api.listRecords.mockImplementation(async (kind: string) => ({
      items:
        kind === 'vendor-invoices'
          ? [{ ...record, id: 'invoice-one', details: { vendor: 'Vendor' } }]
          : kind === 'vcc-transactions'
            ? [
                {
                  ...record,
                  id: 'consume-one',
                  details: {
                    merchant: 'Merchant',
                    transaction_type: 'CONSUME',
                  },
                },
              ]
            : [],
      total: 1,
    }))
    const w = render({ section: 'costs' })
    await flushPromises()
    await w
      .findAll('button')
      .find((b) => b.text() === '登记成本分摊')!
      .trigger('click')
    await flushPromises()
    const selects = w.findAll('.dialog select')
    await selects[0]!.setValue('invoice-one')
    await selects[2]!.setValue('consume-one')
    await w.get('.dialog input[inputmode=decimal]').setValue('10')
    await w.get('.dialog form').trigger('submit')
    await flushPromises()
    expect(w.get('[role=alert]').text()).toContain('金额超过')
    await w.get('.dialog form').trigger('submit')
    await flushPromises()
    expect(api.mutate).toHaveBeenCalledTimes(2)
    expect(api.mutate.mock.calls[0]![2]).toBe(api.mutate.mock.calls[1]![2])
  })
  it('does not show closing or reopening controls to customer accounts, including administrators', async () => {
    api.workflow.mockResolvedValue([
      {
        id: 'p',
        month: '2026-01',
        status: 'CLOSED',
        revision: 1,
        statement_id: 's',
      },
    ])
    const w = render({
      section: 'periods',
      customer: true,
      administrator: true,
    })
    await flushPromises()
    expect(w.text()).not.toContain('执行月结')
    expect(w.text()).not.toContain('重新开账')
    expect(api.workflow).toHaveBeenCalledWith(
      'periods',
      'org_one',
      true,
      expect.anything(),
    )
    expect(api.mutate).not.toHaveBeenCalled()
  })
  it('distinguishes unknown opening balances and filters private statement records', async () => {
    api.workflow.mockResolvedValueOnce([
      { id: 'p', month: '2026-01', status: 'CLOSED', statement_id: 's' },
    ])
    api.workflow.mockResolvedValueOnce({
      month: '2026-01',
      opening: null,
      movement: {},
      closing: { CNY: { cash: '20', card: '10' } },
      records: [
        {
          kind: 'expenses',
          record: { ...record, details: { supplier: 'PRIVATE_SUPPLIER' } },
        },
        {
          kind: 'transfers',
          record: {
            ...record,
            details: {
              payer_name: '客户到账',
              receiving_account: 'PRIVATE_BANK',
            },
          },
        },
      ],
    })
    const w = render({ section: 'periods', customer: true })
    await flushPromises()
    await w
      .findAll('button')
      .find((b) => b.text() === '记录详情')!
      .trigger('click')
    await flushPromises()
    expect(w.text()).toContain('期初余额未知')
    expect(w.text()).toContain('客户到账')
    expect(w.text()).not.toContain('PRIVATE')
  })
})
