import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'

const api = vi.hoisted(() => ({
  organizationContacts: vi.fn(),
  listRecords: vi.fn(),
  deleteOrganizationContact: vi.fn(),
  createOrganizationContact: vi.fn(),
}))

vi.mock('@/features/accounting/api', () => api)

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
    api.createOrganizationContact.mockResolvedValue(contact)
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
    expect(links.some((link) => link.text().includes('添加联系人'))).toBe(false)
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

  it('creates a contact in a dialog and refreshes the list without navigating', async () => {
    const wrapper = render(); await flushPromises()
    const add = wrapper.findAll('button').find(button => button.text() === '添加联系人')!
    await add.trigger('click'); await flushPromises()
    const dialog = wrapper.get('[role="dialog"]')
    await dialog.get('input[required]').setValue(' 新联系人 ')
    await dialog.get('select').setValue('LEGAL')
    await dialog.get('input[type="tel"]').setValue(' 13800138001 ')
    await dialog.get('input[type="email"]').setValue('legal@example.test')
    await dialog.findAll('input[autocomplete="off"]')[1].setValue(' legal-wechat ')
    await dialog.get('textarea').setValue(' 合同对接 ')
    await dialog.get('input[type="checkbox"]').setValue(true)
    api.organizationContacts.mockResolvedValueOnce([contact, { ...contact, id: 'new-contact', name: '新联系人' }])
    await dialog.get('form').trigger('submit'); await flushPromises()
    expect(api.createOrganizationContact).toHaveBeenCalledWith({
      organization_id: 'org_one', name: '新联系人', role: 'LEGAL', phone: '13800138001',
      email: 'legal@example.test', wechat: 'legal-wechat', remark: '合同对接', is_primary: true,
    })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('新联系人')
    expect(api.organizationContacts).toHaveBeenCalledTimes(2)
    expect(document.activeElement).toBe(add.element)
  })

  it('validates required names and email addresses before sending a request', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '添加联系人')!.trigger('click')
    const form = wrapper.get('form')
    await form.trigger('submit')
    await form.get('input[required]').setValue('   ')
    await form.trigger('submit')
    await form.get('input[required]').setValue('测试')
    await form.get('input[type="email"]').setValue('invalid-email')
    await form.trigger('submit'); await flushPromises()
    expect(api.createOrganizationContact).not.toHaveBeenCalled()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
  })

  it('retains entered fields after a save failure and allows retrying', async () => {
    api.createOrganizationContact.mockRejectedValueOnce({ message: '保存失败，请重试' })
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '添加联系人')!.trigger('click')
    await wrapper.get('input[required]').setValue('待保存联系人')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="alert"]').text()).toBe('保存失败，请重试')
    expect((wrapper.get('input[required]').element as HTMLInputElement).value).toBe('待保存联系人')
    expect(api.organizationContacts).toHaveBeenCalledTimes(1)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.createOrganizationContact).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('confirms discarding unsaved changes and resets fields when reopened', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const wrapper = render(); await flushPromises()
    const add = wrapper.findAll('button').find(button => button.text() === '添加联系人')!
    await add.trigger('click')
    await wrapper.get('input[required]').setValue('未保存')
    await wrapper.get('[role="dialog"] button[type="button"]').trigger('click')
    expect(confirm).toHaveBeenCalled()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    confirm.mockReturnValue(true)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(add.element)
    await add.trigger('click')
    expect((wrapper.get('input[required]').element as HTMLInputElement).value).toBe('')
    expect(api.createOrganizationContact).not.toHaveBeenCalled()
  })

  it('prevents duplicate saves and closing while the request is pending', async () => {
    let finish!: (value: typeof contact) => void
    api.createOrganizationContact.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '添加联系人')!.trigger('click')
    await wrapper.get('input[required]').setValue('保存中')
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(api.createOrganizationContact).toHaveBeenCalledTimes(1)
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="dialog"] button[type="button"]').attributes('disabled')).toBeDefined()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    finish(contact); await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('closes the creation dialog when switching organizations', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '添加联系人')!.trigger('click')
    await wrapper.setProps({ organizationId: 'org_two' }); await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(api.organizationContacts).toHaveBeenLastCalledWith('org_two')
  })
})
