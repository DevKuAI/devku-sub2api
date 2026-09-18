import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const desktopAPI = vi.hoisted(() => ({
  getOrganization: vi.fn(),
  getGatewayUser: vi.fn(),
  listMembers: vi.fn(),
  listAvailableGatewayUsers: vi.fn(),
  listActiveGroups: vi.fn(),
  updateOrganization: vi.fn(),
  updateMember: vi.fn(),
  createMember: vi.fn(),
  deleteMember: vi.fn(),
  rotateModelToken: vi.fn(),
  updateModelConfiguration: vi.fn(),
}))
const managedDesktopAPI = vi.hoisted(() => ({
  getOrganization: vi.fn(),
  listMembers: vi.fn(),
  updateOrganization: vi.fn(),
  updateMember: vi.fn(),
  createMember: vi.fn(),
  deleteMember: vi.fn(),
  rotateModelToken: vi.fn(),
  updateModelConfiguration: vi.fn(),
}))

const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn() }))
const route = vi.hoisted(() => ({
  params: { organizationId: 'org_one' },
  query: { tab: 'members' },
}))
const appStore = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
const authStore = vi.hoisted(() => ({ isAdmin: false }))

vi.mock('@/api/admin', () => ({ adminAPI: { desktop: desktopAPI } }))
vi.mock('@/api/desktopOrganization', () => ({ default: managedDesktopAPI }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, te: () => true }),
  }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/composables/usePersistedPageSize', () => ({ getPersistedPageSize: () => 20 }))

import DesktopOrganizationDetailView from './DesktopOrganizationDetailView.vue'

const organization = {
  public_id: 'org_one',
  code: 'desktop',
  name: 'Desktop Organization',
  status: 'active',
  gateway_user: { id: 42, email: 'carrier@example.com', username: 'carrier' },
  group: { id: 7, name: 'Responses' },
  member_count: 1,
  member_limit: 10,
  conversation_reporting_enabled: false,
  target_config_assigned: false,
  target_config: null,
  created_at: '2026-08-26T00:00:00Z',
  updated_at: '2026-08-26T00:00:00Z',
}

const member = {
	public_id: 'mem_one',
	name: 'Member',
	phone: '+8613800000000',
  status: 'active',
  model_token_status: 'active',
  usage: {
    today_tokens: 1250,
    last_30_days_tokens: 2_500_000,
    total_tokens: 4_000_000_000,
    today_actual_cost: 0.125,
    last_30_days_actual_cost: 2.5,
    total_actual_cost: 4,
  },
  created_at: '2026-08-26T00:00:00Z',
  updated_at: '2026-08-26T00:00:00Z',
}

function mountView() {
  return mount(DesktopOrganizationDetailView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        DataTable: true,
        DesktopConversationRecords: true,
        Pagination: true,
        BaseDialog: true,
        ConfirmDialog: true,
        StatusBadge: true,
        Select: true,
        Input: true,
        Icon: true,
      },
    },
  })
}

function mountManagedView() {
  return mount(DesktopOrganizationDetailView, {
    props: { selfManaged: true },
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        DataTable: true,
        DesktopConversationRecords: true,
        Pagination: true,
        BaseDialog: true,
        ConfirmDialog: true,
        StatusBadge: true,
        Select: true,
        Input: true,
        Icon: true,
      },
    },
  })
}

function mountViewWithRealMemberForm() {
	return mount(DesktopOrganizationDetailView, {
		global: {
			stubs: {
				AppLayout: { template: '<main><slot /></main>' },
				DataTable: true,
				Pagination: true,
				ConfirmDialog: true,
				StatusBadge: true,
				Select: true,
				Icon: true,
				Teleport: true,
			},
		},
	})
}

