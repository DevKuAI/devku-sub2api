import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import type { AccountingRecord, Vendor } from '../types'

export function i18n() {
  return createI18n({
    legacy: false,
    locale: 'zh',
    messages: { zh },
    messageCompiler: (message) => (context) =>
      String(message).replace(/\{(\w+)\}/g, (_match, key: string) => String(context.named(key))),
  })
}

export const stubs = {
  AppLayout: { template: '<div><slot /></div>' },
  Icon: true,
  RouterLink: { name: 'RouterLink', props: ['to'], template: '<a><slot /></a>' },
  BaseDialog: {
    props: ['show', 'title'],
    template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>',
  },
}

export function record(overrides: Partial<AccountingRecord> = {}): AccountingRecord {
  return {
    id: 'record-one',
    organization_id: 'org-one',
    status: 'DRAFT',
    currency: 'CNY',
    amount: '100',
    occurred_at: '2026-10-12T02:00:00Z',
    details: {},
    customer_visible: false,
    version: 1,
    created_at: '2026-10-12T02:00:00Z',
    ...overrides,
  }
}

export const organization = { id: 'org-one', name: '上海星海智能科技有限公司', summary: [] }

export const receivingAccount = {
  id: 'account-one',
  currency: 'CNY',
  account_name: '上海星海智能科技有限公司',
  bank_name: '招商银行',
  bank_account: '6225888800001234',
  bank_branch: '上海科技园支行',
  remark: '',
  created_at: '2026-10-12T00:00:00Z',
  updated_at: '2026-10-12T00:00:00Z',
}

function vendor(id: string, kind: Vendor['kind'], name: string, status: Vendor['status'] = 'ACTIVE'): Vendor {
  return { id, kind, name, aliases: [name.toLowerCase()], status, sort: 0, remark: '', created_at: '2026-10-01T00:00:00Z', updated_at: '2026-10-01T00:00:00Z' }
}
export const vendors: Vendor[] = [
  vendor('vendor-sunrate', 'CARD_PROVIDER', 'SUNRATE'),
  vendor('vendor-openai', 'AI_VENDOR', 'OpenAI'),
  vendor('vendor-anthropic', 'AI_VENDOR', 'Anthropic'),
  vendor('vendor-retired', 'AI_VENDOR', 'Retired', 'DISABLED'),
]
