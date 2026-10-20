import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { i18n, organization, stubs } from './testing'

const api = vi.hoisted(() => ({
  organizations: vi.fn(),
  mutate: vi.fn(),
  stableRequest: () => Object.assign((path: string) => `key-${path}`, { reset: vi.fn() }),
}))
const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))
const router = vi.hoisted(() => ({ push: vi.fn() }))
vi.mock('../api', () => api)
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router, onBeforeRouteLeave: vi.fn() }))

import OrderCreateView from '../OrderCreateView.vue'
import { localDate } from '../shared'

enableAutoUnmount(afterEach)

async function render(query: Record<string, string> = {}) {
  route.query = query
  const wrapper = mount(OrderCreateView, { global: { plugins: [i18n()], stubs } })
  await flushPromises()
  return wrapper
}

describe('new order page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.organizations.mockResolvedValue([organization, { id: 'org-two', name: '另一家企业', summary: [] }])
    api.mutate.mockResolvedValue({ id: 'order-new' })
  })

  it('names the order after the chosen organization and opens it once created', async () => {
    const wrapper = await render({ organization: 'org-one' })
    const name = wrapper.get('[data-testid="order-name"]').element as HTMLInputElement
    expect(name.value).toBe(`上海星海智能科技有限公司 ${localDate()}`)

    await wrapper.get('[data-testid="order-organization"]').setValue('org-two')
    expect(name.value).toBe(`另一家企业 ${localDate()}`)

    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.mutate).toHaveBeenCalledWith('orders', { organization_id: 'org-two', name: `另一家企业 ${localDate()}`, remark: '' }, expect.any(String))
    expect(router.push).toHaveBeenCalledWith({ name: 'AdminAccountingOrder', params: { orderId: 'order-new' } })
  })

  it('keeps a name the user typed when the organization changes', async () => {
    const wrapper = await render({ organization: 'org-one' })
    await wrapper.get('[data-testid="order-name"]').setValue('Gemini 试用')
    await wrapper.get('[data-testid="order-organization"]').setValue('org-two')
    expect((wrapper.get('[data-testid="order-name"]').element as HTMLInputElement).value).toBe('Gemini 试用')
  })
})