describe('DesktopOrganizationDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    desktopAPI.getOrganization.mockResolvedValue({ ...organization })
    desktopAPI.getGatewayUser.mockResolvedValue({ id: 42, email: 'carrier@example.com', username: 'carrier' })
    desktopAPI.listMembers.mockResolvedValue({ items: [{ ...member }], total: 1, page: 1, page_size: 20, pages: 1 })
	desktopAPI.listAvailableGatewayUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 30, pages: 0 })
	desktopAPI.listActiveGroups.mockResolvedValue([])
	desktopAPI.updateMember.mockResolvedValue({ ...member })
    desktopAPI.updateModelConfiguration.mockResolvedValue({ ...organization })
    managedDesktopAPI.getOrganization.mockResolvedValue({ ...organization })
    managedDesktopAPI.listMembers.mockResolvedValue({ items: [{ ...member }], total: 1, page: 1, page_size: 20, pages: 1 })
    managedDesktopAPI.updateOrganization.mockResolvedValue({ ...organization })
    authStore.isAdmin = false
    route.query.tab = 'members'
  })

  it('hydrates the assigned gateway user separately and preserves tab state in the URL', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(desktopAPI.getGatewayUser).toHaveBeenCalledWith(42)
    expect(desktopAPI.listMembers).toHaveBeenCalledWith(
      'org_one', 1, 20, { search: '', status: '' }, expect.any(AbortSignal)
    )
    ;(wrapper.vm as any).setTab('configuration')
    expect(router.replace).toHaveBeenCalledWith({ query: { tab: 'configuration' } })
    wrapper.unmount()
  })

  it('supports the ARIA tabs keyboard model', async () => {
    const wrapper = mountView()
    await flushPromises()
    const memberTab = wrapper.get('#desktop-organization-tab-members')
    const configurationTab = wrapper.get('#desktop-organization-tab-configuration')

    expect(memberTab.attributes('tabindex')).toBe('0')
    expect(configurationTab.attributes('tabindex')).toBe('-1')
    expect(memberTab.attributes('aria-controls')).toBe('desktop-organization-panel-members')
    await memberTab.trigger('keydown', { key: 'ArrowRight' })

    expect(router.replace).toHaveBeenCalledWith({ query: { tab: 'configuration' } })
    wrapper.unmount()
  })

	it('serializes destructive confirmation submissions', async () => {
    let resolveDelete!: (value: { deleted: boolean }) => void
    desktopAPI.deleteMember.mockImplementation(() => new Promise((resolve) => { resolveDelete = resolve }))
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.confirmDeleteMember(member)
    const first = vm.runConfirmedAction()
    const second = vm.runConfirmedAction()
    expect(desktopAPI.deleteMember).toHaveBeenCalledTimes(1)

    resolveDelete({ deleted: true })
    await Promise.all([first, second])
    wrapper.unmount()
	})

	it('shows the full phone and only submits it when edited', async () => {
		const wrapper = mountView()
		await flushPromises()
		const vm = wrapper.vm as any

		expect(vm.memberColumns.map((column: { key: string }) => column.key)).toContain('phone')
		expect(vm.memberColumns.map((column: { key: string }) => column.key)).not.toContain('masked_phone')
		vm.openEditMember(member)
		expect(vm.memberForm.phone).toBe('+8613800000000')

		await vm.saveMember()
		expect(desktopAPI.updateMember).toHaveBeenLastCalledWith('org_one', 'mem_one', { name: 'Member' })

		vm.memberForm.phone = '13800000000'
		await vm.saveMember()
		expect(desktopAPI.updateMember).toHaveBeenLastCalledWith('org_one', 'mem_one', {
			name: 'Member',
			phone: '13800000000',
		})
		wrapper.unmount()
	})

	it('shows member cost and token usage in separate columns', async () => {
		const wrapper = mountView()
		await flushPromises()
		const vm = wrapper.vm as any

		expect(vm.memberColumns.map((column: { key: string }) => column.key)).toContain('usage_cost')
		expect(vm.memberColumns.map((column: { key: string }) => column.key)).toContain('usage_tokens')
		expect(vm.formatUsageCost(member.usage.today_actual_cost)).toBe('$0.1250')
		expect(vm.formatUsageTokens(member.usage.today_tokens)).toBe('1.3K')
		expect(vm.formatUsageTokens(member.usage.last_30_days_tokens)).toBe('2.5M')
		expect(vm.formatUsageTokens(member.usage.total_tokens)).toBe('4.0B')
		expect(vm.formatUsageTokensFull(member.usage.today_tokens)).toContain('1,250')
		wrapper.unmount()
	})

	it('prefills the full phone in the real edit input', async () => {
		const wrapper = mountViewWithRealMemberForm()
		await flushPromises()

		;(wrapper.vm as any).openEditMember(member)
		await flushPromises()

		expect(wrapper.get<HTMLInputElement>('input[type="tel"]').element.value).toBe('+8613800000000')
		wrapper.unmount()
	})

  it.each([false, true])('shows managed model configuration as read-only with isAdmin=%s', async (isAdmin) => {
    authStore.isAdmin = isAdmin
    route.query.tab = 'configuration'
    managedDesktopAPI.getOrganization.mockResolvedValue({
      ...organization,
      target_config_assigned: true,
      target_config: {
        schema_version: 1,
        targets: {
          chatgpt_codex: {
            enabled: true, provider_id: 'openai', display_name: 'Codex', requested_model: 'codex-model',
            wire_api: 'responses', minimum_app_version: '1.2.3', restart_required: true,
          },
          workbuddy: {
            enabled: false, provider_id: 'work-provider', display_name: 'Workbuddy', requested_model: 'work-model',
            wire_api: 'chat_completions', restart_required: false,
          },
        },
      },
    })
    const wrapper = mountManagedView()
    await flushPromises()
    const panel = wrapper.get('[role="tabpanel"]')

    for (const value of ['Codex', 'openai', 'codex-model', '/responses', '1.2.3', 'Workbuddy', 'work-provider', 'work-model', '/chat/completions', 'common.enabled', 'common.disabled', 'common.yes', 'common.no']) {
      expect(panel.text()).toContain(value)
    }
    expect(panel.find('form, input, input-stub, select, textarea, button').exists()).toBe(false)
    expect(wrapper.findAll('button').some((button) => button.text() === 'common.edit')).toBe(false)
    await (wrapper.vm as any).saveConfiguration()
    expect(managedDesktopAPI.updateModelConfiguration).not.toHaveBeenCalled()
    expect(desktopAPI.updateModelConfiguration).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows unassigned managed targets without editable defaults', async () => {
    route.query.tab = 'configuration'
    const wrapper = mountManagedView()
    await flushPromises()
    const panel = wrapper.get('[role="tabpanel"]')

    expect(panel.text()).toContain('ChatGPT Codex')
    expect(panel.text()).toContain('Workbuddy')
    expect(panel.text()).toContain('admin.desktop.notConfigured')
    expect(panel.text()).not.toContain('common.enabled')
    expect(panel.find('form, input, input-stub, select, textarea, button').exists()).toBe(false)
    wrapper.unmount()
  })

  it('rejects incomplete model configuration before calling the API', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.configForm.chat.provider_id = ''
    vm.configForm.chat.display_name = 'Model'
    vm.configForm.chat.requested_model = 'model-one'
    await vm.saveConfiguration()

    expect(desktopAPI.updateModelConfiguration).not.toHaveBeenCalled()
    expect(appStore.showError).toHaveBeenCalledWith('admin.desktop.errors.VALIDATION_FAILED')
    wrapper.unmount()
  })

  it.each([false, true])('keeps managed organization details read-only with isAdmin=%s', async (isAdmin) => {
    authStore.isAdmin = isAdmin
    const wrapper = mountManagedView()
    await flushPromises()
    const vm = wrapper.vm as any

    expect(managedDesktopAPI.getOrganization).toHaveBeenCalled()
    expect(managedDesktopAPI.listMembers).toHaveBeenCalled()
    expect(desktopAPI.getGatewayUser).not.toHaveBeenCalled()
    expect(desktopAPI.listAvailableGatewayUsers).not.toHaveBeenCalled()
    expect(desktopAPI.listActiveGroups).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain(organization.name)
    expect(wrapper.findAll('button').some((button) => button.text() === 'common.edit')).toBe(false)
    expect(wrapper.find('base-dialog-stub[title="admin.desktop.editOrganization"]').exists()).toBe(false)

    await vm.openEditOrganization()
    expect(vm.showOrganizationEdit).toBe(false)
    vm.organizationForm.name = 'Managed Organization'
    vm.organizationForm.status = 'disabled'
    vm.organizationForm.member_limit = 100
    vm.submitOrganizationEdit()
    await vm.saveOrganization()

    expect(vm.confirmState.show).toBe(false)
    expect(managedDesktopAPI.updateOrganization).not.toHaveBeenCalled()
    expect(desktopAPI.updateOrganization).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows the total member capacity in the managed view', async () => {
    const wrapper = mountManagedView()
    await flushPromises()

    expect(wrapper.text()).toContain('1 / 10')
    await (wrapper.vm as any).openEditOrganization()
    expect(wrapper.find('#desktop-edit-member-limit').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each([false, true])('submits the edit checkbox value %s explicitly', async (enabled) => {
    desktopAPI.getOrganization.mockResolvedValue({ ...organization, conversation_reporting_enabled: !enabled })
    desktopAPI.updateOrganization.mockResolvedValue({ ...organization, conversation_reporting_enabled: enabled })
    const wrapper = mountViewWithRealMemberForm()
    await flushPromises()
    await (wrapper.vm as any).openEditOrganization()
    await flushPromises()
    const checkbox = wrapper.get('[data-testid="desktop-edit-conversation-reporting"]')
    expect((checkbox.element as HTMLInputElement).checked).toBe(!enabled)
    await checkbox.setValue(enabled)
    await wrapper.get('#desktop-organization-edit').trigger('submit')
    await flushPromises()
    expect(desktopAPI.updateOrganization).toHaveBeenCalledWith('org_one', expect.objectContaining({ conversation_reporting_enabled: enabled }))
    wrapper.unmount()
  })

  it.each([false, true, undefined])('gates the managed conversation tab for reporting=%s', async (enabled) => {
    managedDesktopAPI.getOrganization.mockResolvedValue({ ...organization, conversation_reporting_enabled: enabled })
    route.query.tab = 'conversations'
    const wrapper = mountManagedView()
    await flushPromises()
    const tab = wrapper.find('#desktop-organization-tab-conversations')
    expect(tab.exists()).toBe(enabled === true)
    expect(wrapper.find('desktop-conversation-records-stub').exists()).toBe(enabled === true)
    if (enabled !== true) {
      expect(router.replace).toHaveBeenCalledWith({ query: { tab: 'members' } })
      expect(wrapper.find('#desktop-organization-panel-members').exists()).toBe(true)
    }
    wrapper.unmount()
  })

  it('unmounts managed conversation data after the organization turns reporting off', async () => {
    managedDesktopAPI.getOrganization.mockResolvedValue({ ...organization, conversation_reporting_enabled: true })
    route.query.tab = 'conversations'
    const wrapper = mountManagedView()
    await flushPromises()
    expect(wrapper.find('desktop-conversation-records-stub').exists()).toBe(true)
    managedDesktopAPI.getOrganization.mockResolvedValue({ ...organization, conversation_reporting_enabled: false })
    await (wrapper.vm as any).loadOrganization()
    await flushPromises()
    expect(wrapper.find('desktop-conversation-records-stub').exists()).toBe(false)
    expect(wrapper.find('#desktop-organization-tab-conversations').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps administrator history visible when reporting is disabled', async () => {
    route.query.tab = 'conversations'
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('#desktop-organization-tab-conversations').exists()).toBe(true)
    expect(wrapper.find('desktop-conversation-records-stub').exists()).toBe(true)
    wrapper.unmount()
  })

  it('allows administrators to change the member limit', async () => {
    desktopAPI.updateOrganization.mockResolvedValue({ ...organization, member_limit: 25 })
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    const editButton = wrapper.findAll('button').find((button) => button.text() === 'common.edit')
    expect(editButton).toBeDefined()
    await editButton!.trigger('click')
    await flushPromises()
    expect(vm.showOrganizationEdit).toBe(true)
    expect(vm.organizationForm.member_limit).toBe(10)
    vm.organizationForm.member_limit = 25
    await vm.saveOrganization()

    expect(desktopAPI.updateOrganization).toHaveBeenCalledWith('org_one', {
      name: organization.name, status: 'active', member_limit: 25, conversation_reporting_enabled: false,
    })
    expect(wrapper.text()).toContain('1 / 25')
    wrapper.unmount()
  })

  it.each([0, -1, 1.5])('rejects invalid member limit %s', async (limit) => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openEditOrganization()
    vm.organizationForm.member_limit = limit
    await vm.saveOrganization()

    expect(desktopAPI.updateOrganization).not.toHaveBeenCalled()
    expect(appStore.showError).toHaveBeenCalledWith('admin.desktop.errors.VALIDATION_FAILED')
    wrapper.unmount()
  })

  it.each([false, true])('blocks member creation at capacity with selfManaged=%s', async (selfManaged) => {
    const api = selfManaged ? managedDesktopAPI : desktopAPI
    api.getOrganization.mockResolvedValue({ ...organization, member_limit: 1 })
    const wrapper = selfManaged ? mountManagedView() : mountView()
    await flushPromises()
    const button = wrapper.findAll('button').find((item) => item.text().includes('admin.desktop.createMember'))
    expect(button?.attributes('disabled')).toBeDefined()
    const vm = wrapper.vm as any
    Object.assign(vm.memberForm, { name: 'Extra', phone: '13800138001' })
    await vm.saveMember()

    expect(api.createMember).not.toHaveBeenCalled()
    expect(appStore.showError).toHaveBeenCalledWith('admin.desktop.errors.MEMBER_LIMIT_REACHED')
    wrapper.unmount()
  })

  it('redirects direct managed access when the user has no organization', async () => {
    managedDesktopAPI.getOrganization.mockResolvedValue(null)
    const wrapper = mountManagedView()
    await flushPromises()

    expect(router.replace).toHaveBeenCalledWith('/dashboard')
    expect(managedDesktopAPI.listMembers).not.toHaveBeenCalled()
    wrapper.unmount()
  })

	it('allows Workbuddy to be enabled from the configuration form', async () => {
		route.query.tab = 'configuration'
		const wrapper = mountView()
		await flushPromises()
		const disabledLabel = wrapper.findAll('label').find((label) => label.text() === 'common.disabled')

		expect(disabledLabel).toBeDefined()
		const toggle = disabledLabel!.get('input[type="checkbox"]')
		expect(toggle.attributes('disabled')).toBeUndefined()
		await toggle.setValue(true)
		expect((wrapper.vm as any).configForm.work.enabled).toBe(true)
		expect((wrapper.vm as any).configForm.includeWorkbuddy).toBe(true)
		wrapper.unmount()
	})

	it('submits the target-specific Gateway protocol and enabled state for each client', async () => {
		const wrapper = mountView()
		await flushPromises()
		const vm = wrapper.vm as any

		Object.assign(vm.configForm.chat, {
			provider_id: 'provider', display_name: 'Codex', requested_model: 'model-one',
		})
		Object.assign(vm.configForm.work, {
			enabled: true, provider_id: 'provider', display_name: 'Workbuddy', requested_model: 'model-two',
		})
		vm.configForm.includeWorkbuddy = true

		await vm.saveConfiguration()

		expect(desktopAPI.updateModelConfiguration).toHaveBeenCalledWith('org_one', {
			schema_version: 1,
			targets: {
				chatgpt_codex: {
					enabled: true, provider_id: 'provider', display_name: 'Codex', requested_model: 'model-one',
					wire_api: 'responses', restart_required: false,
				},
				workbuddy: {
					enabled: true, provider_id: 'provider', display_name: 'Workbuddy', requested_model: 'model-two',
					wire_api: 'chat_completions', restart_required: false,
				},
			},
		})
		wrapper.unmount()
	})
})
