import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({ listResources: vi.fn(), getResource: vi.fn(), listResourceVersions: vi.fn(), validateResource: vi.fn(), publishResource: vi.fn(), findPublishedResource: vi.fn(), setResourceStatus: vi.fn(), getResourceDownloadURL: vi.fn() }))
const app = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin/desktopResources', async importOriginal => ({ ...await importOriginal<object>(), ...api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => app }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<object>(), useI18n: () => ({ t: (key: string) => key }) }))
import DesktopResourcesView from './DesktopResourcesView.vue'

const manifest = { key: 'test', name: 'Resource', description: '', version: '1.0.0', platform: 'any', kind: 'skill', targets: ['chatgpt_codex'] }
const preview = { manifest, sha256: 'hash', sizeBytes: 3, current: null }
const resource = { id: 'res_one', name: 'Resource', key: 'test', status: 'active', artifacts: [] }
function mountView() {
  return mount(DesktopResourcesView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' },
    DataTable: true, Pagination: true, Select: true, EmptyState: true, StatusBadge: true,
  } } })
}

describe('DesktopResourcesView', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.listResources.mockResolvedValue({ items: [], total: 0 })
    api.validateResource.mockResolvedValue(preview)
    api.publishResource.mockResolvedValue(resource)
    api.findPublishedResource.mockResolvedValue(null)
    api.setResourceStatus.mockResolvedValue({ ...resource, status: 'disabled' })
    api.getResource.mockResolvedValue(resource)
    api.listResourceVersions.mockResolvedValue({ items: [], total: 0 })
  })
  it('validates without publishing and requires a separate confirmation', async () => {
    const wrapper = mountView(); await flushPromises()
    const vm = wrapper.vm as any
    vm.showUpload = true
    const file = new File(['ZIP'], 'resource.zip')
    await vm.selectFile({ target: { files: [file] } })
    expect(api.validateResource).toHaveBeenCalledWith(file, expect.any(Function))
    expect(api.publishResource).not.toHaveBeenCalled()
    await vm.publish()
    expect(api.publishResource).toHaveBeenCalledTimes(1)
    expect(vm.published).toBe(true)
    await vm.publish()
    expect(api.publishResource).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it.each([0, 409, 502])('keeps publication pending after ambiguous status %s and verifies before success', async status => {
    api.publishResource.mockRejectedValue({ status })
    const wrapper = mountView(); await flushPromises(); const vm = wrapper.vm as any
    await vm.selectFile({ target: { files: [new File(['ZIP'], 'resource.zip')] } })
    await vm.publish()
    expect(vm.pending).toBe(true); expect(vm.published).toBe(false); expect(app.showSuccess).not.toHaveBeenCalled()
    expect(api.publishResource).toHaveBeenCalledTimes(1)
    api.findPublishedResource.mockResolvedValue(resource)
    await vm.verifyPublication()
    expect(vm.pending).toBe(false); expect(vm.published).toBe(true)
    wrapper.unmount()
  })
  it('invalidates the previous preview when a replacement fails validation', async () => {
    const wrapper = mountView(); await flushPromises(); const vm = wrapper.vm as any
    await vm.selectFile({ target: { files: [new File(['ZIP'], 'first.zip')] } })
    api.validateResource.mockRejectedValue({ error: { message: 'bad hash' } })
    await vm.selectFile({ target: { files: [new File(['bad'], 'second.zip')] } })
    expect(vm.preview).toBe(null); expect(vm.uploadError).toBe('bad hash')
    await vm.publish(); expect(api.publishResource).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('preserves a validated selection on cancel and prevents publishing after clearing it', async () => {
    const wrapper = mountView(); await flushPromises(); const vm = wrapper.vm as any
    const file = new File(['ZIP'], 'resource.zip')
    await vm.selectFile({ target: { files: [file] } })
    await vm.selectFile({ target: { files: [] } })
    expect(vm.selectedFile).toBe(file)
    expect(vm.preview).toEqual(preview)
    vm.clearSelectedFile()
    await vm.publish()
    expect(vm.preview).toBe(null)
    expect(vm.selectedFile).toBe(null)
    expect(api.publishResource).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('disables a resource with a reason and presents the public URL limitation', async () => {
    const wrapper = mountView(); await flushPromises(); const vm = wrapper.vm as any
    vm.statusTarget = resource; vm.statusReason = 'investigate'
    await flushPromises()
    expect(wrapper.text()).toContain('admin.desktop.resources.statusHint')
    await vm.saveStatus()
    expect(api.setResourceStatus).toHaveBeenCalledWith('res_one', 'disabled', 'investigate')
    expect(vm.statusTarget).toBe(null)
    wrapper.unmount()
  })
  it('loads a fresh detail and paginated version history', async () => {
    const wrapper = mountView(); await flushPromises(); const vm = wrapper.vm as any
    await vm.openDetail('res_one'); await vm.loadVersions(2)
    expect(api.getResource).toHaveBeenCalledWith('res_one')
    expect(api.listResourceVersions).toHaveBeenLastCalledWith('res_one', 2)
    expect(vm.versionPage).toBe(2)
    wrapper.unmount()
  })
  it('obtains a fresh private URL per download and blocks disabled resources', async () => {
    const clicks: HTMLAnchorElement[] = []
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) { clicks.push(this) })
    api.getResourceDownloadURL.mockResolvedValue({ url: 'https://private.example.com/file?signature=one' })
    const wrapper = mountView(); await flushPromises(); const vm = wrapper.vm as any
    vm.detail = resource
    await vm.downloadArtifact('res_one', '1.0.0', 'any')
    await vm.downloadArtifact('res_one', '1.0.0', 'any')
    expect(api.getResourceDownloadURL).toHaveBeenCalledTimes(2)
    expect(clicks[0].href).toBe('https://private.example.com/file?signature=one')
    expect(clicks[0].referrerPolicy).toBe('no-referrer')
    vm.detail = { ...resource, status: 'disabled' }
    await vm.downloadArtifact('res_one', '1.0.0', 'any')
    expect(api.getResourceDownloadURL).toHaveBeenCalledTimes(2)
    click.mockRestore(); wrapper.unmount()
  })
})
