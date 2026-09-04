import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const accountAPI = vi.hoisted(() => ({
  getById: vi.fn(),
  bindUser: vi.fn(),
}))
const userAPI = vi.hoisted(() => ({ list: vi.fn() }))
const router = vi.hoisted(() => ({ push: vi.fn() }))
const appStore = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
const refreshSubscriptionAccountAccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin', () => ({ adminAPI: { accounts: accountAPI, users: userAPI } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/composables/useSubscriptionAccountAccess', () => ({
  useSubscriptionAccountAccess: () => ({ refreshSubscriptionAccountAccess }),
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '7' } }),
  useRouter: () => router,
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

import AccountBindingView from '../AccountBindingView.vue'

const boundAccount = {
  id: 7,
  name: 'Team subscription',
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  bound_user_id: 42,
  bound_user: { id: 42, username: 'Alice', email: 'alice@example.com' },
}

function mountView() {
  return mount(AccountBindingView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Select: { template: '<div data-test="user-select" />' },
        PlatformTypeBadge: true,
        Icon: true,
        ConfirmDialog: {
          props: ['show'],
          emits: ['confirm', 'cancel'],
          template: '<button v-if="show" data-test="confirm-remove" @click="$emit(\'confirm\')">confirm</button>',
        },
      },
    },
  })
}

describe('AccountBindingView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    accountAPI.getById.mockResolvedValue({ ...boundAccount })
    accountAPI.bindUser.mockResolvedValue({ ...boundAccount, bound_user_id: null, bound_user: undefined })
    userAPI.list.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    refreshSubscriptionAccountAccess.mockResolvedValue(false)
  })

  it('removes the bound user after an explicit confirmation', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.btn-danger').trigger('click')
    await wrapper.get('[data-test="confirm-remove"]').trigger('click')
    await flushPromises()

    expect(accountAPI.bindUser).toHaveBeenCalledWith(7, null, 42)
    expect(appStore.showSuccess).toHaveBeenCalledWith('admin.accounts.binding.removed')
    expect(refreshSubscriptionAccountAccess).toHaveBeenCalledWith(true)
    expect(wrapper.find('.btn-danger').exists()).toBe(false)
  })

  it('reloads the latest binding after a concurrent update conflict', async () => {
    accountAPI.bindUser.mockRejectedValueOnce({ reason: 'ACCOUNT_BINDING_CONFLICT' })
    const latestAccount = {
      ...boundAccount,
      bound_user_id: 84,
      bound_user: { id: 84, username: 'Bob', email: 'bob@example.com' },
    }
    accountAPI.getById.mockResolvedValueOnce({ ...boundAccount }).mockResolvedValueOnce(latestAccount)
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.btn-danger').trigger('click')
    await wrapper.get('[data-test="confirm-remove"]').trigger('click')
    await flushPromises()

    expect(accountAPI.bindUser).toHaveBeenCalledWith(7, null, 42)
    expect(accountAPI.getById).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Bob')
    expect(appStore.showError).toHaveBeenCalledWith('admin.accounts.binding.conflict')
    expect(appStore.showSuccess).not.toHaveBeenCalled()
  })
})
