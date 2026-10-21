import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'

const api = vi.hoisted(() => ({ organization: vi.fn(), all: vi.fn(), member: vi.fn(), assign: vi.fn(), history: vi.fn(), unit: vi.fn(), save: vi.fn(), members: { createMember: vi.fn(), updateMember: vi.fn() } }))
const navigation = vi.hoisted(() => ({ push: vi.fn(), guards: [] as Array<() => boolean> }))
const route = reactive({ path: '/admin/desktop/organizations/org_one/members/new', fullPath: '/admin/desktop/organizations/org_one/members/new', params: { organizationId: 'org_one', memberId: '', unitId: '' }, query: {} as Record<string, string> })
vi.mock('@/api/desktopStructure', async original => ({ ...await original<typeof import('@/api/desktopStructure')>(), desktopStructure: () => api }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 42 } }) }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => navigation, onBeforeRouteLeave: (guard: () => boolean) => navigation.guards.push(guard), onBeforeRouteUpdate: vi.fn() }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

import MemberForm from './DesktopMemberFormView.vue'
import PositionForm from './DesktopMemberPositionView.vue'
import UnitForm from './DesktopUnitFormView.vue'

const department = { public_id: 'dep_one', name: 'Delivery', status: 'active', revision: 1 }
const position = { public_id: 'pos_one', name: 'Engineer', status: 'active', revision: 1, department }
const member = { public_id: 'mem_one', name: 'Member', phone: '+8613800138000', remark: 'Original', position, department, position_revision: 2, budget: { quota: 12, limit_1d: 2, limit_7d: 8 } }
const options = { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, RouterLink: { props: ['to'], template: '<a><slot /></a>' }, Pagination: true } } }

describe('Desktop structure routed forms', () => {
  beforeEach(() => {
    vi.clearAllMocks(); navigation.guards.length = 0
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
})
