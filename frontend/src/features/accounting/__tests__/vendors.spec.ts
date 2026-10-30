import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, receivingAccount, record, stubs, vendors } from './testing'
import type { AccountingOrder } from '../types'

const api = vi.hoisted(() => ({
  vendors: vi.fn(),
  vendor: vi.fn(),
  createVendor: vi.fn(),
  updateVendor: vi.fn(),
  addEntry: vi.fn(),
  updateEntry: vi.fn(),
  stableRequest: () => Object.assign((path: string) => `key-${path}`, { reset: vi.fn() }),
}))
const router = vi.hoisted(() => ({ push: vi.fn() }))
const route = vi.hoisted(() => ({ params: {} as Record<string, string>, query: {} as Record<string, string>, fullPath: '/admin/accounting/orders/order-one' }))

vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRouter: () => router, useRoute: () => route, onBeforeRouteLeave: vi.fn() }))

import VendorsView from '../VendorsView.vue'
import VendorFormView from '../VendorFormView.vue'
import EntryForm from '../components/order/EntryForm.vue'

enableAutoUnmount(afterEach)
const options = { global: { plugins: [i18n()], stubs: { ...stubs, FormPage: { props: ['error'], emits: ['submit'], template: '<form @submit.prevent="$emit(\'submit\')"><p v-if="error">{{ error }}</p><slot /></form>' }, FormSection: { template: '<section><slot /></section>' } } } }

describe('vendor lists', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.params = {}
    route.query = {}
    api.vendors.mockResolvedValue(vendors)
  })

  it('lists card providers and AI vendors separately with their status', async () => {
    const wrapper = mount(VendorsView, options)
    await flushPromises()
    expect(wrapper.get('[data-testid="vendors-CARD_PROVIDER"]').text()).toContain('SUNRATE')
    const ai = wrapper.get('[data-testid="vendors-AI_VENDOR"]').text()
    expect(ai).toContain('OpenAI')
    expect(ai).toContain('已停用')
  })

  it('saves a new vendor of the requested kind with its aliases split', async () => {
    route.query = { kind: 'AI_VENDOR' }
    api.createVendor.mockResolvedValue(vendors[1])
    const wrapper = mount(VendorFormView, options)
    await flushPromises()
    await wrapper.get('[data-testid="vendor-name"]').setValue(' Mistral ')
    await wrapper.get('[data-testid="vendor-aliases"]').setValue('mistral ai、le chat, mistral')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.createVendor).toHaveBeenCalledWith(
      { kind: 'AI_VENDOR', name: 'Mistral', aliases: ['mistral ai', 'le chat', 'mistral'], status: 'ACTIVE', sort: 0, remark: '' },
      'key-vendors',
    )
  })

  it('offers only active vendors and names the free text of an unclassified record', async () => {
    const context = {
      order: { id: 'order-one' } as AccountingOrder,
      items: { transfers: [], invoices: [], 'vcc-cards': [], 'vcc-transactions': [], 'vendor-invoices': [] },
      receivingAccounts: [receivingAccount],
      vendors,
    }
    const fresh = mount(EntryForm, { ...options, props: { kind: 'vendor-invoices', context } })
    const offered = fresh.get('[data-testid="entry-vendor-vendor_id"]').findAll('option').map((option) => option.text())
    expect(offered).toContain('OpenAI')
    expect(offered.join()).not.toContain('Retired')

    const legacy = record({ id: 'invoice-one', status: 'CONFIRMED', currency: 'USD', amount: '20', details: { vendor: 'OPENAI *CHATGPT', period_start: '2026-10-01', period_end: '2026-10-31' } })
    const editing = mount(EntryForm, { ...options, props: { kind: 'vendor-invoices', context, record: legacy } })
    expect(editing.text()).toContain('原记录填写的是“OPENAI *CHATGPT”')
  })

  it('lets the browser accept a card ID made of digits and reject one with spaces', () => {
    const context = {
      order: { id: 'order-one' } as AccountingOrder,
      items: { transfers: [], invoices: [], 'vcc-cards': [], 'vcc-transactions': [], 'vendor-invoices': [] },
      receivingAccounts: [receivingAccount],
      vendors,
    }
    const wrapper = mount(EntryForm, { ...options, props: { kind: 'vcc-cards', context } })
    // Browsers compile the pattern attribute as an anchored regular expression with the v flag.
    const pattern = new RegExp(`^(?:${wrapper.get('[data-testid="entry-card-id-provider_card_id"]').attributes('pattern')})$`, 'v')
    expect(pattern.test('2108375553744965633')).toBe(true)
    expect(pattern.test('2108 3755')).toBe(false)
  })
})
