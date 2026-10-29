import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>
) => Promise<void>

const routerHarness = vi.hoisted(() => ({
  guard: null as NavigationGuard | null,
}))

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isFinance: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: false,
  cachedPublicSettings: null as null | {
    payment_enabled?: boolean
    risk_control_enabled?: boolean
    desktop_enabled?: boolean
    subscription_enabled?: boolean
    custom_menu_items?: []
  },
  fetchPublicSettings: vi.fn(),
}))

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn(() => ({
    beforeEach: vi.fn((guard: NavigationGuard) => {
      routerHarness.guard = guard
    }),
    afterEach: vi.fn(),
    onError: vi.fn(),
  })),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => ({
    initialized: true,
    fetchStatus: vi.fn(),
    requireAcknowledgement: vi.fn(),
  }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function runGuard(meta: Record<string, unknown>, path: string) {
  if (!routerHarness.guard) {
    throw new Error('router guard was not registered')
  }

  const next = vi.fn()
  const navigation = routerHarness.guard(
    {
      path,
      fullPath: path,
      name: 'FeatureRoute',
      params: {},
      meta: { requiresAuth: true, ...meta },
    },
    {},
    next
  )
  return { navigation, next }
}

describe('feature route guard', () => {
  beforeAll(async () => {
    await import('@/router')
  })

  beforeEach(() => {
    authStore.isAuthenticated = true
    authStore.isAdmin = false
    authStore.isFinance = false
    authStore.isSimpleMode = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockReset()
  })

  it.each(['/admin/dashboard', '/admin/users', '/admin/settings', '/dashboard', '/keys', '/profile', '/login', '/model-plaza', '/admin/accounting-other', '/admin/accounting/organizations/org_one/settings', '/admin/accounting/other', '/admin/accounting/organizations/org_one/transfers/record-one/unknown', '/admin/accounting/vcc-cards/new'])('redirects finance users from %s to accounting', async (path) => {
    authStore.isFinance = true
    const { navigation, next } = runGuard({ requiresAdmin: path.startsWith('/admin'), requiresAuth: path !== '/login' }, path)
    await navigation
    expect(next).toHaveBeenCalledWith('/admin/accounting')
  })

  it.each([false, true])('allows finance accounting access in backend mode %s', async (backendMode) => {
    authStore.isFinance = true
    appStore.backendModeEnabled = backendMode
    const { navigation, next } = runGuard({ requiresAdmin: true }, '/admin/accounting')
    await navigation
    expect(next).toHaveBeenCalledWith()
    appStore.backendModeEnabled = false
  })

  it.each(['/admin/accounting/organizations/org_one', '/admin/accounting/organizations/org_two'])('allows finance users into scoped accounting %s', async path => {
    authStore.isFinance = true
    const { navigation, next } = runGuard({ requiresAdmin: true }, path)
    await navigation
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    '/admin/accounting/orders/new',
    '/admin/accounting/orders/order-one',
    '/admin/accounting/cards',
    '/admin/accounting/channels',
    '/admin/accounting/channels/import',
    '/admin/accounting/channels/account-one',
    '/admin/accounting/organizations/org_one/contacts/new',
    '/admin/accounting/organizations/org_one/contacts/contact-one/edit',
    '/admin/accounting/organizations/org_one/orders/order-one',
    '/admin/accounting/receiving-accounts',
    '/admin/accounting/receiving-accounts/new',
  ])('allows finance users to open accounting page %s', async (path) => {
    authStore.isFinance = true
    const { navigation, next } = runGuard({ requiresAdmin: true }, path)
    await navigation
    expect(next).toHaveBeenCalledWith()
  })

  it('blocks an ordinary user from the enterprise accounting workspace', async () => {
    const { navigation, next } = runGuard({ requiresAdmin: true }, '/admin/accounting/organizations/org_one')
    await navigation
    expect(next).toHaveBeenCalledWith('/dashboard')
  })

  it('requires login before opening finance enterprise accounting', async () => {
    authStore.isFinance = true
    authStore.isAuthenticated = false
    const path = '/admin/accounting/organizations/org_one'
    const { navigation, next } = runGuard({ requiresAdmin: true }, path)
    await navigation
    expect(next).toHaveBeenCalledWith({ path: '/login', query: { redirect: path } })
  })
  it('keeps unauthenticated finance state behind login', async () => {
    authStore.isFinance = true
    authStore.isAuthenticated = false
    const { navigation, next } = runGuard({ requiresAdmin: true }, '/admin/accounting')
    await navigation
    expect(next).toHaveBeenCalledWith({ path: '/login', query: { redirect: '/admin/accounting' } })
  })

  it('waits for the first public-settings request before deciding payment access', async () => {
    const deferred = createDeferred<{ payment_enabled: boolean }>()
    appStore.fetchPublicSettings.mockImplementation(async () => {
      const settings = await deferred.promise
      appStore.cachedPublicSettings = settings
      appStore.publicSettingsLoaded = true
      return settings
    })

    const { navigation, next } = runGuard({ requiresPayment: true }, '/purchase')

    await vi.waitFor(() => expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1))
    expect(next).not.toHaveBeenCalled()

    deferred.resolve({ payment_enabled: true })
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, '/purchase'],
    ['risk control', { requiresRiskControl: true }, '/admin/risk-control'],
    ['subscription', { requiresSubscription: true }, '/subscriptions'],
  ])('does not treat a failed %s settings load as explicitly disabled', async (_name, meta, path) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.fetchPublicSettings.mockResolvedValue(null)

    const { navigation, next } = runGuard(meta, path)
    await navigation

    expect(appStore.publicSettingsLoaded).toBe(false)
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, { payment_enabled: false }, '/dashboard'],
    [
      'risk control',
      { requiresRiskControl: true },
      { risk_control_enabled: false },
      '/admin/settings',
    ],
    ['subscription', { requiresSubscription: true }, { subscription_enabled: false }, '/dashboard'],
  ])('redirects when loaded settings explicitly disable %s', async (_name, meta, settings, target) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.cachedPublicSettings = settings
    appStore.publicSettingsLoaded = true

    const { navigation, next } = runGuard(meta, '/feature')
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith(target)
  })

  it('fails closed when Desktop settings cannot be loaded', async () => {
    authStore.isAdmin = true
    appStore.fetchPublicSettings.mockResolvedValue(null)

    const { navigation, next } = runGuard(
      { requiresAdmin: true, requiresDesktop: true },
      '/admin/desktop/organizations'
    )
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith('/admin/dashboard')
  })

  it('allows Desktop routes only when the opt-in flag is true', async () => {
    authStore.isAdmin = true
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = { desktop_enabled: true }

    const { navigation, next } = runGuard(
      { requiresAdmin: true, requiresDesktop: true },
      '/admin/desktop/organizations'
    )
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it('redirects a user Desktop route to the user dashboard when Desktop is disabled', async () => {
    authStore.isAdmin = false
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = { desktop_enabled: false }

    const { navigation, next } = runGuard(
      { requiresDesktop: true },
      '/desktop/organization'
    )
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith('/dashboard')
  })

  it('allows Desktop software updates when the organization feature is disabled', async () => {
    authStore.isAdmin = true
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = { desktop_enabled: false }

    const { navigation, next } = runGuard(
      { requiresAdmin: true },
      '/admin/desktop/updates'
    )
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it('allows Desktop software updates in simple mode', async () => {
    authStore.isAdmin = true
    authStore.isSimpleMode = true

    const { navigation, next } = runGuard(
      { requiresAdmin: true },
      '/admin/desktop/updates'
    )
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })
})

describe('subscription route guard (opt-out flag)', () => {
  beforeEach(() => {
    authStore.isAdmin = false
    authStore.isSimpleMode = false
    appStore.publicSettingsLoaded = true
    appStore.fetchPublicSettings.mockReset()
  })

  it.each([
    ['missing key', {}],
    ['explicit true', { subscription_enabled: true }],
  ])('lets /subscriptions through when the flag is %s', async (_name, settings) => {
    appStore.cachedPublicSettings = settings

    const { navigation, next } = runGuard({ requiresSubscription: true }, '/subscriptions')
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it('sends admins to the admin dashboard when subscriptions are disabled', async () => {
    authStore.isAdmin = true
    appStore.cachedPublicSettings = { subscription_enabled: false }

    const { navigation, next } = runGuard({ requiresSubscription: true }, '/subscriptions')
    await navigation

    expect(next).toHaveBeenCalledWith('/admin/dashboard')
  })
})
