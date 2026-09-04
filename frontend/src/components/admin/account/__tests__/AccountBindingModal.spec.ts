import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const accountAPI = vi.hoisted(() => ({
  getById: vi.fn(),
  bindUser: vi.fn()
}))
const userAPI = vi.hoisted(() => ({ list: vi.fn() }))
const appStore = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
const refreshSubscriptionAccountAccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin', () => ({ adminAPI: { accounts: accountAPI, users: userAPI } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/composables/useSubscriptionAccountAccess', () => ({
  useSubscriptionAccountAccess: () => ({ refreshSubscriptionAccountAccess })
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key })
}))

import AccountBindingModal from '../AccountBindingModal.vue'

const account = {
  id: 7,
  name: 'Team subscription',
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  bound_user_id: null
}

const SelectStub = {
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<button data-test="user-select" @click="$emit(\'update:modelValue\', 84)">{{ modelValue }}</button>'
}

function mountModal() {
  return mount(AccountBindingModal, {
    props: { show: true, account },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          emits: ['close'],
          template: '<section v-if="show" role="dialog"><h1>{{ title }}</h1><slot /></section>'
        },
        Select: SelectStub,
        PlatformTypeBadge: true,
        Icon: true,
        ConfirmDialog: true
      }
    }
  })
}

describe('AccountBindingModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    accountAPI.getById.mockResolvedValue({ ...account })
    userAPI.list.mockResolvedValue({
      items: [{ id: 84, username: 'Bob', email: 'bob@example.com' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    accountAPI.bindUser.mockResolvedValue({
      ...account,
      bound_user_id: 84,
      bound_user: { id: 84, username: 'Bob', email: 'bob@example.com' }
    })
    refreshSubscriptionAccountAccess.mockResolvedValue(true)
  })

  it('binds the selected user and reports the updated account', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await wrapper.get('[data-test="user-select"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(accountAPI.bindUser).toHaveBeenCalledWith(7, 84, null)
    expect(appStore.showSuccess).toHaveBeenCalledWith('admin.accounts.binding.saved')
    expect(refreshSubscriptionAccountAccess).toHaveBeenCalledWith(true)
    expect(wrapper.emitted('updated')?.[0]?.[0]).toEqual(expect.objectContaining({
      id: 7,
      bound_user_id: 84
    }))
  })
})
