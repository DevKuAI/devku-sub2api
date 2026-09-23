import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import LegalDocumentView from './LegalDocumentView.vue'

const state = vi.hoisted(() => ({ getBIPrivacyNotice: vi.fn(), fetchPublicSettings: vi.fn(), route: { params: { documentId: 'bi-privacy' } } }))
vi.mock('@/api/biPrivacy', () => ({ getBIPrivacyNotice: state.getBIPrivacyNotice }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({
  cachedPublicSettings: { site_name: 'Example', login_agreement_updated_at: '2026-01-01', login_agreement_documents: [{ id: 'terms', title: 'Terms', content_md: 'Web terms' }] },
  fetchPublicSettings: state.fetchPublicSettings,
}) }))
vi.mock('vue-router', () => ({ useRoute: () => state.route }))
vi.mock('@/i18n', () => ({ getLocale: () => 'en' }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const notice = { site_url: 'https://example.com', title: 'Privacy', version: 'v2', content_md: '# Scope\n\nRead this.\n\n<img src="x" onerror="alert(1)"><script>alert(2)</script>\n\n[unsafe](javascript:alert(3))', updated_at: '2026-09-23T00:00:00Z', url: 'https://example.com/legal/bi-privacy' }
const options = { global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, Icon: true } } }

describe('Public BI privacy page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    state.route = reactive({ params: { documentId: 'bi-privacy' } })
    state.fetchPublicSettings.mockResolvedValue({ site_name: 'Example' })
    state.getBIPrivacyNotice.mockResolvedValue({ ...notice })
  })

  it('loads the public record and sanitizes Markdown without requiring login', async () => {
    const wrapper = mount(LegalDocumentView, options)
    await flushPromises()
    expect(state.getBIPrivacyNotice).toHaveBeenCalledWith()
    expect(wrapper.get('h1').text()).toBe('Privacy')
    expect(wrapper.text()).toContain('v2')
    expect(wrapper.get('.legal-document-content h1').text()).toBe('Scope')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.find('[onerror]').exists()).toBe(false)
    expect(wrapper.find('a[href^="javascript:"]').exists()).toBe(false)
  })

  it.each([[404, 'legal.notFound'], [503, 'legal.loadFailed']])('shows the correct state for HTTP %s', async (status, message) => {
    state.getBIPrivacyNotice.mockRejectedValue({ status })
    const wrapper = mount(LegalDocumentView, options)
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe(message)
    expect(wrapper.find('article').exists()).toBe(false)
  })

  it('keeps a published notice readable when branding settings are unavailable', async () => {
    state.fetchPublicSettings.mockRejectedValue(new Error('branding failed'))
    const wrapper = mount(LegalDocumentView, options)
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Privacy')
  })

  it('does not replace a different legal document with a late privacy response', async () => {
    let resolve!: (value: typeof notice) => void
    state.getBIPrivacyNotice.mockReturnValue(new Promise(r => { resolve = r }))
    const wrapper = mount(LegalDocumentView, options)
    state.route.params.documentId = 'terms'
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Terms')
    resolve(notice)
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Terms')
    expect(wrapper.text()).not.toContain('v2')
  })
})
