import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'

const api = vi.hoisted(() => ({
  organizations: vi.fn(),
  receivingAccounts: vi.fn(),
  createReceivingAccount: vi.fn(),
  updateReceivingAccount: vi.fn(),
}))
const route = vi.hoisted(() => ({
  name: 'AdminAccountingReceivingAccountNew',
  params: {} as Record<string, string>,
  query: {} as Record<string, string>,
}))
const router = vi.hoisted(() => ({ push: vi.fn() }))

vi.mock('../api', () => api)
vi.mock('vue-router', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-router')>()
  return {
    ...original,
    useRoute: () => route,
    useRouter: () => router,
    onBeforeRouteLeave: vi.fn(),
  }
})

import OrganizationReceivingAccountFormView from '../OrganizationReceivingAccountFormView.vue'

enableAutoUnmount(afterEach)

function render() {
  return mount(OrganizationReceivingAccountFormView, {
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
      },
    },
  })
}

describe('receiving account form page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.name = 'AdminAccountingReceivingAccountNew'
    route.params = {}
    route.query = {}
    api.receivingAccounts.mockResolvedValue([])
    api.createReceivingAccount.mockResolvedValue({ id: 'account-one' })
    api.updateReceivingAccount.mockResolvedValue({ id: 'account-one' })
  })

  it('creates the fixed account on a dedicated page', async () => {
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('[data-testid="accounting-form-page"]').exists()).toBe(true)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    const inputs = wrapper.findAll('input')
    await inputs[0]!.setValue('CNY')
    await inputs[1]!.setValue('上海星海智能科技有限公司')
    await inputs[2]!.setValue('招商银行')
    await inputs[3]!.setValue('上海科技园支行')
    await inputs[4]!.setValue('6225888800001234')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.createReceivingAccount).toHaveBeenCalledWith({
      currency: 'CNY',
      account_name: '上海星海智能科技有限公司',
      bank_name: '招商银行',
      bank_account: '6225888800001234',
      bank_branch: '上海科技园支行',
      remark: '',
    })
    expect(router.push).toHaveBeenCalledWith({
      name: 'AdminAccountingReceivingAccounts',
    })
  })

  it('keeps currency fixed while editing an existing account', async () => {
    route.name = 'AdminAccountingReceivingAccountEdit'
    route.params = { accountId: 'account-one' }
    api.receivingAccounts.mockResolvedValue([
      {
        id: 'account-one',
        currency: 'CNY',
        account_name: '上海星海智能科技有限公司',
        bank_name: '招商银行',
        bank_account: '6225888800001234',
        bank_branch: '',
        remark: '',
      },
    ])
    const wrapper = render()
    await flushPromises()

    const currencyInput = wrapper.findAll('input')[0]!
    expect((currencyInput.element as HTMLInputElement).value).toBe('CNY')
    expect(currencyInput.attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.updateReceivingAccount).toHaveBeenCalledWith(
      'account-one',
      expect.objectContaining({ currency: 'CNY', bank_account: '6225888800001234' }),
    )
  })
})
