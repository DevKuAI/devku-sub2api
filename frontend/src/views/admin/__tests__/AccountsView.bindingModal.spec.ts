import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const { listAccounts } = vi.hoisted(() => ({
  listAccounts: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag: vi.fn(),
      getBatchTodayStats: vi.fn().mockResolvedValue({ stats: {} }),
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: { getAll: vi.fn().mockResolvedValue([]) },
    groups: { getAll: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.id" data-test="account-row">
        <slot name="cell-bound_user" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
}

const AccountBindingModalStub = {
  props: ['show', 'account'],
  emits: ['close', 'updated'],
  template: `
    <div v-if="show" data-test="binding-modal" :data-account-id="account.id">
      <button
        data-test="complete-binding"
        @click="$emit('updated', {
          ...account,
          bound_user_id: 42,
          bound_user: { id: 42, username: 'Alice', email: 'alice@example.com' }
        })"
      />
    </div>
  `
}

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        AccountBindingModal: AccountBindingModalStub,
        AccountTableActions: { template: '<div><slot name="after" /></div>' },
        AccountTableFilters: true,
        AccountBulkActionsBar: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        HelpTooltip: true,
        Icon: true,
        Teleport: true
      }
    }
  })
}

describe('admin AccountsView binding modal', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset().mockResolvedValue({
      items: [{
        id: 7,
        name: 'Team subscription',
        platform: 'openai',
        type: 'apikey',
        status: 'active',
        schedulable: true,
        priority: 0,
        rate_multiplier: 1,
        bound_user_id: null,
        created_at: '2026-09-04T01:02:03Z',
        updated_at: '2026-09-04T01:02:03Z'
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
  })

  it('opens from the row action and updates the bound user without navigation', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[aria-label="admin.accounts.binding.action"]').trigger('click')

    expect(wrapper.get('[data-test="binding-modal"]').attributes('data-account-id')).toBe('7')

    await wrapper.get('[data-test="complete-binding"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="binding-modal"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="account-row"]').text()).toContain('Alice')
    expect(wrapper.get('[data-test="account-row"]').text()).toContain('alice@example.com')
  })
})
