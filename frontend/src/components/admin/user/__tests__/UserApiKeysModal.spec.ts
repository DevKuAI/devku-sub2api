import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import UserApiKeysModal from '../UserApiKeysModal.vue'

const { getUserApiKeys, getAllGroups, updateApiKeyGroup } = vi.hoisted(() => ({
  getUserApiKeys: vi.fn(),
  getAllGroups: vi.fn(),
  updateApiKeyGroup: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { getUserApiKeys },
    groups: { getAll: getAllGroups },
    apiKeys: { updateApiKeyGroup },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const createApiKey = (id: number, managedBy?: string) => ({
  id,
  key: `sk-test-${id}`,
  name: `key-${id}`,
  display_name: managedBy === 'desktop' ? 'Desktop member' : undefined,
  managed_by: managedBy,
  group_id: null,
  status: 'active',
  created_at: '2026-09-04T00:00:00Z',
})

const mountAndOpen = async () => {
  const wrapper = mount(UserApiKeysModal, {
    props: {
      show: false,
      user: { id: 1, email: 'user@example.com', username: 'user' } as any,
    },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /></div>',
        },
        GroupBadge: true,
        GroupOptionItem: true,
        Teleport: true,
      },
    },
  })

  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

enableAutoUnmount(afterEach)

describe('UserApiKeysModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getUserApiKeys.mockResolvedValue({
      items: [createApiKey(1, 'desktop'), createApiKey(2)],
    })
    getAllGroups.mockResolvedValue([])
  })

  it('disables group editing only for desktop-managed API keys', async () => {
    const wrapper = await mountAndOpen()
    const groupButtons = wrapper.findAll('button')

    expect(groupButtons).toHaveLength(2)
    expect(groupButtons[0].attributes('disabled')).toBeDefined()
    expect(groupButtons[1].attributes('disabled')).toBeUndefined()

    await groupButtons[0].trigger('click')
    expect((wrapper.vm as any).groupSelectorKeyId).toBeNull()

    await groupButtons[1].trigger('click')
    expect((wrapper.vm as any).groupSelectorKeyId).toBe(2)
    expect(updateApiKeyGroup).not.toHaveBeenCalled()
  })
})

function deferred() {
  let resolve!: (value: unknown) => void
  let reject!: (error: Error) => void
  const promise = new Promise((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
const user = (id: number) => ({ id, email: `user${id}@example.com`, username: `user${id}` }) as any
const keys = (id: number, name: string) => ({ items: [{ id, name, key: 'sk-example-key-value-for-tests', status: 'active', created_at: '2026-09-20', group_id: null }] })

async function switchUser(wrapper: Awaited<ReturnType<typeof mountAndOpen>>) {
  await wrapper.setProps({ show: false })
  await wrapper.setProps({ show: true, user: user(2) })
}

describe('user API key loading', () => {
  beforeEach(() => {
    getUserApiKeys.mockReset()
    getAllGroups.mockResolvedValue([])
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => vi.restoreAllMocks())
  it('does not display the previous user keys when the next load fails', async () => {
    getUserApiKeys.mockResolvedValueOnce(keys(1, 'first-user-key')).mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = await mountAndOpen(); await flushPromises()
    expect(wrapper.text()).toContain('first-user-key')
    await switchUser(wrapper); await flushPromises()
    expect(wrapper.text()).toContain('user2@example.com')
    expect(wrapper.text()).not.toContain('first-user-key')
  })

  it('does not replace current keys with a late previous response', async () => {
    const old = deferred()
    getUserApiKeys.mockReturnValueOnce(old.promise).mockResolvedValueOnce(keys(2, 'current-user-key'))
    const wrapper = await mountAndOpen()
    await switchUser(wrapper); await flushPromises()
    old.resolve(keys(1, 'old-user-key')); await flushPromises()
    expect(wrapper.text()).toContain('current-user-key')
    expect(wrapper.text()).not.toContain('old-user-key')
  })

  it('keeps the current request loading when an obsolete request fails', async () => {
    const old = deferred(); const current = deferred()
    getUserApiKeys.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = await mountAndOpen()
    await switchUser(wrapper)
    old.reject(new Error('obsolete')); await flushPromises()
    expect(wrapper.find('.animate-spin').exists()).toBe(true)
    current.resolve(keys(2, 'current-user-key')); await flushPromises()
    expect(wrapper.text()).toContain('current-user-key')
    expect(wrapper.find('.animate-spin').exists()).toBe(false)
  })

  it('loads keys when the selected user changes while the dialog is open', async () => {
    getUserApiKeys.mockResolvedValueOnce(keys(1, 'first-user-key')).mockResolvedValueOnce(keys(2, 'second-user-key'))
    const wrapper = await mountAndOpen(); await flushPromises()
    await wrapper.setProps({ user: user(2) }); await flushPromises()
    expect(getUserApiKeys).toHaveBeenLastCalledWith(2)
    expect(wrapper.text()).toContain('second-user-key')
    expect(wrapper.text()).not.toContain('first-user-key')
  })
})
