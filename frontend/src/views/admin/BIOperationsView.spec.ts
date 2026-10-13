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
  it('requests metadata only and renders execution fees without business content', async () => { api.list.mockImplementation(async(path:string)=>page(path==='report-tasks'?[{task_id:1,organization_id:'org',organization_name:'Name',kind:'week',status:'partial',total_tokens:'9007199254740993',actual_cost:1.2,content:'FORBIDDEN_BODY',sources:'FORBIDDEN_INPUT'}]:[]));const w=mount(BIOperationsView,{global});await w.findAll('button').find(b=>b.text()==='biNative.tasks')!.trigger('click');await flushPromises();expect(w.text()).toContain('9007199254740993');expect(w.text()).not.toContain('FORBIDDEN');expect(api.list).toHaveBeenCalledWith('report-tasks',1,{kind:''});expect(api.list).toHaveBeenCalledWith('report-executions',1,{kind:''});w.unmount() })
  it('lets a failed identity region retry while healthy session metadata remains',async()=>{api.list.mockImplementation(async(path:string)=>{if(path==='identities/users')throw new Error('failure');return page(path==='identities/sessions'?[{id:'s',bi_user_id:'biu_visible',status:'active'}]:[])});const w=mount(BIOperationsView,{global});await w.findAll('button').find(b=>b.text()==='biNative.identities')!.trigger('click');await flushPromises();expect(w.text()).toContain('biu_visible');expect(w.text()).toContain('biNative.failed');w.unmount()})
})
