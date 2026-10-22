import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'

const api = vi.hoisted(() => ({ organization: vi.fn(), all: vi.fn(), member: vi.fn(), assign: vi.fn(), history: vi.fn(), unit: vi.fn(), save: vi.fn(), status: vi.fn(), list: vi.fn(), members: { createMember: vi.fn(), updateMember: vi.fn() } }))
const navigation = vi.hoisted(() => ({ push: vi.fn(), resolve: vi.fn(), guards: [] as Array<() => boolean> }))
const route = reactive({ path: '/admin/desktop/organizations/org_one/members/new', fullPath: '/admin/desktop/organizations/org_one/members/new', params: { organizationId: 'org_one', memberId: '', unitId: '' }, query: {} as Record<string, string> })
vi.mock('@/api/desktopStructure', async original => ({ ...await original<typeof import('@/api/desktopStructure')>(), desktopStructure: () => api }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 42 } }) }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => navigation, onBeforeRouteLeave: (guard: () => boolean) => navigation.guards.push(guard), onBeforeRouteUpdate: vi.fn() }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }) }))

import MemberForm from './DesktopMemberFormView.vue'
import PositionForm from './DesktopMemberPositionView.vue'
import UnitForm from './DesktopUnitFormView.vue'
import StructureList from './DesktopStructureView.vue'

const department = { public_id: 'dep_one', name: 'Delivery', status: 'active', revision: 1 }
const position = { public_id: 'pos_one', name: 'Engineer', status: 'active', revision: 1, department }
const member = { public_id: 'mem_one', name: 'Member', phone: '+8613800138000', remark: 'Original', position, department, position_revision: 2, budget: { quota: 12, limit_1d: 2, limit_7d: 8 } }
const options = { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, RouterLink: { props: ['to'], template: '<a><slot /></a>' }, Pagination: true } } }

