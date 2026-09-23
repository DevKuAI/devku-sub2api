import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BIPrivacySettings from './BIPrivacySettings.vue'

const api = vi.hoisted(() => ({ getBIPrivacyNotice: vi.fn(), saveBIPrivacyNotice: vi.fn() }))
vi.mock('@/api/biPrivacy', () => api)
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const notice = { site_url: 'https://example.com', title: 'Privacy', version: 'v1', content_md: '# Scope', updated_at: '2026-09-23T00:00:00Z', url: 'https://example.com/legal/bi-privacy' }
const options = { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } }

describe('BI privacy maintenance', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.getBIPrivacyNotice.mockResolvedValue({ ...notice })
    api.saveBIPrivacyNotice.mockImplementation(async input => ({ ...notice, ...input, url: `${input.site_url}/legal/bi-privacy` }))
  })

  it('loads the admin record and publishes only editable fields with a separate action', async () => {
    const wrapper = mount(BIPrivacySettings, options)
    await flushPromises()
    expect(api.getBIPrivacyNotice).toHaveBeenCalledWith(true)
    await wrapper.get('#bi-privacy-site-url').setValue('https://bi.example.com')
    await wrapper.get('#bi-privacy-content').setValue('# Updated')
    await wrapper.get('#bi-privacy-version').setValue('v2')
    expect(wrapper.get('button').attributes('type')).toBe('button')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(api.saveBIPrivacyNotice).toHaveBeenCalledWith({ site_url: 'https://bi.example.com', title: 'Privacy', version: 'v2', content_md: '# Updated' })
    expect(wrapper.get('[role="status"]').text()).toBe('bi.privacy.saved')
    expect(wrapper.get('a').text()).toBe('bi.privacy.view')
    expect(wrapper.get('a').attributes('href')).toBe('https://bi.example.com/legal/bi-privacy')
  })

  it('does not allow a failed load to overwrite the saved notice and supports retry', async () => {
    api.getBIPrivacyNotice.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount(BIPrivacySettings, options)
    await flushPromises()
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toBe('bi.privacy.loadFailed')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.find('textarea').exists()).toBe(true)
    expect(api.saveBIPrivacyNotice).not.toHaveBeenCalled()
  })

  it('requires content and reports version errors without discarding edits', async () => {
    const wrapper = mount(BIPrivacySettings, options)
    await flushPromises()
    await wrapper.get('#bi-privacy-content').setValue(' ')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    await wrapper.get('#bi-privacy-content').setValue('# Changed')
    api.saveBIPrivacyNotice.mockRejectedValue({ reason: 'BI_PRIVACY_VERSION_REQUIRED' })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('bi.privacy.versionRequired')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('# Changed')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })

  it('requires the independent site address before publication', async () => {
    api.getBIPrivacyNotice.mockResolvedValue({ ...notice, site_url: '', url: '' })
    const wrapper = mount(BIPrivacySettings, options)
    await flushPromises()
    expect(wrapper.text()).toContain('bi.privacy.urlHint')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(wrapper.find('a').exists()).toBe(false)
    await wrapper.get('#bi-privacy-site-url').setValue('https://bi.example.com')
    expect(wrapper.get('button').attributes('disabled')).toBeUndefined()
  })
  it('reports an invalid site address without losing the edit', async () => {
    api.saveBIPrivacyNotice.mockRejectedValue({ reason: 'INVALID_BI_PRIVACY_SITE_URL' })
    const wrapper = mount(BIPrivacySettings, options)
    await flushPromises()
    await wrapper.get('#bi-privacy-site-url').setValue('http://bi.example.com')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('bi.privacy.siteURLInvalid')
    expect((wrapper.get('#bi-privacy-site-url').element as HTMLInputElement).value).toBe('http://bi.example.com')
  })

})
