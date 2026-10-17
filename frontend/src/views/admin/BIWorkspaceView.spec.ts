import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter, RouterView, type Router } from 'vue-router'
import { biRoutes } from '@/router/biRoutes'
import type { BIIssuedInvitation } from '@/api/bi'

const api = vi.hoisted(() => ({ users: vi.fn(), invitations: vi.fn(), create: vi.fn(), access: vi.fn(), revoke: vi.fn(), qrcode: vi.fn() }))
const deps = vi.hoisted(() => ({ organization: vi.fn(), ownerOrganization: vi.fn(), enabled: vi.fn(), management: vi.fn(() => api) }))
vi.mock('@/api/bi', async original => ({ ...await original<typeof import('@/api/bi')>(), biManagement: deps.management, isBIEnabled: deps.enabled }))
vi.mock('@/api/admin/desktop', () => ({ getOrganization: deps.organization, default: { getOrganization: deps.organization } }))
vi.mock('@/api/desktopOrganization', () => ({ default: { getOrganization: deps.ownerOrganization } }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const base = '/admin/desktop/organizations/org/bi'
const user = { bi_user_id: 'biu_reader', display_name: 'Reader', organization_alias: 'Manager', user_status: 'active', status: 'active', capabilities: ['analytics:read'], revision: 3 }
const invitation: BIIssuedInvitation = { id: 'invite-one', organization_id: 'org', organization_name: 'Company', capabilities: ['analytics:read'], note: '', status: 'pending', created_at: '', expires_at: '2099-01-01T00:00:00Z', claimed_by: null, claimed_at: null, token: 'bii_secret', scene: 's'.repeat(32), path: '/legacy' }
const page = (items: unknown[] = [], has_more = false) => ({ items, page: 1, page_size: 20, has_more, total: items.length })
const wrappers: ReturnType<typeof mount>[] = []
const global = { stubs: {
  AppLayout: { template: '<main><slot/></main>' },
  BaseDialog: { props: ['show'], template: '<div v-if="show" role="dialog"><slot/><slot name="footer"/></div>' },
  TotpStepUpDialog: { props: ['controller'], template: '<div v-if="controller.visible.value"><button data-verify @click="controller.onVerified()">verify</button><button data-cancel @click="controller.onCancel()">cancel</button></div>' },
} }
async function settle() { await flushPromises(); await flushPromises() }
async function waitPath(router: Router, path: string) { await vi.waitFor(() => expect(router.currentRoute.value.path).toBe(path)); await settle() }
async function render(path = base) {
  const router = createRouter({ history: createMemoryHistory(), routes: [...biRoutes, { path: '/admin/desktop/operations', component: { template: '<p>Operations</p>' } }, { path: '/desktop/organization', component: { template: '<p>Organization</p>' } }] })
  await router.push(path); await router.isReady()
  const wrapper = mount(RouterView, { attachTo: document.body, global: { ...global, plugins: [router] } })
  wrappers.push(wrapper); await settle()
  return { wrapper, router }
}
async function navigate(router: Router, path: string) { await router.push(path); await settle() }
beforeEach(() => {
  vi.clearAllMocks()
  deps.organization.mockResolvedValue({ public_id: 'org', name: 'Company', status: 'active' })
  deps.ownerOrganization.mockResolvedValue({ public_id: 'org', name: 'My company', status: 'active' })
  deps.enabled.mockResolvedValue(true)
  api.users.mockResolvedValue(page([user])); api.invitations.mockResolvedValue(page())
  api.create.mockResolvedValue(invitation); api.access.mockResolvedValue(user)
  api.qrcode.mockResolvedValue({ mime_type: 'image/png', image_base64: 'aW1hZ2U=' })
})
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.restoreAllMocks() })

