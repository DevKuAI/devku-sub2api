import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'

const api = vi.hoisted(() => ({ receivingAccounts: vi.fn() }))
const router = vi.hoisted(() => ({ push: vi.fn() }))

vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRouter: () => router }))

import ReceivingAccountsView from '../ReceivingAccountsView.vue'

enableAutoUnmount(afterEach)

describe('receiving accounts page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.receivingAccounts.mockResolvedValue([
      {
        id: 'account-one',
        currency: 'CNY',
        account_name: '上海星海智能科技有限公司',
        bank_name: '招商银行',
        bank_account: '6225888800001234',
        bank_branch: '上海科技园支行',
        remark: '',
      },
    ])
  })

  it('lists our accounts and links to dedicated create and edit pages', async () => {
    const wrapper = mount(ReceivingAccountsView, {
      global: {
        plugins: [
          createI18n({
            legacy: false,
            locale: 'zh',
            messages: { zh },
            messageCompiler: (message) => () => String(message),
          }),
        ],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
          RouterLink: {
            name: 'RouterLink',
            props: ['to'],
            template: '<a><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    expect(api.receivingAccounts).toHaveBeenCalledWith()
    expect(wrapper.text()).toContain('收款账户')
    expect(wrapper.text()).toContain('6225888800001234')
    const links = wrapper.findAllComponents({ name: 'RouterLink' })
    expect(links.find((link) => link.text().includes('新增账户'))!.props('to')).toEqual({
      name: 'AdminAccountingReceivingAccountNew',
    })
    expect(links.find((link) => link.attributes('aria-label') === '编辑 CNY 上海星海智能科技有限公司')!.props('to')).toEqual({
      name: 'AdminAccountingReceivingAccountEdit',
      params: { accountId: 'account-one' },
    })
  })
})
