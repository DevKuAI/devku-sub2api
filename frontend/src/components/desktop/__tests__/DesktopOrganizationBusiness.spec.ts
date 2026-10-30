import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'

const api = vi.hoisted(() => ({
  organizationContacts: vi.fn(),
  listRecords: vi.fn(),
  deleteOrganizationContact: vi.fn(),
}))

vi.mock('@/features/accounting/api', () => api)
vi.mock('vue-router', () => ({ useRoute: () => ({ fullPath: '/admin/desktop/organizations/org_one?tab=business' }) }))

import DesktopOrganizationBusiness from '../DesktopOrganizationBusiness.vue'

enableAutoUnmount(afterEach)

const contact = {
  id: 'contact-one',
  organization_id: 'org_one',
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

function render() {
  return mount(DesktopOrganizationBusiness, {
    attachTo: document.body,
    props: { organizationId: 'org_one' },
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
        teleport: true,
        Icon: true,
        RouterLink: {
          name: 'RouterLink',
          props: ['to'],
          template: '<a data-testid="manage-contracts"><slot /></a>',
        },
      },
    },
  })
}

describe('DesktopOrganizationBusiness', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.organizationContacts.mockResolvedValue([contact])
    api.listRecords.mockResolvedValue({
      items: [
        {
          id: 'contract-one',
          organization_id: 'org_one',
          status: 'SIGNED',
          currency: 'CNY',
          amount: '12000',
          occurred_at: '2026-10-12T00:00:00Z',
          details: {
            name: '年度服务合同',
            contract_number: 'HT-2026-001',
            contact_name: '王财务',
            effective_at: '2026-01-01',
            expires_at: '2026-12-31',
          },
          customer_visible: false,
          version: 1,
          created_at: '2026-10-12T00:00:00Z',
        },
      ],
      total: 1,
    })
    api.deleteOrganizationContact.mockResolvedValue({ id: contact.id, deleted: true })
  })
  afterEach(() => vi.restoreAllMocks())

  it('loads enterprise contacts and contracts with the accounting deep link', async () => {
    const wrapper = render()
    await flushPromises()

    expect(api.organizationContacts).toHaveBeenCalledWith('org_one')
    expect(api.listRecords).toHaveBeenCalledWith('contracts', false, {
      organization_id: 'org_one',
      page_size: 100,
    })
    expect(wrapper.text()).toContain('王财务')
    expect(wrapper.text()).toContain('主要联系人')
    expect(wrapper.text()).toContain('年度服务合同')
    expect(wrapper.text()).toContain('HT-2026-001')
    expect(wrapper.text()).not.toContain('企业收款账户')
    // Contracts are listed for reference only; the books no longer have a contracts page.
    expect(wrapper.findAllComponents({ name: 'RouterLink' }).some((link) => link.text().includes('管理合同'))).toBe(false)
  })

  it('keeps contact editing on its dedicated page and delete as confirmation', async () => {
    const wrapper = render()
    await flushPromises()

    const links = wrapper.findAllComponents({ name: 'RouterLink' })
    expect(links.find((link) => link.attributes('aria-label') === '编辑 王财务')!.props('to')).toEqual({
      name: 'AdminAccountingOrganizationContactEdit',
      params: { organizationId: 'org_one', contactId: 'contact-one' },
    })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)

    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    await wrapper.get('button[aria-label="删除 王财务"]').trigger('click')
    await flushPromises()
    expect(confirm).toHaveBeenCalled()
    expect(api.deleteOrganizationContact).toHaveBeenCalledWith('contact-one')
  })

  it('opens contact and contract forms as their own pages that come back here', async () => {
    const wrapper = render()
    await flushPromises()

    const back = { return_to: '/admin/desktop/organizations/org_one?tab=business' }
    const links = wrapper.findAllComponents({ name: 'RouterLink' })
    expect(links.find((link) => link.text().includes('添加联系人'))!.props('to')).toEqual({
      name: 'AdminAccountingOrganizationContactNew',
      params: { organizationId: 'org_one' },
      query: back,
    })
    expect(links.find((link) => link.text().includes('添加合同'))!.props('to')).toEqual({
      name: 'AdminAccountingContractNew',
      params: { organizationId: 'org_one' },
      query: back,
    })
    expect(links.find((link) => link.attributes('aria-label') === '编辑 年度服务合同')!.props('to')).toEqual({
      name: 'AdminAccountingContractEdit',
      params: { organizationId: 'org_one', contractId: 'contract-one' },
      query: back,
    })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })
})
