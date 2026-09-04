import { beforeEach, describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  isAuthenticated: true,
  user: { id: 9 } as { id: number } | null,
}))
const listSubscriptionAccounts = vi.hoisted(() => vi.fn())

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/api/subscriptionAccounts', () => ({
  default: { list: listSubscriptionAccounts },
}))

import { useSubscriptionAccountAccess } from '@/composables/useSubscriptionAccountAccess'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('useSubscriptionAccountAccess', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authStore.isAuthenticated = true
    authStore.user = { id: 9 }
    useSubscriptionAccountAccess().resetSubscriptionAccountAccess()
  })

  it('ignores a stale response after the authenticated user changes', async () => {
    const first = deferred<Array<{ id: number }>>()
    listSubscriptionAccounts.mockReturnValueOnce(first.promise).mockResolvedValueOnce([])
    const access = useSubscriptionAccountAccess()

    const firstLoad = access.refreshSubscriptionAccountAccess(true)
    authStore.user = { id: 10 }
    await access.refreshSubscriptionAccountAccess(true)
    first.resolve([{ id: 7 }])
    await firstLoad

    expect(access.hasSubscriptionAccounts.value).toBe(false)
  })

  it('retains the last known state when a forced refresh fails', async () => {
    const access = useSubscriptionAccountAccess()
    access.setSubscriptionAccountAccess(true)
    listSubscriptionAccounts.mockRejectedValueOnce(new Error('network unavailable'))

    const result = await access.refreshSubscriptionAccountAccess(true)

    expect(result).toBe(true)
    expect(access.hasSubscriptionAccounts.value).toBe(true)
  })
})
