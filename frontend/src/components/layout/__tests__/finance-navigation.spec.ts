import { mount, flushPromises, RouterLinkStub } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import AppSidebar from '../AppSidebar.vue'
import AppHeader from '../AppHeader.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/admin/accounting', meta: { titleKey: 'accounting.title' } }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

const apiRequest = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', () => ({
  apiClient: { get: apiRequest, post: apiRequest },
  default: { get: apiRequest, post: apiRequest },
}))

describe('finance navigation', () => {
  it('shows only accounting and avoids loading other business APIs', async () => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.user = { id: 7, role: 'finance', email: 'finance@example.com', username: 'Finance' } as never
    auth.token = 'finance-token'
    const app = useAppStore()
    app.cachedPublicSettings = { desktop_enabled: true, subscription_enabled: true } as never
    apiRequest.mockClear()

    const sidebar = mount(AppSidebar, {
      global: { stubs: { RouterLink: RouterLinkStub, VersionBadge: true, Icon: true } },
    })
    await flushPromises()
    expect(sidebar.get('nav').findAllComponents(RouterLinkStub).map((link) => link.props('to'))).toEqual(['/admin/accounting'])
    expect(apiRequest).not.toHaveBeenCalled()

    const header = mount(AppHeader, {
      global: { stubs: {
        RouterLink: RouterLinkStub, Icon: true, LocaleSwitcher: true,
        AnnouncementBell: true, SubscriptionProgressMini: true,
      } },
    })
    expect(header.findComponent({ name: 'AnnouncementBell' }).exists()).toBe(false)
    expect(header.findComponent({ name: 'SubscriptionProgressMini' }).exists()).toBe(false)
    await header.get('button[aria-label="common.userMenu"]').trigger('click')
    expect(header.findAllComponents(RouterLinkStub)).toHaveLength(0)
    sidebar.unmount()
    header.unmount()
  })
})
