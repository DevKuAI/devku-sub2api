import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import type { MessageContext } from 'vue-i18n'

const subscriptionAPI = vi.hoisted(() => ({
  list: vi.fn(), getUsage: vi.fn(), refreshUsage: vi.fn(), refreshQuota: vi.fn(), resetQuota: vi.fn(), getTiboResetMonitor: vi.fn(),
}))
const { list, getUsage, refreshUsage, refreshQuota, resetQuota } = subscriptionAPI
const adminQuotaAPI = vi.hoisted(() => ({ refreshOpenAIQuota: vi.fn(), resetOpenAIQuota: vi.fn() }))
const setSubscriptionAccountAccess = vi.hoisted(() => vi.fn())
const appStore = vi.hoisted(() => ({ showError: vi.fn() }))

vi.mock('@/api/subscriptionAccounts', () => ({ default: subscriptionAPI }))
vi.mock('@/api/admin/accounts', () => ({ ...adminQuotaAPI, default: adminQuotaAPI }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/composables/useSubscriptionAccountAccess', () => ({
  useSubscriptionAccountAccess: () => ({ setSubscriptionAccountAccess }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key} ${Object.values(params).join(' ')}` : key }),
}))

import SubscriptionAccountsView from '../SubscriptionAccountsView.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'
import { i18n } from '@/i18n'
import type { SubscriptionAccount, SubscriptionAccountUsage } from '@/types'

enableAutoUnmount(afterEach)

function mountView() {
  return mount(SubscriptionAccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        PlatformIcon: true,
        Icon: true,
      },
    },
  })
}

describe('SubscriptionAccountsView', () => {
  beforeAll(() => {
    i18n.global.locale.value = 'en'
    i18n.global.setLocaleMessage('en', {
      common: { time: { countdown: {
        daysHours: ({ named }: MessageContext) => `${named('d')}d ${named('h')}h`,
        hoursMinutes: ({ named }: MessageContext) => `${named('h')}h ${named('m')}m`,
        minutes: ({ named }: MessageContext) => `${named('m')}m`,
        withSuffix: ({ named }: MessageContext) => `${named('time')} remaining`,
      } } },
    })
  })

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-04T00:00:00Z'))
    vi.resetAllMocks()
    subscriptionAPI.getTiboResetMonitor.mockResolvedValue({
      schemaVersion: 1,
      timezone: 'Asia/Shanghai',
      checkedAt: '2026-09-04T00:00:00Z',
      historyFrom: '2026-06-12T00:00:00Z',
      count: 1,
      events: [{
        id: 'reset-1', type: 'direct_reset', label: '全员重置', status: 'confirmed', title: 'Codex 额度重置已完成',
        scope: '所有付费订阅', createdAt: '2026-09-03T00:00:00Z', updatedAt: '2026-09-03T00:00:00Z',
        confirmedAt: '2026-09-03T00:00:00Z', occurredOn: '2026-09-03', confirmationBasis: 'source_post',
        schedule: null, posts: [{ id: 'post-1', publishedAt: '2026-09-03T00:00:00Z', stage: '确认完成', text: '完成', url: 'https://x.com/post-1' }],
        url: 'https://aihot.news/codex-reset',
      }, {
        id: 'credit-1', type: 'reset_credit', label: '发重置卡', status: 'confirmed', title: '重置卡已发放',
        scope: '', createdAt: '2026-09-04T00:00:00Z', updatedAt: '2026-09-14T00:00:00Z',
        confirmedAt: null, occurredOn: '2026-09-13', confirmationBasis: 'receipt_review', schedule: null, posts: [],
        url: 'https://aihot.news/codex-reset',
      }],
    })
    list.mockResolvedValue([
      {
        id: 8,
        name: 'Read-only subscription',
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        plan_type: 'pro',
        privacy_mode: 'training_off',
        subscription_expires_at: '2026-09-13T00:00:00Z',
        openai_compact_state: 'auto',
        current_concurrency: 1,
        reset_credits: {
          available_count: 2,
          credits: [{ expires_at: '2099-10-04T01:56:00Z' }, { expires_at: '2099-10-05T01:56:00Z' }],
        },
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
        current_concurrency: 0,
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

  it('shows an active rate limit instead of normal account status', async () => {
    const accounts = await list()
    accounts[0].rate_limit_reset_at = '2026-09-06T00:00:00Z'
    list.mockResolvedValue(accounts)

    const wrapper = mountView()
    await flushPromises()

    const header = wrapper.find('article').find('h2').element.parentElement!
    expect(header.textContent).toContain('admin.accounts.status.rateLimited')
    expect(header.textContent).toContain('2d 0h')
    expect(header.textContent).toContain('429')
    expect(header.textContent).not.toContain('subscriptionAccounts.status.active')
    expect(header.textContent).not.toContain('admin.accounts.status.active')
  })

  it.each<[Partial<SubscriptionAccount>, string]>([
    [{ schedulable: true }, 'active'],
    [{ schedulable: false }, 'paused'],
    [{ status: 'inactive' }, 'inactive'],
    [{ status: 'error' }, 'error'],
    [{ overload_until: '2026-09-06T00:00:00Z' }, 'overloaded'],
    [{ temp_unschedulable_until: '2026-09-06T00:00:00Z' }, 'tempUnschedulable'],
    [{ quota_daily_limit: 10, quota_daily_used: 10 }, 'quotaExceeded'],
    [{ rate_limit_reset_at: '2026-09-03T00:00:00Z', schedulable: true }, 'active'],
  ])('uses the admin status rules for %j', async (status, expected) => {
    const accounts = await list()
    Object.assign(accounts[0], status)
    list.mockResolvedValue(accounts)
    const wrapper = mountView()
    await flushPromises()

    const header = wrapper.find('article').find('h2').element.parentElement!
    expect(header.textContent).toContain(`admin.accounts.status.${expected}`)
    expect(header.querySelector('button')).toBeNull()
    if (expected === 'overloaded') expect(header.textContent).toContain('529')
    if (expected === 'tempUnschedulable') expect(header.textContent).toContain('admin.accounts.status.tempUnschedulableUntil')
  })

  it('shows model limits from the public status fields', async () => {
    const accounts = await list()
    Object.assign(accounts[0], {
      model_rate_limits: { 'claude-sonnet-5': { rate_limit_reset_at: '2026-09-06T00:00:00Z' } },
      allow_overages: true,
    })
    list.mockResolvedValue(accounts)
    const wrapper = mountView()
    await flushPromises()

    const header = wrapper.find('article').find('h2').element.parentElement!
    expect(header.textContent).toContain('CSon5')
    expect(header.textContent).toContain('⚡')
  })

  it('shows subscription details and quota actions for supported accounts', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(list).toHaveBeenCalledWith(true)
    expect(wrapper.findAll('article')).toHaveLength(2)
    expect(wrapper.text()).toContain('Read-only subscription')
    expect(wrapper.text()).toContain('Second subscription')
    expect(wrapper.text()).toContain('Pro')
    expect(wrapper.text()).toContain('Private')
    expect(wrapper.text()).toContain('admin.accounts.subscriptionExpires 2026-09-13')
    expect(wrapper.text()).toContain('subscriptionAccounts.noExpiration')
    expect(wrapper.text()).toContain('common.time.never')
    expect(wrapper.text()).toContain('admin.accounts.openai.compactAuto')
    const capacities = wrapper.findAll('dd[title="subscriptionAccounts.currentConcurrency"]')
    expect(capacities.map((capacity) => capacity.text())).toEqual(['1', '0'])
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
    expect(wrapper.findAll('[data-test="estimated-total-cost"]')).toHaveLength(1)
    expect(wrapper.find('[data-test="estimated-total-cost"]').text()).toContain('admin.accounts.usageWindow.estimatedTotalCost')
    const usageBars = wrapper.findAllComponents(UsageProgressBar)
    expect(usageBars[1].props('estimatedTotalCost')).toBeCloseTo(2501)
    const articles = wrapper.findAll('article')
    expect(articles[0].findAll('button')).toHaveLength(4)
    expect(articles[1].findAll('button')).toHaveLength(0)
    expect(articles[0].text()).toContain('admin.accounts.usageWindow.activeQuery')
    expect(articles[0].text()).toContain('admin.accounts.openaiQuotaReset.expiresAt')
    expect(articles[0].text()).toContain('+1')
    expect(refreshQuota).not.toHaveBeenCalled()
    expect(adminQuotaAPI.refreshOpenAIQuota).not.toHaveBeenCalled()
    expect(setSubscriptionAccountAccess).toHaveBeenCalledWith(true)
    expect(subscriptionAPI.getTiboResetMonitor).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="tibo-reset-monitor"]').text()).toContain('Codex 额度重置已完成')
    expect(wrapper.find('[data-testid="tibo-reset-monitor"]').text()).toContain('重置卡已发放')
    expect(wrapper.find('[data-testid="tibo-reset-monitor"]').text()).toContain('subscriptionAccounts.tiboReset.directReset')
    expect(wrapper.find('[data-testid="tibo-reset-monitor"]').text()).toContain('subscriptionAccounts.tiboReset.resetCard')
  })

  it('refreshes usage and disables related actions while the query is pending', async () => {
    let finishQuery!: (usage: SubscriptionAccountUsage) => void
    refreshUsage.mockReturnValue(new Promise<SubscriptionAccountUsage>((resolve) => { finishQuery = resolve }))
    const wrapper = mountView()
    await flushPromises()
    const buttons = wrapper.find('article').findAll('button')

    await buttons[0].trigger('click')
    expect(refreshUsage).toHaveBeenCalledWith(8)
    for (const button of buttons.slice(0, 3)) expect(button.attributes('disabled')).toBeDefined()
    finishQuery({ five_hour: { utilization: 42, resets_at: null, remaining_seconds: 0 } })
    await flushPromises()

    expect(wrapper.find('article').text()).toContain('42%')
    expect(buttons[0].attributes('disabled')).toBeUndefined()
    expect(adminQuotaAPI.refreshOpenAIQuota).not.toHaveBeenCalled()
  })

  it('refreshes status after an active query without losing the new usage', async () => {
    const accounts = await list()
    const wrapper = mountView()
    await flushPromises()
    list.mockResolvedValue(accounts.map((account: SubscriptionAccount) => ({
      ...account, usage: undefined, rate_limit_reset_at: '2026-09-06T00:00:00Z',
    })))
    refreshUsage.mockResolvedValue({ five_hour: { utilization: 100, resets_at: null } })

    await wrapper.find('article').find('button').trigger('click')
    await flushPromises()

    expect(list).toHaveBeenLastCalledWith()
    expect(wrapper.find('article').text()).toContain('100%')
    expect(wrapper.find('article').text()).toContain('admin.accounts.status.rateLimited')
  })

  it('preserves usage when the status refresh fails and reports the failure', async () => {
    const wrapper = mountView()
    await flushPromises()
    list.mockRejectedValue(new Error('Status read failed'))
    refreshUsage.mockResolvedValue({ five_hour: { utilization: 42, resets_at: null } })

    await wrapper.find('article').find('button').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('article')).toHaveLength(2)
    expect(wrapper.find('article').text()).toContain('42%')
    expect(appStore.showError).toHaveBeenCalledWith('subscriptionAccounts.failedToLoad')
  })

  it('queries reset credits through the user API and expands expiration details', async () => {
    refreshQuota.mockResolvedValue({
      fetched_at: 123, cache_persisted: true,
      rate_limit_reset_credits: {
        available_count: 3,
        credits: [
          { expires_at: '2099-10-04T01:56:00Z' },
          { expires_at: '2099-10-05T01:56:00Z' },
          { expires_at: '2099-10-06T01:56:00Z' },
        ],
      },
    })
    const wrapper = mountView()
    await flushPromises()
    const article = wrapper.find('article')
    await article.findAll('button')[1].trigger('click')
    await flushPromises()

    expect(refreshQuota).toHaveBeenCalledWith(8)
    expect(adminQuotaAPI.refreshOpenAIQuota).not.toHaveBeenCalled()
    expect(article.findAll('button')[1].text()).toMatch(/count\s*3/)
    const toggle = article.find('[data-testid="reset-credit-expiry-toggle"]')
    expect(toggle.text()).toBe('+2')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(article.findAll('[data-testid="reset-credit-expiry-details"] span.truncate')).toHaveLength(3)
  })

  it('requires confirmation before spending a credit and refreshes usage after success', async () => {
    const accounts = await list()
    accounts[0].rate_limit_reset_at = '2026-09-06T00:00:00Z'
    list.mockResolvedValue(accounts)
    resetQuota.mockResolvedValue({
      code: 'success', windows_reset: 1, cache_refreshed: true, account_state_recovered: true,
      quota: { fetched_at: 123, rate_limit_reset_credits: { available_count: 1, credits: [{ expires_at: '2099-10-05T01:56:00Z' }] } },
    })
    getUsage.mockResolvedValue({ five_hour: { utilization: 0, resets_at: null } })
    const wrapper = mountView()
    await flushPromises()
    const article = wrapper.find('article')
    expect(article.text()).toContain('admin.accounts.status.rateLimited')
    const resetButton = article.findAll('button')[2]
    await resetButton.trigger('click')
    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('show')).toBe(true)
    expect(resetQuota).not.toHaveBeenCalled()
    dialog.vm.$emit('cancel')
    await flushPromises()
    expect(resetQuota).not.toHaveBeenCalled()

    await resetButton.trigger('click')
    list.mockResolvedValue(accounts.map((account: SubscriptionAccount) => ({
      ...account, usage: undefined, rate_limit_reset_at: null,
    })))
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(resetQuota).toHaveBeenCalledTimes(1)
    expect(resetQuota).toHaveBeenCalledWith(8)
    expect(adminQuotaAPI.resetOpenAIQuota).not.toHaveBeenCalled()
    expect(getUsage).toHaveBeenCalledWith(8)
    expect(article.findAll('button')[1].text()).toMatch(/count\s*1/)
    expect(article.find('[data-testid="reset-credit-expiry-toggle"]').exists()).toBe(false)
    expect(article.text()).toContain('admin.accounts.openaiQuotaReset.resetSuccess')
    expect(article.text()).toContain('admin.accounts.status.active')
    expect(article.text()).not.toContain('admin.accounts.status.rateLimited')
  })

  it('keeps reset success distinct from a failed cache or usage refresh', async () => {
    const accounts = await list()
    accounts[0].rate_limit_reset_at = '2026-09-06T00:00:00Z'
    list.mockResolvedValue(accounts)
    resetQuota.mockResolvedValue({
      code: 'success', windows_reset: 1, cache_refreshed: false, account_state_recovered: true,
      warning_code: 'reset_credit_cache_refresh_failed',
    })
    getUsage.mockRejectedValue(new Error('Usage read failed'))
    const wrapper = mountView()
    await flushPromises()
    const article = wrapper.find('article')
    list.mockResolvedValue(accounts.map((account: SubscriptionAccount) => ({
      ...account, usage: undefined, rate_limit_reset_at: null,
    })))
    await article.findAll('button')[2].trigger('click')
    wrapper.findComponent(ConfirmDialog).vm.$emit('confirm')
    await flushPromises()

    expect(resetQuota).toHaveBeenCalledTimes(1)
    expect(article.findAll('button')[2].attributes('disabled')).toBeDefined()
    expect(article.findAll('button')[1].text()).not.toMatch(/\d/)
    expect(article.text()).toContain('admin.accounts.openaiQuotaReset.resetCacheRefreshFailed')
    expect(article.text()).toContain('subscriptionAccounts.failedToRefreshUsage')
    expect(article.text()).not.toContain('admin.accounts.status.rateLimited')
  })

  it.each([
    ['active', 'admin.accounts.openai.compactSupported'],
    ['blocked', 'admin.accounts.openai.compactUnsupported'],
    [undefined, undefined],
  ])('shows compact state %s and unavailable capacity', async (state, label) => {
    list.mockResolvedValue([{
      id: 10,
      name: 'Subscription with unavailable capacity',
      platform: 'openai',
      type: 'oauth',
      status: 'active',
      openai_compact_state: state,
      current_concurrency: null,
      last_used_at: null,
      expires_at: null,
      created_at: '2026-09-04T01:02:03Z',
    }])
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('dd[title="subscriptionAccounts.currentConcurrency"]').text()).toBe('-')
    if (label) {
      expect(wrapper.text()).toContain(label)
    } else {
      expect(wrapper.text()).not.toContain('admin.accounts.openai.compact')
    }
    expect(wrapper.text()).not.toContain('admin.accounts.subscriptionExpires')
    expect(wrapper.text()).not.toContain('Private')
    expect(wrapper.text()).not.toContain('Pro')
  })
})