describe('Mini-program access routes', () => {
  it('creates on its own route and replaces the form with an invitation result route without persisting credentials', async () => {
    const { wrapper, router } = await render(`${base}?from=identities`)
    await wrapper.findAll('button').find(b => b.text() === 'biNative.createInvite')!.trigger('click'); await settle()
    await waitPath(router, `${base}/invitations/new`)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    await wrapper.get('input[maxlength="80"]').setValue('张经理')
    await wrapper.findAll('input[type="checkbox"]')[4].setValue(true)
    await wrapper.get('form').trigger('submit'); await settle()
    expect(api.create).toHaveBeenCalledWith(['analytics:read', 'members:read', 'reports:read', 'reports:copy'], '', '张经理')
    await waitPath(router, `${base}/invitations/invite-one`)
    expect(router.currentRoute.value.query).toEqual({ from: 'identities' })
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('img').exists()).toBe(true)
    expect(router.currentRoute.value.fullPath).not.toContain(invitation.scene)
    expect(router.currentRoute.value.fullPath).not.toContain(invitation.token)
    await navigate(router, base)
    await navigate(router, `${base}/invitations/invite-one`)
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toContain('biNative.inviteCredentialsHint')
    expect(api.create).toHaveBeenCalledTimes(1)
  })

  it('keeps the created invitation when official code generation fails and retries only the code', async () => {
    api.qrcode.mockRejectedValueOnce({ status: 503 })
    const { wrapper, router } = await render(`${base}/invitations/new`)
    await wrapper.get('form').trigger('submit'); await settle()
    await waitPath(router, `${base}/invitations/invite-one`)
    expect(wrapper.text()).toContain('biNative.qrGenerationFailed')
    await wrapper.findAll('button').find(b => b.text() === 'biNative.retryQRCode')!.trigger('click'); await settle()
    expect(api.create).toHaveBeenCalledTimes(1)
    expect(api.qrcode).toHaveBeenCalledTimes(2)
  })

  it('guards cancel and browser navigation against discarding unsaved input', async () => {
    const { wrapper, router } = await render(`${base}/invitations/new`)
    await wrapper.get('input[maxlength="100"]').setValue('Draft')
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    await wrapper.findAll('button').find(b => b.text() === 'common.cancel')!.trigger('click'); await settle()
    expect(router.currentRoute.value.path).toContain('/new')
    expect((wrapper.get('input[maxlength="100"]').element as HTMLInputElement).value).toBe('Draft')
    const unload = new Event('beforeunload', { cancelable: true }); window.dispatchEvent(unload)
    expect(unload.defaultPrevented).toBe(true)
    confirm.mockReturnValue(true)
    await navigate(router, base)
    expect(wrapper.find('form').exists()).toBe(false)
    expect(api.create).not.toHaveBeenCalled()
  })

  it('keeps a draft after verification cancellation, then retries the same create after verification', async () => {
    api.create.mockRejectedValueOnce({ status: 403, code: 'STEP_UP_REQUIRED' })
    const { wrapper } = await render(`${base}/invitations/new`)
    await wrapper.get('input[maxlength="100"]').setValue('Draft')
    await wrapper.get('form').trigger('submit'); await settle()
    await wrapper.get('[data-cancel]').trigger('click'); await settle()
    expect((wrapper.get('input[maxlength="100"]').element as HTMLInputElement).value).toBe('Draft')
    expect(document.activeElement).toBe(wrapper.get('input[maxlength="100"]').element)
    api.create.mockRejectedValueOnce({ status: 403, code: 'STEP_UP_REQUIRED' })
    await wrapper.get('form').trigger('submit'); await settle()
    await wrapper.get('[data-verify]').trigger('click'); await settle()
    expect(api.create).toHaveBeenCalledTimes(3)
    expect(api.create.mock.calls[1]).toEqual(api.create.mock.calls[2])
    expect(wrapper.find('img').exists()).toBe(true)
  })

  it('loads a direct edit URL across paginated users, preserves conflicts, and saves the loaded revision', async () => {
    api.users.mockResolvedValueOnce(page([], true)).mockResolvedValueOnce(page([user]))
    api.access.mockRejectedValueOnce({ status: 409 })
    const { wrapper, router } = await render(`${base}/users/biu_reader/edit`)
    expect(api.users).toHaveBeenCalledWith(2)
    expect((wrapper.get('input[maxlength="80"]').element as HTMLInputElement).value).toBe('Manager')
    await wrapper.get('input[maxlength="80"]').setValue('新称呼')
    await wrapper.get('form').trigger('submit'); await settle()
    expect(api.access).toHaveBeenCalledWith(user, ['analytics:read'], 'active', '新称呼')
    expect(wrapper.text()).toContain('biNative.conflict')
    expect((wrapper.get('input[maxlength="80"]').element as HTMLInputElement).value).toBe('新称呼')
    await wrapper.get('form').trigger('submit'); await settle()
    expect(router.currentRoute.value.path).toBe(base)
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('uses owner APIs and returns to the owner access list', async () => {
    const { wrapper, router } = await render('/desktop/organization/bi/users/biu_reader/edit')
    expect(deps.ownerOrganization).toHaveBeenCalled()
    expect(deps.organization).not.toHaveBeenCalled()
    expect(deps.management).toHaveBeenCalledWith('org', true)
    await wrapper.get('form').trigger('submit'); await settle()
    expect(router.currentRoute.value.path).toBe('/desktop/organization/bi')
  })

  it('does not show a form for unavailable access or a disabled service', async () => {
    api.users.mockResolvedValueOnce(page())
    const { wrapper } = await render(`${base}/users/missing/edit`)
    expect(wrapper.text()).toContain('biNative.accessNotFound')
    expect(wrapper.find('form').exists()).toBe(false)
    deps.enabled.mockResolvedValueOnce(false)
    const other = await render(`${base}/invitations/new`)
    expect(other.wrapper.text()).toContain('biNative.disabled')
    expect(other.wrapper.find('form').exists()).toBe(false)
  })

  it('returns to the originating operations tab', async () => {
    const { wrapper, router } = await render(`${base}?from=onboarding`)
    await wrapper.get('header a').trigger('click'); await settle()
    expect(router.currentRoute.value.fullPath).toBe('/admin/desktop/operations?tab=onboarding')
  })

  it('shows a safe explanation when a result URL is loaded directly', async () => {
    const { wrapper } = await render(`${base}/invitations/invite-one`)
    expect(wrapper.text()).toContain('biNative.inviteCredentialsHint')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(api.create).not.toHaveBeenCalled()
    expect(api.qrcode).not.toHaveBeenCalled()
  })
})
