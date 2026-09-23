import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  isBIEnabled: vi.fn(), listBindings: vi.fn(), approveBinding: vi.fn(), revokeBinding: vi.fn(),
  listGrants: vi.fn(), saveGrant: vi.fn(), revokeGrant: vi.fn(),
}))
vi.mock('@/api/bi', () => api)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { email: 'manager@example.com' } }) }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import BIBindingsView from '@/views/user/BIBindingsView.vue'
import BIGrantManagement from './BIGrantManagement.vue'

const global = {
  stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
    ConfirmDialog: { props: ['show'], emits: ['confirm', 'cancel'], template: '<button v-if="show" data-confirm @click="$emit(\'confirm\')">confirm</button>' },
  },
}

describe('BI management flows', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.isBIEnabled.mockResolvedValue(true)
    api.listBindings.mockResolvedValue({ items: [], next_cursor: null, has_more: false, snapshot_id: 'snapshot' })
    api.listGrants.mockResolvedValue({ items: [], page: 1, has_more: false, capabilities: ['analytics:read', 'reports:share'] })
  })

  it('requires explicit approval and sends only the normalized challenge code', async () => {
    const wrapper = mount(BIBindingsView, { global })
    await flushPromises()
    await wrapper.get('#bi-code').setValue('abcd1234')
    await wrapper.get('form').trigger('submit')
    expect(api.approveBinding).not.toHaveBeenCalled()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.approveBinding).toHaveBeenCalledWith('ABCD1234')
    expect(wrapper.get('[role="status"]').text()).toBe('bi.approved')
    expect((wrapper.get('#bi-code').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })

  it('shows an expired challenge without pretending the binding succeeded', async () => {
    api.approveBinding.mockRejectedValue({ status: 410, code: 'BINDING_EXPIRED' })
    const wrapper = mount(BIBindingsView, { global })
    await flushPromises()
    await wrapper.get('#bi-code').setValue('ABCD1234')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('bi.expired')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('revokes a binding only after confirmation', async () => {
    api.listBindings.mockResolvedValueOnce({ items: [{ id: 'binding-one', display_name: 'mini-program', created_at: '2026-09-23T00:00:00Z', last_login_at: null }], next_cursor: null, has_more: false, snapshot_id: 'snapshot' })
    const wrapper = mount(BIBindingsView, { global })
    await flushPromises()
    await wrapper.get('li button').trigger('click')
    expect(api.revokeBinding).not.toHaveBeenCalled()
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(api.revokeBinding).toHaveBeenCalledWith('binding-one')
    expect(wrapper.find('li').exists()).toBe(false)
    wrapper.unmount()
  })

  it('retains the edited grant and displays a conflict instead of overwriting newer permissions', async () => {
    api.listGrants.mockResolvedValue({ items: [{ manager_id: 'manager', user_id: 12, display_name: 'Manager', role: 'viewer', all_teams: true, team_ids: [], capabilities: ['analytics:read'], revision: 3, revoked: false }], page: 1, has_more: false, capabilities: ['analytics:read', 'reports:share'] })
    api.saveGrant.mockRejectedValue({ status: 409 })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org-one' }, global })
    await flushPromises()
    await wrapper.get('li button').trigger('click')
    await wrapper.get('#bi-grant-form').trigger('submit')
    await flushPromises()
    expect(api.saveGrant).toHaveBeenCalledWith('org-one', expect.objectContaining({ user_id: 12, expected_revision: 3, capabilities: ['analytics:read'] }))
    expect(wrapper.get('#bi-grant-form [role="alert"]').text()).toBe('bi.reload')
    expect(wrapper.find('#bi-grant-form').exists()).toBe(true)
    wrapper.unmount()
  })

  it('ignores an old organization response after switching scope', async () => {
    let resolveOld!: (value: unknown) => void
    api.listGrants.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'old-org' }, global })
    await flushPromises()
    await wrapper.setProps({ organizationId: 'new-org' })
    await flushPromises()
    resolveOld({ items: [{ manager_id: 'old', display_name: 'Old secret name' }], page: 1, has_more: false, capabilities: [] })
    await flushPromises()
    expect(wrapper.text()).not.toContain('Old secret name')
    expect(api.listGrants).toHaveBeenLastCalledWith('new-org', 1)
    wrapper.unmount()
  })
})
