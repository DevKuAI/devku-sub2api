import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const list = vi.hoisted(() => vi.fn())
const setSubscriptionAccountAccess = vi.hoisted(() => vi.fn())
const appStore = vi.hoisted(() => ({ showError: vi.fn() }))

vi.mock('@/api/subscriptionAccounts', () => ({ default: { list } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/composables/useSubscriptionAccountAccess', () => ({
  useSubscriptionAccountAccess: () => ({ setSubscriptionAccountAccess }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

import SubscriptionAccountsView from '../SubscriptionAccountsView.vue'

describe('SubscriptionAccountsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    list.mockResolvedValue([{
      id: 8,
      name: 'Read-only subscription',
      platform: 'openai',
      type: 'oauth',
      status: 'active',
      last_used_at: null,
      expires_at: null,
      created_at: '2026-09-04T01:02:03Z',
      usage: null,
    }])
  })

  it('shows assigned subscription information without operation controls', async () => {
    const wrapper = mount(SubscriptionAccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          SubscriptionAccountUsage: true,
          PlatformTypeBadge: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(list).toHaveBeenCalledWith(true)
    expect(wrapper.text()).toContain('Read-only subscription')
    expect(wrapper.findAll('button')).toHaveLength(0)
    expect(setSubscriptionAccountAccess).toHaveBeenCalledWith(true)
  })
})
