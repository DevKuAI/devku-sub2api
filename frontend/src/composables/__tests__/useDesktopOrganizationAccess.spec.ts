import { beforeEach, describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  isAuthenticated: true,
  user: { id: 9 } as { id: number } | null,
}))
const getOrganization = vi.hoisted(() => vi.fn())

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/api/desktopOrganization', () => ({ default: { getOrganization } }))

import { useDesktopOrganizationAccess } from '@/composables/useDesktopOrganizationAccess'

describe('useDesktopOrganizationAccess', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authStore.isAuthenticated = true
    authStore.user = { id: 9 }
    useDesktopOrganizationAccess().resetDesktopOrganizationAccess()
  })

  it('keeps the menu unavailable when the account has no organization', async () => {
    getOrganization.mockResolvedValue(null)
    const access = useDesktopOrganizationAccess()

    await access.refreshDesktopOrganizationAccess()

    expect(access.canManageDesktopOrganization.value).toBe(false)
    expect(access.managedDesktopOrganization.value).toBeNull()
  })

  it('exposes the associated organization and resets it after logout', async () => {
    getOrganization.mockResolvedValue({ public_id: 'org_one', name: 'Organization' })
    const access = useDesktopOrganizationAccess()

    await access.refreshDesktopOrganizationAccess()
    expect(access.canManageDesktopOrganization.value).toBe(true)

    authStore.isAuthenticated = false
    authStore.user = null
    await access.refreshDesktopOrganizationAccess()
    expect(access.canManageDesktopOrganization.value).toBe(false)
  })
})
