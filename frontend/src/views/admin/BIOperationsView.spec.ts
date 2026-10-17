import { reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const api = vi.hoisted(() => ({ getOverview: vi.fn(), list: vi.fn(), userStatus: vi.fn(), retention: vi.fn(), saveRetention: vi.fn(), cleanup: vi.fn(), reportStatistics: vi.fn() }))
vi.mock('@/api/admin/bi', () => ({ default: api }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
import BIOperationsView from './BIOperationsView.vue'
const route = reactive({ query: {} as Record<string, string> })
const router = { push: vi.fn(), replace: vi.fn(async ({ query }: { query: Record<string, string> }) => { route.query = query }) }
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => router }))
const global = { stubs: { AppLayout: { template: '<main><slot/></main>' }, BIGrantManagement: true, TotpStepUpDialog: true, RouterLink: { props: ['to'], template: '<a><slot/></a>' } } }
const page = (items: unknown[] = []) => ({ items, total: items.length, page: 1, page_size: 20, has_more: false })
const statistics = { task_count: '25', organization_count: '2', execution_count: '40', total_tokens: '9007199254740993', actual_cost: '1.2500000001', missing_token_count: '3', missing_cost_count: '2', task_statuses: { completed: '24', failed: '1' }, execution_statuses: { completed: '39', failed: '1' }, models: [] }
beforeEach(() => { vi.clearAllMocks(); route.query = {}; api.reportStatistics.mockResolvedValue(statistics); api.getOverview.mockResolvedValue({ bi_enabled: true, desktop_enabled: true, privacy_notice_published: false, counts: {} }); api.list.mockResolvedValue(page()); api.retention.mockResolvedValue({ audit_days: 180, ephemeral_days: 7 }) })
describe('BI native operations', () => {
  it('keeps organization and report operations available when the mini-program is disabled', async () => {
    api.getOverview.mockResolvedValue({ bi_enabled: false, desktop_enabled: true, privacy_notice_published: false, counts: { organizations: 3, members: 8 } })
    const w = mount(BIOperationsView, { global }); await flushPromises()
    expect(w.get('h1').classes()).toContain('lg:hidden')
    expect(w.text()).toContain('biNative.counts.organizations')
    expect(w.text()).not.toContain('biNative.config.appid')
    await w.get('#bi-tab-onboarding').trigger('click'); await flushPromises()
    expect(api.list).toHaveBeenCalledWith('organizations', 1, { search: '' })
    expect(w.text()).not.toContain('biNative.invitations')
    await w.get('#bi-tab-tasks').trigger('click'); await flushPromises()
    expect(api.reportStatistics).toHaveBeenCalledWith({ kind: '' })
    api.list.mockClear()
    for (const tab of ['identities', 'audit']) {
      await w.get(`#bi-tab-${tab}`).trigger('click'); await flushPromises()
      expect(w.text()).toContain('biNative.miniProgramDisabled')
    }
    expect(api.list).not.toHaveBeenCalled()
    expect(api.retention).not.toHaveBeenCalled()
    await w.get('#bi-tab-identities').trigger('click'); await flushPromises()
    expect(w.text()).toContain('biNative.configure')
    expect(w.text()).toContain('biNative.privacyMissing')
    w.unmount()
  })
  it('restores a linked tab and preserves other query parameters when switching', async () => {
    route.query = { tab: 'tasks', source: 'organization' }
    const w = mount(BIOperationsView, { global }); await flushPromises()
    expect(w.get('#bi-tab-tasks').attributes('aria-selected')).toBe('true')
    expect(api.reportStatistics).toHaveBeenCalled()
    await w.get('#bi-tab-identities').trigger('click'); await flushPromises()
    expect(route.query).toEqual({ tab: 'identities', source: 'organization' })
    expect(api.list).toHaveBeenCalledWith('identities/users', 1, {})
    w.unmount()
  })
  it('shows a retryable status failure instead of claiming the mini-program is disabled', async () => {
    route.query = { tab: 'identities' }
    api.getOverview.mockRejectedValueOnce(new Error('offline'))
    const w = mount(BIOperationsView, { global }); await flushPromises()
    expect(w.find('[role="alert"]').exists()).toBe(true)
    expect(w.text()).not.toContain('biNative.miniProgramDisabled')
    expect(api.list).not.toHaveBeenCalled()
    await w.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(api.list).toHaveBeenCalledWith('identities/users', 1, {})
    w.unmount()
  })
  it('requests metadata only and renders execution fees without business content', async () => { api.list.mockImplementation(async(path:string)=>page(path==='report-tasks'?[{task_id:1,organization_id:'org',organization_name:'Name',kind:'week',status:'partial',total_tokens:'9007199254740993',actual_cost:1.2,content:'FORBIDDEN_BODY',sources:'FORBIDDEN_INPUT'}]:[]));const w=mount(BIOperationsView,{global});await w.findAll('button').find(b=>b.text()==='biNative.tasks')!.trigger('click');await flushPromises();expect(w.text()).toContain('9,007,199,254,740,993');expect(w.text()).not.toContain('FORBIDDEN');expect(api.list).toHaveBeenCalledWith('report-tasks',1,{kind:''});expect(api.list).toHaveBeenCalledWith('report-executions',1,{kind:''});w.unmount() })
  it('lets a failed identity region retry while healthy session metadata remains',async()=>{api.list.mockImplementation(async(path:string)=>{if(path==='identities/users')throw new Error('failure');return page(path==='identities/sessions'?[{id:'s',bi_user_id:'biu_visible',status:'active'}]:[])});const w=mount(BIOperationsView,{global});await w.findAll('button').find(b=>b.text()==='biNative.identities')!.trigger('click');await flushPromises();expect(w.text()).toContain('biu_visible');expect(w.text()).toContain('biNative.failed');w.unmount()})
})

