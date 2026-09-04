import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

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
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-04T00:00:00Z'))
    vi.clearAllMocks()
    list.mockResolvedValue([
      {
        id: 8,
        name: 'Read-only subscription',
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        last_used_at: null,
        expires_at: null,
        created_at: '2026-09-04T01:02:03Z',
        usage: {
          five_hour: {
            utilization: 0,
            resets_at: '2026-09-04T00:00:00Z',
            window_stats: {
              requests: 660,
              tokens: 60100000,
              cost: 54.82,
              standard_cost: 48,
              user_cost: 54.82,
            },
          },
          seven_day: {
            utilization: 7,
            resets_at: '2026-09-08T19:00:00Z',
            window_stats: {
              requests: 2300,
              tokens: 195800000,
              cost: 175.07,
              standard_cost: 160,
              user_cost: 175.07,
            },
          },
        },
      },
      {
        id: 9,
        name: 'Second subscription',
        platform: 'anthropic',
        type: 'oauth',
        status: 'active',
        last_used_at: '2026-09-04T02:03:04Z',
        expires_at: null,
        created_at: '2026-09-03T01:02:03Z',
        usage: null,
      },
    ])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows assigned subscription information without operation controls', async () => {
    const wrapper = mount(SubscriptionAccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          PlatformTypeBadge: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(list).toHaveBeenCalledWith(true)
    expect(wrapper.findAll('article')).toHaveLength(2)
    expect(wrapper.text()).toContain('Read-only subscription')
    expect(wrapper.text()).toContain('Second subscription')
    expect(wrapper.text()).toContain('660 req')
    expect(wrapper.text()).toContain('60.1M')
    expect(wrapper.text()).toContain('A $54.82')
    expect(wrapper.text()).toContain('U $54.82')
    expect(wrapper.text()).toContain('2.3K req')
    expect(wrapper.text()).toContain('195.8M')
    expect(wrapper.text()).toContain('A $175.07')
    expect(wrapper.text()).toContain('U $175.07')
    expect(wrapper.text()).toContain('5h')
    expect(wrapper.text()).toContain('7d')
    expect(wrapper.text()).toContain('0%')
    expect(wrapper.text()).toContain('7%')
    expect(wrapper.text()).toContain('usage.resetNow')
    expect(wrapper.text()).toContain('4d 19h')
    expect(wrapper.findAll('button')).toHaveLength(0)
    expect(setSubscriptionAccountAccess).toHaveBeenCalledWith(true)
  })
})
