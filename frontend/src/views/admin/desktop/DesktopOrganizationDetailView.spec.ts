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

const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn() }))
const route = vi.hoisted(() => ({
  params: { organizationId: 'org_one' },
  query: { tab: 'members' },
}))
const appStore = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { desktop: desktopAPI } }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
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
  created_at: '2026-08-26T00:00:00Z',
  updated_at: '2026-08-26T00:00:00Z',
}

function mountView() {
  return mount(DesktopOrganizationDetailView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        DataTable: true,
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

	it('prefills the full phone in the real edit input', async () => {
		const wrapper = mountViewWithRealMemberForm()
		await flushPromises()

		;(wrapper.vm as any).openEditMember(member)
		await flushPromises()

		expect(wrapper.get<HTMLInputElement>('input[type="tel"]').element.value).toBe('+8613800000000')
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

	it('submits the target-specific Gateway protocol for each client', async () => {
		const wrapper = mountView()
		await flushPromises()
		const vm = wrapper.vm as any

		Object.assign(vm.configForm.chat, {
			provider_id: 'provider', display_name: 'Codex', requested_model: 'model-one',
		})
		Object.assign(vm.configForm.work, {
			provider_id: 'provider', display_name: 'Workbuddy', requested_model: 'model-two',
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
					enabled: false, provider_id: 'provider', display_name: 'Workbuddy', requested_model: 'model-two',
					wire_api: 'chat_completions', restart_required: false,
				},
			},
		})
		wrapper.unmount()
	})
})
