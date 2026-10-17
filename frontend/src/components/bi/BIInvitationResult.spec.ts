import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { BIIssuedInvitation } from '@/api/bi'
import BIInvitationResult from './BIInvitationResult.vue'

const api = vi.hoisted(() => ({ qrcode: vi.fn() }))
const management = vi.hoisted(() => vi.fn(() => api))
vi.mock('@/api/bi', () => ({ biManagement: management }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
const global = { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/></div>' }, TotpStepUpDialog: { props: ['controller'], template: '<div v-if="controller.visible.value"><button data-verify @click="controller.onVerified()">verify</button><button data-cancel @click="controller.onCancel()">cancel</button></div>' } } }
const invitation: BIIssuedInvitation = {
  id: 'invite-one', organization_id: 'org', organization_name: '测试企业', capabilities: ['analytics:read'],
  note: '', status: 'pending', expires_at: '2099-01-01T00:00:00Z', created_at: '2026-10-17T00:00:00Z',
  claimed_by: null, claimed_at: null, token: 'bii_manual-secret', scene: 's'.repeat(32), path: '/legacy-path',
}
const result = { mime_type: 'image/png', image_base64: 'aW1hZ2U=' }
const wrappers: ReturnType<typeof mount>[] = []
function render(overrides: Partial<BIIssuedInvitation> = {}) {
  const wrapper = mount(BIInvitationResult, { props: { organizationId: 'org', invitation: { ...invitation, ...overrides } }, global })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => { vi.resetAllMocks(); management.mockReturnValue(api); api.qrcode.mockResolvedValue(result) })
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers(); vi.unstubAllGlobals() })

describe('Official WeChat invitation result', () => {
  it('requests the official code, enables saving after image load, and copies only the manual token', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const wrapper = render(); await flushPromises()
    expect(api.qrcode).toHaveBeenCalledWith(invitation.id, invitation.scene)
    expect(wrapper.get('img').attributes('src')).toBe('data:image/png;base64,aW1hZ2U=')
    expect(wrapper.find('a[download]').exists()).toBe(false)
    await wrapper.get('img').trigger('load')
    expect(wrapper.get('a[download]').attributes('download')).toBe('wechat-invitation.png')
    await wrapper.get('button').trigger('click'); await flushPromises()
    expect(writeText).toHaveBeenCalledWith(invitation.token)
    expect(wrapper.text()).not.toContain(invitation.scene)
    expect(wrapper.text()).not.toContain(invitation.path)
  })

  it('keeps the manual code and retries the same invitation after upstream failure', async () => {
    api.qrcode.mockRejectedValueOnce({ status: 502 })
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('biNative.qrGenerationFailed')
    expect((wrapper.get('input').element as HTMLInputElement).value).toBe(invitation.token)
    await wrapper.findAll('button')[1].trigger('click'); await flushPromises()
    expect(api.qrcode.mock.calls).toEqual([[invitation.id, invitation.scene], [invitation.id, invitation.scene]])
    expect(wrapper.find('img').exists()).toBe(true)
  })

  it('honors Retry-After before enabling retry', async () => {
    vi.useFakeTimers()
    api.qrcode.mockRejectedValueOnce({ status: 429, retryAfter: '2' })
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeDefined()
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeUndefined()
    await wrapper.findAll('button')[1].trigger('click'); await flushPromises()
    expect(api.qrcode).toHaveBeenCalledTimes(2)
  })

  it('does not request another code when the invitation list refreshes unchanged metadata', async () => {
    api.qrcode.mockRejectedValueOnce({ status: 503 })
    const wrapper = render(); await flushPromises()
    await wrapper.setProps({ invitation: { ...invitation, note: 'Updated note' } }); await flushPromises()
    expect(api.qrcode).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('biNative.qrGenerationFailed')
  })

  it('retries after step-up and allows retry after verification cancellation', async () => {
    api.qrcode.mockRejectedValueOnce({ status: 403, code: 'STEP_UP_REQUIRED' })
    const wrapper = render(); await flushPromises()
    await wrapper.get('[data-cancel]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('biNative.qrVerificationCancelled')
    api.qrcode.mockRejectedValueOnce({ status: 403, code: 'STEP_UP_REQUIRED' })
    await wrapper.findAll('button')[1].trigger('click'); await flushPromises()
    await wrapper.get('[data-verify]').trigger('click'); await flushPromises()
    expect(api.qrcode).toHaveBeenCalledTimes(3)
    expect(api.qrcode.mock.calls.every(call => call[0] === invitation.id && call[1] === invitation.scene)).toBe(true)
    expect(wrapper.find('img').exists()).toBe(true)
  })

  it('discards a late response after switching organizations', async () => {
    let resolve!: (value: typeof result) => void
    api.qrcode.mockReturnValueOnce(new Promise(r => { resolve = r }))
    const wrapper = render(); await flushPromises()
    await wrapper.setProps({ organizationId: 'other', invitation: { ...invitation, id: 'invite-two', scene: 'n'.repeat(32) }, selfManaged: true })
    await flushPromises()
    resolve({ ...result, image_base64: 'c3RhbGU=' }); await flushPromises()
    expect(management).toHaveBeenLastCalledWith('other', true)
    expect(wrapper.get('img').attributes('src')).not.toContain('c3RhbGU=')
    expect(wrapper.emitted('busyChange')?.at(-1)).toEqual([false])
  })

  it.each(['used', 'revoked', 'expired'])('hides credentials when an invitation becomes %s', async status => {
    const wrapper = render(); await flushPromises()
    await wrapper.setProps({ invitation: { ...invitation, status } })
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.text()).toContain('biNative.invitationUnavailable')
  })

  it('does not open verification for a stale request after switching organizations', async () => {
    let reject!: (error: unknown) => void
    api.qrcode.mockReturnValueOnce(new Promise((_resolve, r) => { reject = r }))
    const wrapper = render(); await flushPromises()
    await wrapper.setProps({ organizationId: 'other', invitation: { ...invitation, id: 'invite-two' } })
    await flushPromises()
    reject({ status: 403, code: 'STEP_UP_REQUIRED' }); await flushPromises()
    expect(wrapper.find('[data-verify]').exists()).toBe(false)
    expect(wrapper.find('img').exists()).toBe(true)
  })

  it('hides credentials when they expire while the page remains open', async () => {
    vi.useFakeTimers()
    const wrapper = render({ expires_at: new Date(Date.now() + 2000).toISOString() }); await flushPromises()
    expect(wrapper.find('img').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(false)
  })

  it('removes credentials on an unavailable invitation response', async () => {
    api.qrcode.mockRejectedValueOnce({ status: 404 })
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.emitted('busyChange')?.at(-1)).toEqual([false])
  })

  it('keeps manual fallback for legacy invitations without a scene', async () => {
    const wrapper = render({ scene: '' }); await flushPromises()
    expect(api.qrcode).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('biNative.qrCredentialMissing')
    expect(wrapper.findAll('button')).toHaveLength(1)
    expect(wrapper.find('input').exists()).toBe(true)
  })

  it('reports a broken image and supports a JPEG retry', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.get('img').trigger('error')
    expect(wrapper.find('a[download]').exists()).toBe(false)
    expect(wrapper.text()).toContain('biNative.qrGenerationFailed')
    api.qrcode.mockResolvedValueOnce({ ...result, mime_type: 'image/jpeg' })
    await wrapper.findAll('button')[1].trigger('click'); await flushPromises()
    await wrapper.get('img').trigger('load')
    expect(wrapper.get('a[download]').attributes('download')).toBe('wechat-invitation.jpg')
  })
})
