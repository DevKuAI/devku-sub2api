import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import en from '@/i18n/locales/en'

const desktopAPI = vi.hoisted(() => ({
  listOrganizations: vi.fn(),
  createOrganization: vi.fn(),
  getOrganization: vi.fn(),
  getGatewayUser: vi.fn(),
  listMembers: vi.fn(),
  listAvailableGatewayUsers: vi.fn(),
  listActiveGroups: vi.fn(),
  updateOrganization: vi.fn(),
}))
const appStore = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { desktop: desktopAPI } }))
vi.mock('@/api/desktopOrganization', () => ({ default: {} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: true }) }))
vi.mock('vue-router', () => ({
  useRouter: () => router,
  useRoute: () => ({ params: { organizationId: 'org_one' }, query: {} }),
}))
vi.mock('@/composables/usePersistedPageSize', () => ({ getPersistedPageSize: () => 20 }))

import DesktopOrganizationsView from './DesktopOrganizationsView.vue'
import DesktopOrganizationDetailView from './DesktopOrganizationDetailView.vue'

enableAutoUnmount(afterEach)

const organization = {
  public_id: 'org_one', code: 'desktop', name: 'Desktop', status: 'active',
  gateway_user: { id: 42, email: 'carrier@example.test' },
  group: { id: 7, name: 'Exclusive' }, member_count: 0, member_limit: 10,
  target_config_assigned: false, target_config: null,
  created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z',
}

beforeEach(() => {
  vi.resetAllMocks()
  const emptyPage = { items: [], total: 0, page: 1, page_size: 20, pages: 0 }
  desktopAPI.listOrganizations.mockResolvedValue(emptyPage)
  desktopAPI.getOrganization.mockResolvedValue(organization)
  desktopAPI.getGatewayUser.mockResolvedValue(organization.gateway_user)
  desktopAPI.listMembers.mockResolvedValue(emptyPage)
  desktopAPI.listAvailableGatewayUsers.mockResolvedValue({ ...emptyPage, items: [organization.gateway_user] })
  desktopAPI.listActiveGroups.mockResolvedValue([{ id: 7, name: 'Exclusive', is_exclusive: true }])
})

async function submitOrganization(operation: 'create' | 'edit', locale: 'zh' | 'en', error: unknown) {
  const api = operation === 'create' ? desktopAPI.createOrganization : desktopAPI.updateOrganization
  api.mockRejectedValueOnce(error)
  const options = {
    global: {
      plugins: [createI18n({
        legacy: false, locale, messages: { zh, en },
        // Error messages are plain text; the Vitest alias excludes the message compiler.
        messageCompiler: (message) => () => String(message),
      })],
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
        DataTable: true, Pagination: true, BaseDialog: true, ConfirmDialog: true,
        EmptyState: true, StatusBadge: true, Select: true, Input: true, Icon: true, RouterLink: true,
      },
    },
  }
  const wrapper = operation === 'create'
    ? mount(DesktopOrganizationsView, options)
    : mount(DesktopOrganizationDetailView, options)
  await flushPromises()
  const vm = wrapper.vm as any
  if (operation === 'create') {
    await vm.openCreate()
    Object.assign(vm.form, { name: 'Desktop', code: 'desktop', gateway_user_id: 42, group_id: 7 })
    await vm.createOrganization()
  } else {
    await vm.openEditOrganization()
    await vm.saveOrganization()
  }
  expect(api).toHaveBeenCalledTimes(1)
}

describe.each(['create', 'edit'] as const)('Desktop organization %s errors', (operation) => {
  it.each(['zh', 'en'] as const)('translates group access errors in %s', async (locale) => {
    await submitOrganization(operation, locale, { reason: 'GROUP_NOT_ALLOWED', message: 'user is not allowed to bind this group' })

    expect(appStore.showError).toHaveBeenCalledWith(locale === 'zh'
      ? '承载用户无法使用所选分组，请检查分组状态、访问权限和有效订阅。'
      : 'The gateway user cannot use this group. Check the group status, access permissions, and active subscription.')
  })

  it('uses the server message for an untranslated error reason', async () => {
    await submitOrganization(operation, 'zh', { reason: 'UNRECOGNIZED_DESKTOP_ERROR', message: 'Service unavailable' })

    expect(appStore.showError).toHaveBeenCalledWith('Service unavailable')
  })

  it('uses a localized fallback when an untranslated error has no message', async () => {
    await submitOrganization(operation, 'zh', { reason: 'UNRECOGNIZED_DESKTOP_ERROR' })

    expect(appStore.showError).toHaveBeenCalledWith(zh.admin.desktop.errors.UNKNOWN)
  })
})

describe('Desktop organization member limit on creation', () => {
  it.each([10, 25])('submits member limit %s and resets the next form to 10', async (limit) => {
    desktopAPI.createOrganization.mockResolvedValue(organization)
    const wrapper = mount(DesktopOrganizationsView, {
      global: {
        plugins: [createI18n({ legacy: false, locale: 'en', messages: { en }, messageCompiler: (message) => () => String(message) })],
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          TablePageLayout: true, BaseDialog: true,
        },
      },
    })
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openCreate()
    expect(vm.form.member_limit).toBe(10)
    Object.assign(vm.form, { name: 'Desktop', code: 'desktop', gateway_user_id: 42, group_id: 7, member_limit: limit, conversation_reporting_enabled: false })
    await vm.createOrganization()

    expect(desktopAPI.createOrganization).toHaveBeenCalledWith({ name: 'Desktop', code: 'desktop', gateway_user_id: 42, group_id: 7, member_limit: limit, conversation_reporting_enabled: false })
    await vm.openCreate()
    expect(vm.form.member_limit).toBe(10)
  })
})

describe('Desktop conversation reporting on creation', () => {
  it.each([false, true])('creates an organization with reporting=%s and resets the next form', async (enabled) => {
    desktopAPI.createOrganization.mockResolvedValue({ ...organization, conversation_reporting_enabled: enabled })
    const wrapper = mount(DesktopOrganizationsView, {
      global: {
        plugins: [createI18n({ legacy: false, locale: 'en', messages: { en }, messageCompiler: (message) => () => String(message) })],
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          TablePageLayout: true,
          BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /></section>' },
        },
      },
    })
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openCreate()
    await flushPromises()
    const checkbox = wrapper.get('[data-testid="desktop-create-conversation-reporting"]')
    expect((checkbox.element as HTMLInputElement).checked).toBe(false)
    await checkbox.setValue(enabled)
    Object.assign(vm.form, { name: 'Desktop', code: 'desktop', gateway_user_id: 42, group_id: 7 })
    await wrapper.get('#desktop-organization-create').trigger('submit')
    await flushPromises()
    expect(desktopAPI.createOrganization).toHaveBeenCalledWith(expect.objectContaining({ conversation_reporting_enabled: enabled }))
    await vm.openCreate()
    await flushPromises()
    expect((wrapper.get('[data-testid="desktop-create-conversation-reporting"]').element as HTMLInputElement).checked).toBe(false)
  })
})
