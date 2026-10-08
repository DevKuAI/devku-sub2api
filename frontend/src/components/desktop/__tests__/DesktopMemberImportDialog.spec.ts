import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { saveAs } from 'file-saver'
import type { DesktopMemberImportRow } from '@/utils/desktopMemberImport'

const admin = vi.hoisted(() => ({ createMember: vi.fn() }))
const managed = vi.hoisted(() => ({ createMember: vi.fn() }))
const files = vi.hoisted(() => ({ parseDesktopMemberImportFile: vi.fn(), createDesktopMemberImportTemplate: vi.fn() }))
vi.mock('@/api/admin/desktop', () => admin)
vi.mock('@/api/desktopOrganization', () => ({ default: managed }))
vi.mock('@/utils/desktopMemberImport', () => files)
vi.mock('file-saver', () => ({ saveAs: vi.fn() }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, te: () => true }) }))
import DesktopMemberImportDialog from '../DesktopMemberImportDialog.vue'

const record = (index: number): DesktopMemberImportRow => ({ rowNumber: index + 3, input: { name: `Member ${index}`, phone: `+861380013800${index}`, remark: 'Team' }, errors: [] })
const view = (selfManaged = false, availableSlots = 10) => mount(DesktopMemberImportDialog, {
  props: { show: true, organizationId: 'org_one', selfManaged, availableSlots, organizationActive: true },
  global: { stubs: { BaseDialog: { template: '<div v-if="show"><slot /><slot name="footer" /></div>', props: ['show'] }, Icon: true } },
})
async function upload(wrapper: ReturnType<typeof view>) {
  const input = wrapper.get('input[type="file"]')
  Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['x'], 'members.xlsx')] })
  await input.trigger('change')
  await flushPromises()
}

describe('Desktop member import dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    admin.createMember.mockReset().mockResolvedValue({})
    managed.createMember.mockReset().mockResolvedValue({})
    files.parseDesktopMemberImportFile.mockReset().mockResolvedValue([record(0), record(1)])
    files.createDesktopMemberImportTemplate.mockReset().mockResolvedValue(new Blob(['template']))
  })

  it('offers a working template download before a file is selected', async () => {
    const wrapper = view()
    await wrapper.get('[data-testid="download-member-template"]').trigger('click')
    await flushPromises()
    expect(files.createDesktopMemberImportTemplate).toHaveBeenCalledOnce()
    expect(saveAs).toHaveBeenCalledWith(expect.any(Blob), 'desktop_members_template.xlsx')
    expect(admin.createMember).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it.each([false, true])('previews rows and creates valid members through the correct API for selfManaged=%s', async selfManaged => {
    files.parseDesktopMemberImportFile.mockResolvedValue([record(0), { ...record(1), errors: ['duplicatePhone'] }])
    const wrapper = view(selfManaged)
    await upload(wrapper)
    expect(wrapper.text()).toContain('Member 0')
    expect(wrapper.text()).toContain('duplicatePhone')
    expect(admin.createMember).not.toHaveBeenCalled()
    expect(managed.createMember).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="confirm-member-import"]').trigger('click')
    await flushPromises()
    const api = selfManaged ? managed : admin
    expect(api.createMember).toHaveBeenCalledOnce()
    expect(api.createMember).toHaveBeenCalledWith('org_one', record(0).input, expect.stringMatching(/^desktop-member-import-/))
    expect(wrapper.emitted('imported')).toHaveLength(1)
    expect(wrapper.text()).toContain('memberImport.success')
    expect(wrapper.get('[data-testid="confirm-member-import"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('retries only failed rows and preserves their idempotency keys', async () => {
    admin.createMember.mockRejectedValueOnce({ reason: 'MEMBER_LIMIT_REACHED' }).mockResolvedValue({})
    const wrapper = view()
    await upload(wrapper)
    await wrapper.get('[data-testid="confirm-member-import"]').trigger('click')
    await flushPromises()
    expect(admin.createMember).toHaveBeenCalledTimes(2)
    const firstKey = admin.createMember.mock.calls[0][2]
    expect(wrapper.text()).toContain('MEMBER_LIMIT_REACHED')
    await wrapper.setProps({ availableSlots: 0 })
    await wrapper.get('[data-testid="confirm-member-import"]').trigger('click')
    await flushPromises()
    expect(admin.createMember).toHaveBeenCalledTimes(3)
    expect(admin.createMember).toHaveBeenLastCalledWith('org_one', record(0).input, firstKey)
    wrapper.unmount()
  })

  it('blocks initial imports exceeding capacity or targeting disabled organizations', async () => {
    const wrapper = view(false, 1)
    await upload(wrapper)
    expect(wrapper.text()).toContain('capacityExceeded')
    expect(wrapper.get('[data-testid="confirm-member-import"]').attributes('disabled')).toBeDefined()
    await wrapper.setProps({ availableSlots: 2, organizationActive: false })
    expect(wrapper.get('[data-testid="confirm-member-import"]').attributes('disabled')).toBeDefined()
    expect(admin.createMember).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('stops subsequent writes and discards the old preview when the organization changes', async () => {
    let resolve!: (value: object) => void
    admin.createMember.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const wrapper = view()
    await upload(wrapper)
    await wrapper.get('[data-testid="confirm-member-import"]').trigger('click')
    expect(wrapper.get('input[type="file"]').attributes('disabled')).toBeDefined()
    await wrapper.setProps({ organizationId: 'org_two' })
    resolve({})
    await flushPromises()
    expect(admin.createMember).toHaveBeenCalledOnce()
    expect(wrapper.text()).not.toContain('Member 0')
    expect(wrapper.emitted('imported')).toBeUndefined()
    wrapper.unmount()
  })

  it('reports parse and template errors without writing members', async () => {
    files.parseDesktopMemberImportFile.mockRejectedValueOnce(new Error('missingHeaders'))
    files.createDesktopMemberImportTemplate.mockRejectedValueOnce(new Error('offline'))
    const wrapper = view()
    await upload(wrapper)
    expect(wrapper.get('[role="alert"]').text()).toContain('missingHeaders')
    await wrapper.get('[data-testid="download-member-template"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('templateFailed')
    expect(admin.createMember).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
