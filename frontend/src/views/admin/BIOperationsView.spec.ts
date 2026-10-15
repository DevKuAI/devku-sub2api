import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const api = vi.hoisted(() => ({ getOverview: vi.fn(), list: vi.fn(), userStatus: vi.fn(), retention: vi.fn(), saveRetention: vi.fn(), cleanup: vi.fn() }))
vi.mock('@/api/admin/bi', () => ({ default: api }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import BIOperationsView from './BIOperationsView.vue'
const global = { stubs: { AppLayout: { template: '<main><slot/></main>' }, BIGrantManagement: true, TotpStepUpDialog: true, RouterLink: { props: ['to'], template: '<a><slot/></a>' } } }
const page = (items: unknown[] = []) => ({ items, total: items.length, page: 1, page_size: 20, has_more: false })
beforeEach(() => { vi.clearAllMocks(); api.getOverview.mockResolvedValue({ bi_enabled: false, privacy_notice_published: false, counts: {} }); api.list.mockResolvedValue(page()); api.retention.mockResolvedValue({ audit_days: 180, ephemeral_days: 7 }) })
describe('BI native operations', () => {
  it('offers configuration even when service or privacy is unavailable', async () => { const w = mount(BIOperationsView,{global});await flushPromises();expect(w.text()).toContain('biNative.disabled');expect(w.text()).toContain('biNative.configure');expect(w.text()).toContain('biNative.privacyMissing');w.unmount() })
  it('requests metadata only and renders execution fees without business content', async () => { api.list.mockImplementation(async(path:string)=>page(path==='report-tasks'?[{task_id:1,organization_id:'org',organization_name:'Name',kind:'week',status:'partial',total_tokens:'9007199254740993',actual_cost:1.2,content:'FORBIDDEN_BODY',sources:'FORBIDDEN_INPUT'}]:[]));const w=mount(BIOperationsView,{global});await w.findAll('button').find(b=>b.text()==='biNative.tasks')!.trigger('click');await flushPromises();expect(w.text()).toContain('9,007,199,254,740,993');expect(w.text()).not.toContain('FORBIDDEN');expect(api.list).toHaveBeenCalledWith('report-tasks',1,{kind:''});expect(api.list).toHaveBeenCalledWith('report-executions',1,{kind:''});w.unmount() })
  it('lets a failed identity region retry while healthy session metadata remains',async()=>{api.list.mockImplementation(async(path:string)=>{if(path==='identities/users')throw new Error('failure');return page(path==='identities/sessions'?[{id:'s',bi_user_id:'biu_visible',status:'active'}]:[])});const w=mount(BIOperationsView,{global});await w.findAll('button').find(b=>b.text()==='biNative.identities')!.trigger('click');await flushPromises();expect(w.text()).toContain('biu_visible');expect(w.text()).toContain('biNative.failed');w.unmount()})
})

describe('BI operations interactions', () => {
  it('opens access for the selected organization in a dialog and closes without replacing the list', async () => {
    api.list.mockResolvedValue(page([{ id: 'org-one', name: 'First company' }, { id: 'org-two', name: 'Second company' }]))
    const wrapper = mount(BIOperationsView, { global })
    await wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'biNative.onboarding')!.trigger('click')
    await flushPromises()
    expect(wrapper.findComponent({ name: 'BIGrantManagement' }).exists()).toBe(false)
    await wrapper.findAll('tbody button')[1].trigger('click')
    await flushPromises()
    const dialog = wrapper.findComponent({ name: 'BaseDialog' })
    expect(dialog.props('show')).toBe(true)
    expect(wrapper.findComponent({ name: 'BIGrantManagement' }).props('organizationId')).toBe('org-two')
    dialog.vm.$emit('close')
    await flushPromises()
    expect(wrapper.findComponent({ name: 'BIGrantManagement' }).exists()).toBe(false)
    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    wrapper.unmount()
  })

  it('requires confirmation before identity cleanup and announces the result', async () => {
    const wrapper = mount(BIOperationsView, { global })
    await wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'biNative.audit')!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'biNative.cleanup')!.trigger('click')
    expect(api.cleanup).not.toHaveBeenCalled()
    const confirm = wrapper.findComponent({ name: 'ConfirmDialog' })
    expect(confirm.props('show')).toBe(true)
    confirm.vm.$emit('confirm')
    await flushPromises()
    expect(api.cleanup).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('biNative.cleanupCompleted')
    wrapper.unmount()
  })

  it('invalidates old searches and does not offer pagination for the previous query', async () => {
    let resolveOld!: (value: ReturnType<typeof page>) => void
    const old = new Promise<ReturnType<typeof page>>(resolve => { resolveOld = resolve })
    api.list.mockReturnValueOnce(old)
    const wrapper = mount(BIOperationsView, { global })
    await wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'biNative.onboarding')!.trigger('click')
    await wrapper.get('input[type="search"]').setValue('new')
    resolveOld(page([{ id: 'old', name: 'STALE_ORGANIZATION' }]))
    await flushPromises()
    expect(wrapper.text()).not.toContain('STALE_ORGANIZATION')
    expect(wrapper.text()).not.toContain('bi.loadMore')
    wrapper.unmount()
  })
})
