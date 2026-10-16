import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'

const api = vi.hoisted(() => ({
  organizations: vi.fn(),
  organizationContacts: vi.fn(),
  createOrganizationContact: vi.fn(),
  updateOrganizationContact: vi.fn(),
}))
const route = vi.hoisted(() => ({
  name: 'AdminAccountingOrganizationContactNew',
  params: { organizationId: 'org-one' } as Record<string, string>,
  query: { return_to: '/admin/accounting/organizations/org-one/transfers/new' } as Record<string, string>,
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

import OrganizationContactFormView from '../OrganizationContactFormView.vue'

enableAutoUnmount(afterEach)

function render() {
  return mount(OrganizationContactFormView, {
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

const contact = {
  id: 'contact-one',
  organization_id: 'org-one',
  name: '王财务',
  role: 'FINANCE' as const,
  phone: '13800138000',
  email: 'finance@example.test',
  wechat: 'finance-wechat',
  remark: '负责回款',
  is_primary: true,
  created_at: '2026-10-12T00:00:00Z',
  updated_at: '2026-10-12T00:00:00Z',
}

describe('organization contact form page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.name = 'AdminAccountingOrganizationContactNew'
    route.params = { organizationId: 'org-one' }
    route.query = { return_to: '/admin/accounting/organizations/org-one/transfers/new' }
    api.organizations.mockResolvedValue([
      { id: 'org-one', name: '上海星海智能科技有限公司', summary: [] },
    ])
    api.organizationContacts.mockResolvedValue([contact])
    api.createOrganizationContact.mockResolvedValue(contact)
    api.updateOrganizationContact.mockResolvedValue(contact)
  })

  it('creates a contact on a dedicated page and returns to the receipt form', async () => {
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('[data-testid="accounting-form-page"]').exists()).toBe(true)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('上海星海智能科技有限公司')
    await wrapper.get('input[required]').setValue(' 新联系人 ')
    await wrapper.get('select').setValue('LEGAL')
    await wrapper.get('input[type="email"]').setValue('legal@example.test')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.createOrganizationContact).toHaveBeenCalledWith(expect.objectContaining({
      organization_id: 'org-one',
      name: '新联系人',
      role: 'LEGAL',
      email: 'legal@example.test',
      is_primary: true,
    }))
    expect(router.push).toHaveBeenCalledWith('/admin/accounting/organizations/org-one/transfers/new')
  })

  it('loads and updates a contact before returning to business records', async () => {
    route.name = 'AdminAccountingOrganizationContactEdit'
    route.params = { organizationId: 'org-one', contactId: 'contact-one' }
    route.query = {}
    const wrapper = render()
    await flushPromises()

    expect((wrapper.get('input[required]').element as HTMLInputElement).value).toBe('王财务')
    await wrapper.get('input[required]').setValue('王经理')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.updateOrganizationContact).toHaveBeenCalledWith(
      'contact-one',
      expect.objectContaining({ name: '王经理', organization_id: 'org-one' }),
    )
    expect(router.push).toHaveBeenCalledWith({
      name: 'AdminDesktopOrganizationDetail',
      params: { organizationId: 'org-one' },
      query: { tab: 'business' },
    })
  })
})