describe('Desktop structure routed forms', () => {
  beforeEach(() => {
    vi.clearAllMocks(); navigation.guards.length = 0
    navigation.resolve.mockImplementation(({ path, query }) => ({ fullPath: path + '?' + new URLSearchParams(Object.entries(query).filter(([, value]) => value !== undefined) as [string, string][]).toString() }))
    api.status.mockResolvedValue({ unassigned_members: 0, inconsistent_members: 0 })
    api.list.mockResolvedValue({ items: [department], total: 1 })
    route.params = { organizationId: 'org_one', memberId: '', unitId: '' }; route.query = {}
    route.path = route.fullPath = '/admin/desktop/organizations/org_one/members/new'
    api.organization.mockResolvedValue({ public_id: 'org_one', name: 'Enterprise', status: 'active', member_count: 1, member_limit: 10 })
    api.all.mockImplementation(async (kind: string) => kind === 'departments' ? [department, { ...department, public_id: 'dep_two', name: 'Sales' }] : [position])
    api.member.mockResolvedValue({ ...member }); api.unit.mockResolvedValue({ ...position })
    api.history.mockResolvedValue({ items: [], total: 0 }); api.assign.mockResolvedValue({ ...member }); api.save.mockResolvedValue({ ...position })
    api.members.createMember.mockResolvedValue({ ...member }); api.members.updateMember.mockResolvedValue({ ...member })
    vi.spyOn(window, 'confirm').mockReturnValue(false)
  })

  it('creates a member with the explicitly selected position and clears it when the department changes', async () => {
    const wrapper = mount(MemberForm, options); await flushPromises()
    await wrapper.get('input[maxlength="100"]').setValue('Created')
    await wrapper.get('input[type="tel"]').setValue('13800138000')
    const selects = wrapper.findAll('select')
    await selects[0].setValue('dep_one'); await selects[1].setValue('pos_one')
    await selects[0].setValue('dep_two'); expect((selects[1].element as HTMLSelectElement).value).toBe('')
    await selects[0].setValue('dep_one'); await selects[1].setValue('pos_one')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.members.createMember).toHaveBeenCalledWith('org_one', expect.objectContaining({ name: 'Created', position_id: 'pos_one' }), expect.any(String))
    expect(navigation.push).toHaveBeenCalledWith('/admin/desktop/organizations/org_one?tab=members')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('loads the full phone and preserves budgets and credentials for unchanged edits', async () => {
    route.params.memberId = 'mem_one'
    const wrapper = mount(MemberForm, options); await flushPromises()
    expect((wrapper.get('input[type="tel"]').element as HTMLInputElement).value).toBe(member.phone)
    expect(wrapper.findAll('input[type="number"]').map(input => (input.element as HTMLInputElement).value)).toEqual(['12', '2', '8'])
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.members.updateMember).not.toHaveBeenCalled()
    await wrapper.get('textarea').setValue('')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.members.updateMember).toHaveBeenCalledWith('org_one', 'mem_one', { remark: '' })
    wrapper.unmount()
  })

  it('keeps the same idempotency key for an unchanged creation retry', async () => {
    api.members.createMember.mockRejectedValue(new Error('Temporary failure'))
    const wrapper = mount(MemberForm, options); await flushPromises()
    await wrapper.get('input[maxlength="100"]').setValue('Created'); await wrapper.get('input[type="tel"]').setValue('13800138000')
    await wrapper.findAll('select')[0].setValue('dep_one'); await wrapper.findAll('select')[1].setValue('pos_one')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    const key = api.members.createMember.mock.calls[0][2]
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.members.createMember.mock.calls[1][2]).toBe(key)
    await wrapper.get('input[maxlength="100"]').setValue('Renamed')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.members.createMember.mock.calls[2][2]).not.toBe(key)
    expect(navigation.guards[0]()).toBe(false)
    wrapper.unmount()
  })

  it('retains the selected target on a stale assignment and only uses the loaded revision', async () => {
    route.params.memberId = 'mem_one'; api.assign.mockRejectedValue({ reason: 'ORGANIZATION_STRUCTURE_CONFLICT', message: 'Reload required' })
    const wrapper = mount(PositionForm, options); await flushPromises()
    await wrapper.get('textarea').setValue('Move confirmed')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.assign).toHaveBeenCalledWith('mem_one', { position_id: 'pos_one', expected_revision: 2, reason: 'Move confirmed' }, expect.any(String))
    expect((wrapper.findAll('select')[1].element as HTMLSelectElement).value).toBe('pos_one')
    expect(wrapper.get('[role="alert"]').text()).toContain('desktopStructure.conflict')
    expect(navigation.push).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not send the immutable department when editing a position', async () => {
    route.params.unitId = 'pos_one'
    const wrapper = mount(UnitForm, { ...options, props: { kind: 'positions' as const } }); await flushPromises()
    await wrapper.get('input').setValue('Renamed')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.save).toHaveBeenCalledWith('positions', 'pos_one', { name: 'Renamed', status: 'active', expected_revision: 1 }, expect.any(String))
    expect(wrapper.get('select').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('keeps the search field mounted and focused while filtered results load', async () => {
    vi.useFakeTimers()
    const wrapper = mount(StructureList, { ...options, attachTo: document.body }); await flushPromises()
    const input = wrapper.get('input[type="search"]')
    let resolve!: (value: object) => void
    api.list.mockReturnValueOnce(new Promise(done => { resolve = done }))
    ;(input.element as HTMLInputElement).focus()
    await input.setValue('Delivery')
    await vi.advanceTimersByTimeAsync(300)
    expect(wrapper.get('input[type="search"]').element).toBe(input.element)
    expect(document.activeElement).toBe(input.element)
    resolve({ items: [], total: 0 }); await flushPromises()
    expect(wrapper.text()).toContain('desktopStructure.noResultsHint')
    expect((input.element as HTMLInputElement).value).toBe('Delivery')
    wrapper.unmount(); vi.useRealTimers()
  })

  it('keeps filters in links back to the organization structure list', async () => {
    route.path = '/admin/desktop/organizations/org_one/structure'
    route.query = { kind: 'positions', search: 'Engineer', department_id: 'dep_one', status: 'active' }
    const wrapper = mount(StructureList, options); await flushPromises()
    expect(navigation.resolve).toHaveBeenCalledWith({ path: route.path, query: { ...route.query, page: '1' } })
    expect(api.list).toHaveBeenCalledWith('positions', expect.objectContaining({ search: 'Engineer', department_id: 'dep_one', status: 'active' }), expect.any(AbortSignal))
    wrapper.unmount()
  })

  it('does not present incomplete assignments as fully confirmed', async () => {
    api.status.mockResolvedValue({ unassigned_members: 0, inconsistent_members: 1 })
    const wrapper = mount(StructureList, options); await flushPromises()
    expect(wrapper.text()).toContain('desktopStructure.inconsistent')
    expect(wrapper.text()).not.toContain('desktopStructure.ready')
    wrapper.unmount()
  })

  it('retries an initial load failure without asking to discard input that does not exist', async () => {
    api.organization.mockRejectedValueOnce(new Error('Network unavailable'))
    const wrapper = mount(MemberForm, options); await flushPromises()
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(window.confirm).not.toHaveBeenCalled()
    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.findAll('main')).toHaveLength(1)
    wrapper.unmount()
  })

  it('retries history without replacing a drafted reassignment', async () => {
    route.params.memberId = 'mem_one'
    api.history.mockResolvedValue({ items: [], total: 21 })
    const wrapper = mount(PositionForm, options); await flushPromises()
    await wrapper.get('textarea').setValue('Keep this explanation')
    api.history.mockRejectedValueOnce(new Error('History unavailable'))
    wrapper.findComponent({ name: 'Pagination' }).vm.$emit('update:page', 2)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('desktopStructure.historyLoadFailed')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(api.history).toHaveBeenLastCalledWith('mem_one', 2)
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('Keep this explanation')
    expect(navigation.guards[0]()).toBe(false)
    wrapper.unmount()
  })

  it('keeps submit focus during a pending save and rejects repeated submissions', async () => {
    let finish!: (value: object) => void
    api.members.createMember.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = mount(MemberForm, { ...options, attachTo: document.body }); await flushPromises()
    const submit = wrapper.get('button[type="submit"]')
    ;(submit.element as HTMLButtonElement).focus()
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(api.members.createMember).toHaveBeenCalledTimes(1)
    expect(submit.attributes('aria-disabled')).toBe('true')
    expect(document.activeElement).toBe(submit.element)
    finish({}); await flushPromises()
    wrapper.unmount()
  })

  it('focuses the new page heading when nested creation reuses the route component', async () => {
    const wrapper = mount(UnitForm, { ...options, attachTo: document.body, props: { kind: 'positions' as const } }); await flushPromises()
    ;(wrapper.get('input').element as HTMLInputElement).focus()
    route.path = '/admin/desktop/organizations/org_one/departments/new'
    await wrapper.setProps({ kind: 'departments' }); await flushPromises()
    expect(document.activeElement).toBe(wrapper.get('h1').element)
    wrapper.unmount()
  })

  it('keeps reassignment available when the initial history request fails', async () => {
    route.params.memberId = 'mem_one'
    api.history.mockRejectedValueOnce(new Error('History unavailable'))
    const wrapper = mount(PositionForm, options); await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.get('[role="alert"]').text()).toContain('desktopStructure.historyLoadFailed')
    await wrapper.get('textarea').setValue('Retain my explanation')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('Retain my explanation')
    expect(api.member).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('renders reassignment before a slow history request finishes', async () => {
    route.params.memberId = 'mem_one'
    let finish!: (value: object) => void
    api.history.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const wrapper = mount(PositionForm, options); await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('desktopStructure.historyUnknown')
    finish({ items: [], total: 0 }); await flushPromises(); wrapper.unmount()
  })
  it('retries structure options without replacing the member draft', async () => {
    api.all.mockRejectedValueOnce(new Error('Options unavailable'))
    const wrapper = mount(MemberForm, options); await flushPromises()
    await wrapper.get('input[maxlength="100"]').setValue('Keep my draft')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).not.toContain('desktopStructure.noDepartments')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect((wrapper.get('input[maxlength="100"]').element as HTMLInputElement).value).toBe('Keep my draft')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    expect(api.organization).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('refreshes only results when filters change and cancels the prior list request', async () => {
    vi.useFakeTimers()
    route.path = '/admin/desktop/organizations/org_one/structure'; route.query = { kind: 'positions' }
    const wrapper = mount(StructureList, options); await flushPromises()
    const firstSignal = api.list.mock.calls[0][2] as AbortSignal
    await wrapper.get('input[type="search"]').setValue('New query')
    await vi.advanceTimersByTimeAsync(300); await flushPromises()
    expect(firstSignal.aborted).toBe(true)
    expect(api.list).toHaveBeenCalledTimes(2)
    expect(api.organization).toHaveBeenCalledTimes(1)
    expect(api.status).toHaveBeenCalledTimes(1)
    expect(api.all).toHaveBeenCalledTimes(1)
    wrapper.unmount(); vi.useRealTimers()
  })
  it('preserves the page in edit return links and clamps an out-of-range restored page', async () => {
    route.query = { page: '3', kind: 'positions' }
    api.list.mockResolvedValue({ items: [position], total: 60 })
    const wrapper = mount(StructureList, options); await flushPromises()
    expect(api.list).toHaveBeenCalledWith('positions', expect.objectContaining({ page: 3 }), expect.any(AbortSignal))
    expect(navigation.resolve).toHaveBeenCalledWith(expect.objectContaining({ query: expect.objectContaining({ page: '3' }) }))
    wrapper.unmount()
    api.list.mockClear().mockResolvedValue({ items: [position], total: 21 })
    const restored = mount(StructureList, options); await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith('positions', expect.objectContaining({ page: 2 }), expect.any(AbortSignal))
    expect(navigation.resolve).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ page: '2' }) }))
    restored.unmount()
  })
  it('does not fetch department options on the department tab', async () => {
    const wrapper = mount(StructureList, options); await flushPromises()
    expect(api.all).not.toHaveBeenCalled()
    wrapper.unmount()
  })

})
