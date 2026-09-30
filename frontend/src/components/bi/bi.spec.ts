import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  isBIEnabled: vi.fn(), listBindings: vi.fn(), approveBinding: vi.fn(), revokeBinding: vi.fn(),
  listGrants: vi.fn(), saveGrant: vi.fn(), revokeGrant: vi.fn(), approveGrantBinding: vi.fn(),
}))
const userAPI = vi.hoisted(() => ({ list: vi.fn(), getById: vi.fn() }))
vi.mock('@/api/bi', () => api)
vi.mock('@/api/admin', () => ({ adminAPI: { users: userAPI } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { email: 'manager@example.com' } }) }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import BIBindingsView from '@/views/user/BIBindingsView.vue'
import BIGrantManagement from './BIGrantManagement.vue'
import Select from '@/components/common/Select.vue'

const global = {
  stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
    ConfirmDialog: { props: ['show'], emits: ['confirm', 'cancel'], template: '<button v-if="show" data-confirm @click="$emit(\'confirm\')">confirm</button>' },
  },
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => { resolve = resolvePromise; reject = rejectPromise })
  return { promise, resolve, reject }
}

describe('BI management flows', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.isBIEnabled.mockResolvedValue(true)
    api.listBindings.mockResolvedValue({ items: [], next_cursor: null, has_more: false, snapshot_id: 'snapshot' })
    api.listGrants.mockResolvedValue({ items: [], page: 1, has_more: false, capabilities: ['analytics:read', 'reports:share'] })
    userAPI.list.mockResolvedValue({ items: [{ id: 42, username: 'Manager', email: 'manager@example.com', status: 'active' }], total: 1 })
    userAPI.getById.mockResolvedValue({ id: 12, username: 'Existing manager', email: 'existing@example.com', status: 'active' })
  })

  it('searches active users, displays the selected identity, and submits its ID with imported teams', async () => {
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org-one' }, global })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'bi.addGrant')!.trigger('click')
    await flushPromises()

    const userSelect = wrapper.get('#bi-grant-user')
    expect(userSelect.element.tagName).toBe('BUTTON')
    expect(userSelect.attributes('aria-describedby')).toBe('bi-grant-user-hint')
    expect(userSelect.text()).toContain('bi.selectUserPlaceholder')
    expect(wrapper.get('#bi-grant-user-hint').text()).toContain('bi.userIDHint')
    expect(wrapper.get('#bi-grant-user-hint a').attributes()).toMatchObject({ href: '/admin/users', target: '_blank' })
    expect(wrapper.get('#bi-grant-teams-hint').text()).toBe('bi.teamsHint')
    expect(userAPI.list).toHaveBeenCalledWith(1, 20, { status: 'active', search: undefined }, { signal: expect.any(AbortSignal) })

    await wrapper.get('#bi-grant-form').trigger('submit')
    expect(wrapper.get('#bi-grant-user-error').text()).toBe('bi.selectUserRequired')

    const select = wrapper.findComponent(Select)
    select.vm.$emit('search', 'manager@example.com')
    await flushPromises()
    expect(userAPI.list).toHaveBeenLastCalledWith(1, 20, { status: 'active', search: 'manager@example.com' }, { signal: expect.any(AbortSignal) })
    select.vm.$emit('update:modelValue', 42)
    select.vm.$emit('change', 42)
    await flushPromises()
    expect(wrapper.find('#bi-grant-user-error').exists()).toBe(false)
    expect(wrapper.get('#bi-grant-form dl').text()).toContain('Manager')
    expect(wrapper.get('#bi-grant-form dl').text()).toContain('manager@example.com')
    expect(wrapper.get('#bi-grant-form dl').text()).toContain('42')

    await wrapper.get('#bi-grant-teams').setValue('hr:team, hr:ops')
    await wrapper.get('input[value="analytics:read"]').setValue(true)
    await wrapper.get('#bi-grant-form').trigger('submit')
    await flushPromises()

    expect(api.saveGrant).toHaveBeenCalledWith('org-one', expect.objectContaining({ user_id: 42, team_ids: ['hr:team', 'hr:ops'] }))
    wrapper.unmount()
  })

  it('shows the current account details when editing an existing grant', async () => {
    api.listGrants.mockResolvedValue({
      items: [{ manager_id: 'manager', user_id: 12, display_name: 'Existing manager', role: 'viewer', all_teams: true, team_ids: [], capabilities: ['analytics:read'], revision: 3, revoked: false }],
      page: 1, has_more: false, capabilities: ['analytics:read'],
    })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org-one' }, global })
    await flushPromises()
    await wrapper.get('li button').trigger('click')
    await flushPromises()

    expect(userAPI.getById).toHaveBeenCalledWith(12)
    expect(wrapper.get('#bi-grant-user').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#bi-grant-form dl').text()).toContain('Existing manager')
    expect(wrapper.get('#bi-grant-form dl').text()).toContain('existing@example.com')
    expect(wrapper.get('#bi-grant-form dl').text()).toContain('12')
    wrapper.unmount()
  })

  it('lets an admin approve a binding code for an active enterprise grant', async () => {
    api.listGrants.mockResolvedValue({
      items: [{ manager_id: 'bim_target', user_id: 42, display_name: 'Manager', role: 'viewer', all_teams: true, team_ids: [], capabilities: ['analytics:read'], revision: 1, revoked: false }],
      page: 1, has_more: false, capabilities: ['analytics:read'],
    })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org-one' }, global })
    await flushPromises()
    await wrapper.findAll('li button').find(button => button.text() === 'bi.bindWechat')!.trigger('click')
    expect(wrapper.get('#bi-admin-binding-form').text()).toContain('bi.bindWechatTarget')
    await wrapper.get('#bi-admin-binding-code').setValue('abcd1234')
    await wrapper.get('#bi-admin-binding-form').trigger('submit')
    expect(api.approveGrantBinding).not.toHaveBeenCalled()
    await wrapper.get('#bi-admin-binding-form input[type="checkbox"]').setValue(true)
    await wrapper.get('#bi-admin-binding-form').trigger('submit')
    await flushPromises()
    expect(api.approveGrantBinding).toHaveBeenCalledWith('org-one', 'bim_target', 'ABCD1234')
    expect(wrapper.get('[role="status"]').text()).toBe('bi.adminBindingApproved')
    wrapper.unmount()
  })

  it('shows a binding failure without claiming approval', async () => {
    api.listGrants.mockResolvedValue({
      items: [{ manager_id: 'bim_target', user_id: 42, display_name: 'Manager', role: 'viewer', all_teams: true, team_ids: [], capabilities: ['analytics:read'], revision: 1, revoked: false }],
      page: 1, has_more: false, capabilities: ['analytics:read'],
    })
    api.approveGrantBinding.mockRejectedValue({ code: 'NOT_FOUND' })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org-one' }, global })
    await flushPromises()
    await wrapper.findAll('li button').find(button => button.text() === 'bi.bindWechat')!.trigger('click')
    await wrapper.get('#bi-admin-binding-code').setValue('ABCD1234')
    await wrapper.get('#bi-admin-binding-form input[type="checkbox"]').setValue(true)
    await wrapper.get('#bi-admin-binding-form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('#bi-admin-binding-form [role="alert"]').text()).toBe('bi.adminBindingGrantMissing')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    wrapper.unmount()
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

  it.each(['success', 'failure'])('ignores a pending binding refresh %s after revocation', async (outcome) => {
    const oldPage = { items: [{ id: 'binding-one', display_name: 'mini-program', created_at: '2026-09-23T00:00:00Z', last_login_at: null }], next_cursor: null, has_more: false, snapshot_id: 'old' }
    const refresh = deferred<typeof oldPage>()
    api.listBindings.mockResolvedValueOnce(oldPage).mockReturnValueOnce(refresh.promise)
    const wrapper = mount(BIBindingsView, { global })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'common.refresh')!.trigger('click')
    await flushPromises()
    await wrapper.get('li button').trigger('click')
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(api.listBindings).toHaveBeenCalledTimes(3)
    expect(wrapper.find('li').exists()).toBe(false)
    if (outcome === 'success') refresh.resolve(oldPage)
    else refresh.reject({ status: 500 })
    await flushPromises()
    expect(wrapper.find('li').exists()).toBe(false)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('[role="status"]').text()).toBe('bi.revoked')
    wrapper.unmount()
  })

  it.each(['save', 'revoke'])('keeps the latest grants after %s when an older refresh finishes last', async (operation) => {
    const grant = { manager_id: 'manager', user_id: 12, display_name: 'Manager', role: 'viewer', all_teams: true, team_ids: [], capabilities: ['analytics:read'], revision: 3, revoked: false }
    const oldPage = { items: [grant], page: 1, has_more: false, capabilities: ['analytics:read'] }
    const refresh = deferred<typeof oldPage>()
    const updated = operation === 'save' ? { ...grant, role: 'org_admin', revision: 4 } : { ...grant, revoked: true, revision: 4 }
    api.listGrants.mockResolvedValueOnce(oldPage).mockReturnValueOnce(refresh.promise).mockResolvedValueOnce({ ...oldPage, items: [updated] })
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org-one' }, global })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'common.refresh')!.trigger('click')
    await flushPromises()
    if (operation === 'save') {
      await wrapper.get('li button').trigger('click')
      await wrapper.get('#bi-grant-role').setValue('org_admin')
      await wrapper.get('#bi-grant-form').trigger('submit')
    } else {
      await wrapper.findAll('li button').find(button => button.text() === 'bi.revokeGrant')!.trigger('click')
      await wrapper.get('[data-confirm]').trigger('click')
    }
    await flushPromises()
    const expected = operation === 'save' ? 'bi.roles.org_admin' : 'bi.grantRevoked'
    expect(wrapper.get('li').text()).toContain(expected)
    refresh.resolve(oldPage)
    await flushPromises()
    expect(wrapper.get('li').text()).toContain(expected)
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
