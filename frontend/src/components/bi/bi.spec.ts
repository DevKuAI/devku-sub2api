import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
const api = vi.hoisted(() => ({ users: vi.fn(), invitations: vi.fn(), create: vi.fn(), revoke: vi.fn(), access: vi.fn() }))
const management = vi.hoisted(() => vi.fn(() => api))
vi.mock('@/api/bi', async original => ({ ...await original<typeof import('@/api/bi')>(), isBIEnabled: async () => true, biManagement: management }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import BIGrantManagement from './BIGrantManagement.vue'
import { capabilityDependencies } from '@/api/bi'
const global = { stubs: { TotpStepUpDialog: { props: ['controller'], template: '<button v-if="controller.visible.value" data-verify @click="controller.onVerified()">verify</button>' }, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' } } }
const page = (items: unknown[] = []) => ({ items, page: 1, page_size: 20, total: items.length, has_more: false })
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(r => { resolve = r }); return { promise, resolve } }
beforeEach(() => { vi.clearAllMocks(); api.users.mockResolvedValue(page()); api.invitations.mockResolvedValue(page()); api.create.mockResolvedValue({ token: 'invitation-secret', path: '/pages/settings/index?invite=invitation-secret' }) })
describe('Independent mini-program access', () => {
  it('selects real content permissions and adds dependencies without web-user or team-ID inputs', async () => {
    const wrapper = mount(BIGrantManagement, { props: { organizationId: 'org' }, global }); await flushPromises(); await wrapper.findAll('button').find(b => b.text() === 'biNative.createInvite')!.trigger('click'); await wrapper.get('input[maxlength="80"]').setValue('张经理'); const boxes = wrapper.findAll('input[type="checkbox"]'); await boxes[4].setValue(true); await wrapper.get('#bi-access-form').trigger('submit'); await flushPromises(); expect(api.create).toHaveBeenCalledWith(['analytics:read','members:read','reports:read','reports:copy'],'','张经理'); expect((wrapper.findAll('input[readonly]')[0].element as HTMLInputElement).value).toBe('invitation-secret'); expect(wrapper.find('img[alt="biNative.inviteQRCode"]').attributes('src')).toMatch(/^data:image\/svg\+xml/); expect(wrapper.find('#bi-grant-user').exists()).toBe(false); expect(wrapper.find('#bi-grant-teams').exists()).toBe(false); wrapper.unmount()
  })
  it('uses the owner route scope and drops late results from another organization', async () => {
    const late = deferred<ReturnType<typeof page>>(); api.users.mockReturnValueOnce(late.promise); const wrapper = mount(BIGrantManagement, { props: { organizationId: 'old', selfManaged: true }, global }); await flushPromises(); await wrapper.setProps({ organizationId: 'new' }); await flushPromises(); late.resolve(page([{bi_user_id:'old-user',display_name:'OLD_PRIVATE'}])); await flushPromises(); expect(wrapper.text()).not.toContain('OLD_PRIVATE'); expect(management).toHaveBeenCalledWith('new',true); wrapper.unmount()
  })
  it('retains the draft on revision conflict and does not block invitations when users fail', async () => {
    api.users.mockRejectedValueOnce(new Error('fail')); api.invitations.mockResolvedValue(page([{id:'i',note:'INVITE_VISIBLE',status:'pending',expires_at:'later',capabilities:['analytics:read']}]))
    const wrapper = mount(BIGrantManagement,{props:{organizationId:'org'},global});await flushPromises();expect(wrapper.text()).toContain('INVITE_VISIBLE');expect(wrapper.text()).toContain('biNative.failed');wrapper.unmount()
  })
  it('verifies step-up and retries the same invitation once', async () => { api.create.mockRejectedValueOnce({ status:403, code:'STEP_UP_REQUIRED' }); const w=mount(BIGrantManagement,{props:{organizationId:'org'},global});await flushPromises();await w.findAll('button').find(b=>b.text()==='biNative.createInvite')!.trigger('click');await w.get('#bi-access-form').trigger('submit');await flushPromises();expect(api.create).toHaveBeenCalledTimes(1);await w.get('[data-verify]').trigger('click');await flushPromises();expect(api.create).toHaveBeenCalledTimes(2);expect(api.create.mock.calls[0]).toEqual(api.create.mock.calls[1]);w.unmount() })
  it('keeps the organization label draft on revision conflicts', async () => {
    const user = {bi_user_id:'biu_reader',display_name:'小程序使用者',organization_alias:'项目负责人',user_status:'active',status:'active',capabilities:['analytics:read'],revision:3}
    api.users.mockResolvedValue(page([user])); api.access.mockRejectedValue({status:409})
    const wrapper=mount(BIGrantManagement,{props:{organizationId:'org'},global}); await flushPromises()
    expect(wrapper.text()).toContain('项目负责人')
    await wrapper.findAll('button').find(button=>button.text()==='common.edit')!.trigger('click')
    await wrapper.get('input[maxlength="80"]').setValue('新称呼')
    await wrapper.get('#bi-access-form').trigger('submit');await flushPromises()
    expect(api.access).toHaveBeenCalledWith(user,['analytics:read'],'active','新称呼')
    expect(wrapper.text()).toContain('biNative.conflict')
    expect((wrapper.get('input[maxlength="80"]').element as HTMLInputElement).value).toBe('新称呼')
    wrapper.unmount()
  })
  it('validates capability dependencies' , () => { expect(capabilityDependencies(['reports:copy'])).toEqual(['members:read','reports:read','reports:copy']); expect(capabilityDependencies(['conversations:read'])).toEqual(['members:read','conversations:read']) })
})
