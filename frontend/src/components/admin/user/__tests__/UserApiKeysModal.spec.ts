import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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
