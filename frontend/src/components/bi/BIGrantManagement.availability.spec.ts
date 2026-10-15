import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BIGrantManagement from './BIGrantManagement.vue'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))
vi.mock('vue-i18n', async original => ({
  ...await original<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
const global = { stubs: { BaseDialog: true, TotpStepUpDialog: true } }
const empty = { items: [], total: 0, page: 1, page_size: 20, has_more: false }

beforeEach(() => {
  vi.clearAllMocks()
  client.get.mockImplementation(async (path: string) => {
    if (path === '/settings/public') return { data: { bi_enabled: true } }
    if (path === '/settings') throw { status: 404 }
    return { data: empty }
  })
})

describe('BI availability', () => {
  it('does not treat a missing availability flag as a disabled service', async () => {
    client.get.mockResolvedValueOnce({ data: {} })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org' }, global })
    await flushPromises()
    expect(wrapper.text()).toContain('biNative.availabilityFailed')
    expect(wrapper.text()).not.toContain('biNative.disabled')
    expect(client.get).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('reports a settings failure without claiming the service is disabled, then retries', async () => {
    client.get.mockRejectedValueOnce({ status: 503 })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org' }, global })
    await flushPromises()
    expect(wrapper.text()).toContain('biNative.availabilityFailed')
    expect(wrapper.text()).not.toContain('biNative.disabled')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('biNative.availabilityFailed')
    expect(wrapper.findAll('button').find(button => button.text() === 'biNative.createInvite')!.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('keeps invitations disabled only when the service explicitly returns false', async () => {
    client.get.mockResolvedValueOnce({ data: { bi_enabled: false } })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org' }, global })
    await flushPromises()
    expect(wrapper.text()).toContain('biNative.disabled')
    expect(client.get).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it.each([false, true])('allows invitations when public settings enable BI with selfManaged=%s', async selfManaged => {
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org', selfManaged }, global })
    await flushPromises()
    const invite = wrapper.findAll('button').find(button => button.text() === 'biNative.createInvite')!
    expect(invite.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('biNative.disabled')
    const base = selfManaged ? '/desktop/organization/bi' : '/admin/desktop/operations/organizations/org'
    expect(client.get).toHaveBeenCalledWith(`${base}/users`, expect.any(Object))
    expect(client.get).toHaveBeenCalledWith(`${base}/invitations`, expect.any(Object))
    wrapper.unmount()
  })
})
