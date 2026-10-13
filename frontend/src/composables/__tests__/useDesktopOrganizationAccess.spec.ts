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

describe('Desktop accounting access cache isolation', () => {
  it('does not restore an old account’s organization after a delayed response', async () => {
    useDesktopOrganizationAccess().resetDesktopOrganizationAccess()
    authStore.isAuthenticated = true
    authStore.user = { id: 9 }
    let resolveOld!: (value: unknown) => void
    getOrganization.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const access = useDesktopOrganizationAccess()
    const first = access.refreshDesktopOrganizationAccess(true)
    authStore.user = { id: 10 }
    getOrganization.mockResolvedValueOnce({ public_id: 'org_new' })
    await access.refreshDesktopOrganizationAccess(true)
    resolveOld({ public_id: 'org_old' })
    expect(await first).toBeNull()
    expect(access.managedDesktopOrganization.value?.public_id).toBe('org_new')
  })
  it('coalesces forced refreshes so menu and page access do not overwrite each other', async () => {
    useDesktopOrganizationAccess().resetDesktopOrganizationAccess()
    authStore.isAuthenticated = true
    authStore.user = { id: 9 }
    getOrganization.mockClear()
    getOrganization.mockResolvedValue({ public_id: 'org_one' })
    const access = useDesktopOrganizationAccess()
    await Promise.all([access.refreshDesktopOrganizationAccess(true), access.refreshDesktopOrganizationAccess(true)])
    expect(getOrganization).toHaveBeenCalledTimes(1)
    expect(access.canManageDesktopOrganization.value).toBe(true)
  })
})
