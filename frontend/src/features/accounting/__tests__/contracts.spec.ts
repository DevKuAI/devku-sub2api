import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, record, stubs } from './testing'

const api = vi.hoisted(() => ({
  organizationContacts: vi.fn(),
  getRecord: vi.fn(),
  mutate: vi.fn(),
  upload: vi.fn(),
  organizations: vi.fn(),
  organizationContracts: vi.fn(),
  attachments: vi.fn(),
  stableRequest: () => Object.assign((path: string) => `key-${path}`, { reset: vi.fn() }),
}))
const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }))
const route = await vi.hoisted(async () => {
  const { reactive } = await import('vue')
  return reactive({ params: {} as Record<string, string>, query: {} as Record<string, string>, fullPath: '' })
})

vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRouter: () => router, useRoute: () => route, onBeforeRouteLeave: vi.fn() }))

import ContractFormView from '../ContractFormView.vue'
import OrderCreateView from '../OrderCreateView.vue'

enableAutoUnmount(afterEach)
const options = {
  global: {
    plugins: [i18n()],
    stubs: {
      ...stubs,
      FormPage: { props: ['error'], emits: ['submit'], template: '<form @submit.prevent="$emit(\'submit\')"><p v-if="error">{{ error }}</p><slot /></form>' },
      FormSection: { template: '<section><slot /></section>' },
      RecordFiles: {
        props: ['kind', 'record'],
        emits: ['uploaded'],
        template: '<div data-testid="record-files">{{ kind }}<button type="button" data-testid="scan-uploaded" @click="$emit(\'uploaded\')" /></div>',
      },
      AccountingFileUpload: {
        props: ['modelValue'],
        emits: ['update:modelValue'],
        setup(_props: unknown, { emit }: { emit: (event: 'update:modelValue', file: File) => void }) {
          return { pick: () => emit('update:modelValue', new File(['scan'], '合同.pdf', { type: 'application/pdf' })) }
        },
        template: '<button type="button" data-testid="pick-file" @click="pick" />',
      },
    },
  },
}
const contact = { id: 'contact-one', organization_id: 'org_one', name: '王财务', role: 'FINANCE', phone: '13800138000', email: '', wechat: '', remark: '', is_primary: true, created_at: '', updated_at: '' }

describe('sales contracts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    router.push.mockReset()
    route.params = { organizationId: 'org_one' }
    route.query = {}
    api.organizationContacts.mockResolvedValue([contact])
    api.mutate.mockResolvedValue(record({ id: 'contract-one', organization_id: 'org_one' }))
  })

  it('adds a signed contract with its scan and sharing, the amount being optional', async () => {
    const wrapper = mount(ContractFormView, options)
    await flushPromises()
    await wrapper.get('[data-testid="contract-name"]').setValue(' 年度服务合同 ')
    await wrapper.get('[data-testid="contract-number"]').setValue('HT-2026-001')
    await wrapper.get('[data-testid="contract-shared"]').setValue(true)
    await wrapper.get('[data-testid="pick-file"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.mutate).toHaveBeenNthCalledWith(1, 'contracts', expect.objectContaining({ organization_id: 'org_one', amount: '0', details: expect.objectContaining({ name: '年度服务合同', contract_number: 'HT-2026-001' }) }), 'key-contracts')
    expect(api.mutate).toHaveBeenNthCalledWith(2, 'contracts/contract-one/confirm', {}, 'key-contracts/contract-one/confirm')
    expect(api.mutate).toHaveBeenNthCalledWith(3, 'contracts/contract-one/sharing', { customer_visible: true }, 'key-contracts/contract-one/sharing', 'patch')
    expect(api.upload).toHaveBeenCalledWith(expect.any(File), 'contracts', expect.objectContaining({ id: 'contract-one' }))
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminDesktopOrganizationDetail', params: { organizationId: 'org_one' }, query: { tab: 'business' } })
  })

  it('moves to the edit page and offers the upload again when the scan fails', async () => {
    route.query = { return_to: '/admin/desktop/organizations/org_one?tab=business' }
    api.upload.mockRejectedValue(new Error('upload failed'))
    api.getRecord.mockResolvedValue(record({ id: 'contract-one', organization_id: 'org_one', status: 'SIGNED', details: { name: '年度服务合同', contract_number: 'HT-2026-001' } }))
    router.push.mockImplementation(async (to: { params: Record<string, string>; query: Record<string, string> }) => {
      route.params = { ...to.params }
      route.query = { ...to.query }
    })
    const wrapper = mount(ContractFormView, options)
    await flushPromises()
    await wrapper.get('[data-testid="contract-name"]').setValue('年度服务合同')
    await wrapper.get('[data-testid="contract-number"]').setValue('HT-2026-001')
    await wrapper.get('[data-testid="pick-file"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(router.push).toHaveBeenCalledWith(expect.objectContaining({ name: 'AdminAccountingContractEdit', params: { organizationId: 'org_one', contractId: 'contract-one' } }))
    expect(api.getRecord).toHaveBeenCalledWith('contracts', 'contract-one')
    expect(wrapper.text()).toContain('扫描件上传失败')
    expect(wrapper.find('[data-testid="record-files"]').exists()).toBe(true)

    await wrapper.get('[data-testid="scan-uploaded"]').trigger('click')
    expect(wrapper.text()).not.toContain('扫描件上传失败')
    expect(router.replace).toHaveBeenCalledWith({ query: { return_to: '/admin/desktop/organizations/org_one?tab=business' } })
  })

  it('edits a signed contract and only changes sharing when it changed', async () => {
    route.params = { organizationId: 'org_one', contractId: 'contract-one' }
    route.query = { return_to: '/admin/desktop/organizations/org_one?tab=business' }
    api.getRecord.mockResolvedValue(record({ id: 'contract-one', status: 'SIGNED', amount: '12000.000000', version: 3, customer_visible: true, details: { name: '年度服务合同', contract_number: 'HT-2026-001', contact_id: 'contact-one' } }))
    const wrapper = mount(ContractFormView, options)
    await flushPromises()
    expect(wrapper.find('[data-testid="record-files"]').exists()).toBe(true)
    expect((wrapper.get('[data-testid="contract-amount"]').element as HTMLInputElement).value).toBe('12000')
    await wrapper.get('[data-testid="contract-name"]').setValue('年度服务合同（续签）')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.mutate).toHaveBeenCalledTimes(1)
    expect(api.mutate).toHaveBeenCalledWith('contracts/contract-one', expect.objectContaining({ amount: '12000', version: 3, details: expect.objectContaining({ name: '年度服务合同（续签）', contact_id: 'contact-one' }) }), 'key-contracts/contract-one', 'patch')
    expect(router.push).toHaveBeenCalledWith('/admin/desktop/organizations/org_one?tab=business')
  })

  it('lets a new order name one of the organization’s contracts', async () => {
    route.params = {}
    route.query = { organization: 'org_one' }
    api.organizations.mockResolvedValue([{ id: 'org_one', name: '上海星海智能科技有限公司' }])
    api.organizationContracts.mockResolvedValue([record({ id: 'contract-one', status: 'SIGNED', details: { name: '年度服务合同', contract_number: 'HT-2026-001' } })])
    api.mutate.mockResolvedValue({ id: 'order-one' })
    const wrapper = mount(OrderCreateView, options)
    await flushPromises()
    expect(api.organizationContracts).toHaveBeenCalledWith('org_one')
    await wrapper.get('[data-testid="order-contract"]').setValue('contract-one')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('orders', expect.objectContaining({ organization_id: 'org_one', contract_id: 'contract-one' }), expect.any(String))
  })
})
