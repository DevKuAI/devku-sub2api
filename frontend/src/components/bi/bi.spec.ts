import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
const api = vi.hoisted(() => ({ users: vi.fn(), invitations: vi.fn(), create: vi.fn(), revoke: vi.fn(), access: vi.fn(), qrcode: vi.fn() }))
const management = vi.hoisted(() => vi.fn(() => api))
vi.mock('@/api/bi', async original => ({ ...await original<typeof import('@/api/bi')>(), isBIEnabled: async () => true, biManagement: management }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
const router = vi.hoisted(() => ({ push: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: { from: 'identities' } }), useRouter: () => router }))
import BIGrantManagement from './BIGrantManagement.vue'
import { capabilityDependencies } from '@/api/bi'
const global = { stubs: { TotpStepUpDialog: { props: ['controller'], template: '<div v-if="controller.visible.value"><button data-verify @click="controller.onVerified()">verify</button><button data-cancel @click="controller.onCancel()">cancel</button></div>' }, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' } } }
const page = (items: unknown[] = []) => ({ items, page: 1, page_size: 20, total: items.length, has_more: false })
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(r => { resolve = r }); return { promise, resolve } }
beforeEach(() => { vi.clearAllMocks(); api.users.mockResolvedValue(page()); api.invitations.mockResolvedValue(page()); api.qrcode.mockResolvedValue({ mime_type: 'image/png', image_base64: 'aW1hZ2U=' }); api.create.mockResolvedValue({ id: 'invite-one', scene: 's'.repeat(32), status: 'pending', expires_at: '2099-01-01T00:00:00Z', token: 'invitation-secret', path: '/pages/settings/index?invite=invitation-secret' }) })
describe('Independent mini-program access', () => {
  it('opens creation and editing on independent routes without rendering an editable dialog', async () => {
    api.users.mockResolvedValue(page([{bi_user_id:'biu_reader',display_name:'Reader',status:'active',capabilities:['analytics:read'],revision:3}]))
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org' }, global }); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'biNative.createInvite')!.trigger('click')
    expect(router.push).toHaveBeenCalledWith({path:'/admin/desktop/organizations/org/bi/invitations/new',query:{from:'identities'}})
    await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
    expect(router.push).toHaveBeenLastCalledWith({path:'/admin/desktop/organizations/org/bi/users/biu_reader/edit',query:{from:'identities'}})
    expect(wrapper.find('form').exists()).toBe(false)
    expect(api.create).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('uses the owner route scope and drops late results from another organization', async () => {
    const late = deferred<ReturnType<typeof page>>(); api.users.mockReturnValueOnce(late.promise); const wrapper = mount(BIGrantManagement, { props: { organizationId: 'old', selfManaged: true }, global }); await flushPromises(); await wrapper.setProps({ organizationId: 'new' }); await flushPromises(); late.resolve(page([{bi_user_id:'old-user',display_name:'OLD_PRIVATE'}])); await flushPromises(); expect(wrapper.text()).not.toContain('OLD_PRIVATE'); expect(management).toHaveBeenCalledWith('new',true); wrapper.unmount()
  })
  it('retains the draft on revision conflict and does not block invitations when users fail', async () => {
    api.users.mockRejectedValueOnce(new Error('fail')); api.invitations.mockResolvedValue(page([{id:'i',note:'INVITE_VISIBLE',status:'pending',expires_at:'later',capabilities:['analytics:read']}]))
    const wrapper = mount(BIGrantManagement,{props:{organizationId:'org'},global});await flushPromises();expect(wrapper.text()).toContain('INVITE_VISIBLE');expect(wrapper.text()).toContain('biNative.failed');wrapper.unmount()
  })
  it('requires confirmation before revoking an invitation', async () => {
    api.invitations.mockResolvedValue(page([{ id: 'invite-one', note: 'Recipient', status: 'pending', expires_at: '', capabilities: ['analytics:read'] }]))
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org' }, global })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'biNative.revokeInvite')!.trigger('click')
    expect(api.revoke).not.toHaveBeenCalled()
    const confirm = wrapper.findComponent({ name: 'ConfirmDialog' })
    expect(confirm.props('show')).toBe(true)
    confirm.vm.$emit('confirm')
    await flushPromises()
    expect(api.revoke).toHaveBeenCalledWith('invite-one')
    wrapper.unmount()
  })
  it('validates capability dependencies' , () => { expect(capabilityDependencies(['reports:copy'])).toEqual(['members:read','reports:read','reports:copy']); expect(capabilityDependencies(['conversations:read'])).toEqual(['members:read','conversations:read']) })
})