describe('BI operations interactions', () => {
  it('shows complete server aggregates independently of loaded pages and refreshes them with the period filter', async () => {
    api.list.mockResolvedValue(page([{ task_id: 1, organization_name: 'One company' }]))
    const wrapper = mount(BIOperationsView, { global })
    await wrapper.get('#bi-tab-tasks').trigger('click'); await flushPromises()
    expect(wrapper.get('.bi-report-metrics').text()).toContain('25')
    expect(wrapper.get('.bi-report-metrics').text()).toContain('9,007,199,254,740,993')
    expect(wrapper.get('.bi-report-metrics').text()).toContain('1.2500000001')
    expect(api.reportStatistics).toHaveBeenCalledWith({ kind: '' })
    await wrapper.get('#bi-panel-tasks select').setValue('week'); await flushPromises()
    expect(api.reportStatistics).toHaveBeenLastCalledWith({ kind: 'week' })
    expect(api.list).toHaveBeenLastCalledWith('report-executions', 1, { kind: 'week' })
    wrapper.unmount()
  })

  it('does not replace statistics for a new filter with a late response from the previous filter', async () => {
    let finish!: (value: typeof statistics) => void
    api.reportStatistics.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    api.reportStatistics.mockResolvedValueOnce({ ...statistics, task_count: '7' })
    const wrapper = mount(BIOperationsView, { global })
    await wrapper.get('#bi-tab-tasks').trigger('click')
    await wrapper.get('#bi-panel-tasks select').setValue('month'); await flushPromises()
    finish({ ...statistics, task_count: '999' }); await flushPromises()
    expect(wrapper.get('.bi-report-metrics dd').text()).toBe('7')
    wrapper.unmount()
  })

  it('shows an aggregate failure without substituting a partial-page total and offers retry', async () => {
    api.reportStatistics.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = mount(BIOperationsView, { global })
    await wrapper.get('#bi-tab-tasks').trigger('click'); await flushPromises()
    expect(wrapper.find('.bi-report-metrics').exists()).toBe(false)
    await wrapper.get('#bi-panel-tasks [role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.get('.bi-report-metrics dd').text()).toBe('25')
    wrapper.unmount()
  })

  it('opens a standalone access route for the selected organization', async () => {
    api.list.mockResolvedValue(page([{ id: 'org-one', name: 'First company' }, { id: 'org-two', name: 'Second company' }]))
    const wrapper = mount(BIOperationsView, { global })
    await wrapper.get('#bi-tab-onboarding').trigger('click'); await flushPromises()
    await wrapper.findAll('tbody button')[1].trigger('click'); await flushPromises()
    expect(router.push).toHaveBeenCalledWith({ path: '/admin/desktop/organizations/org-two/bi', query: { from: 'onboarding' } })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
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
