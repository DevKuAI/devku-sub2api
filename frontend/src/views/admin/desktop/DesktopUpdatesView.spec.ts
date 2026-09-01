import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const desktopAPI = vi.hoisted(() => ({
  listUpdateReleases: vi.fn(),
  createUpdateRelease: vi.fn(),
  updateUpdateRelease: vi.fn(),
  uploadUpdateArtifact: vi.fn(),
  publishUpdateRelease: vi.fn(),
  withdrawUpdateRelease: vi.fn(),
}))

const appStore = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { desktop: desktopAPI } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/composables/usePersistedPageSize', () => ({ getPersistedPageSize: () => 20 }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

import DesktopUpdatesView from './DesktopUpdatesView.vue'

function artifact(fileName: string, signature: string) {
  return {
    url: `https://downloads.example.com/desktop-updates/1.2.3/${fileName}`,
    signature,
    object_key: `desktop-updates/1.2.3/${fileName}`,
    file_name: fileName,
    size: 1234,
    sha256: 'a'.repeat(64),
  }
}

const release = {
  public_id: 'upd_one',
  version: '1.2.3',
  notes: 'Release notes',
  artifacts: {
    'darwin-aarch64': artifact('arm.app.tar.gz', 'arm-signature'),
    'darwin-x86_64': artifact('x64.app.tar.gz', 'x64-signature'),
    'windows-x86_64': artifact('app.nsis.zip', 'windows-signature'),
  },
  status: 'draft' as const,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
}

function mountView() {
  return mount(DesktopUpdatesView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: true,
        Pagination: true,
        BaseDialog: { template: '<section><slot /><slot name="footer" /></section>' },
        EmptyState: true,
        StatusBadge: true,
        Select: true,
        Input: true,
        Icon: true,
      },
    },
  })
}

describe('DesktopUpdatesView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    desktopAPI.listUpdateReleases.mockResolvedValue({ items: [{ ...release }], total: 1, page: 1, page_size: 20, pages: 1 })
    desktopAPI.createUpdateRelease.mockResolvedValue({ ...release })
    desktopAPI.updateUpdateRelease.mockResolvedValue({ ...release })
    desktopAPI.uploadUpdateArtifact.mockResolvedValue(artifact('arm.app.tar.gz', ''))
    desktopAPI.publishUpdateRelease.mockResolvedValue({ ...release, status: 'published' })
    desktopAPI.withdrawUpdateRelease.mockResolvedValue({ ...release, status: 'withdrawn' })
  })

  it('loads releases and allows incomplete artifacts while the release is a draft', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    expect(desktopAPI.listUpdateReleases).toHaveBeenCalledWith(1, 20, '', expect.any(AbortSignal))
    vm.openCreate()
    await vm.saveDraft()
    expect(desktopAPI.createUpdateRelease).not.toHaveBeenCalled()

    vm.form.version = '1.2.3'
    vm.form.notes = 'Release notes'
    await vm.saveDraft()

    expect(desktopAPI.createUpdateRelease).toHaveBeenCalledWith({
      version: '1.2.3',
      notes: 'Release notes',
      artifacts: {
        'darwin-aarch64': { url: '', signature: '', object_key: '', file_name: '', size: 0, sha256: '' },
        'darwin-x86_64': { url: '', signature: '', object_key: '', file_name: '', size: 0, sha256: '' },
        'windows-x86_64': { url: '', signature: '', object_key: '', file_name: '', size: 0, sha256: '' },
      },
    })
    expect(desktopAPI.updateUpdateRelease).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('uploads a selected updater bundle before saving its metadata', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.openEdit(release)
    const file = new File(['artifact'], 'DevKu.app.tar.gz', { type: 'application/gzip' })
    vm.pendingFiles['darwin-aarch64'] = file
    await vm.saveDraft()

    expect(desktopAPI.uploadUpdateArtifact).toHaveBeenCalledWith('upd_one', 'darwin-aarch64', file, expect.any(Function))
    expect(desktopAPI.updateUpdateRelease).toHaveBeenCalledWith('upd_one', expect.objectContaining({
      artifacts: expect.objectContaining({
        'darwin-aarch64': expect.objectContaining({ file_name: 'arm.app.tar.gz', signature: 'arm-signature' }),
      }),
    }))
    wrapper.unmount()
  })

  it('requires existing artifacts to be selected again after the version changes', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.openEdit(release)
    vm.form.version = '1.2.4'
    await vm.saveDraft()

    expect(desktopAPI.updateUpdateRelease).not.toHaveBeenCalled()
    expect(vm.errors['darwin-aarch64.file']).toBe('admin.desktop.updates.validation.versionChangedReupload')
    expect(vm.errors['darwin-x86_64.file']).toBe('admin.desktop.updates.validation.versionChangedReupload')
    expect(vm.errors['windows-x86_64.file']).toBe('admin.desktop.updates.validation.versionChangedReupload')
    wrapper.unmount()
  })

  it('updates the draft version before uploading replacement artifacts', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.openEdit(release)
    vm.form.version = '1.2.4'
    for (const platform of ['darwin-aarch64', 'darwin-x86_64', 'windows-x86_64']) {
      const suffix = platform === 'windows-x86_64' ? '.nsis.zip' : '.app.tar.gz'
      vm.pendingFiles[platform] = new File([platform], `${platform}${suffix}`)
    }
    await vm.saveDraft()

    expect(desktopAPI.updateUpdateRelease).toHaveBeenNthCalledWith(1, 'upd_one', {
      version: '1.2.4',
      notes: 'Release notes',
      artifacts: {
        'darwin-aarch64': { url: '', signature: 'arm-signature', object_key: '', file_name: '', size: 0, sha256: '' },
        'darwin-x86_64': { url: '', signature: 'x64-signature', object_key: '', file_name: '', size: 0, sha256: '' },
        'windows-x86_64': { url: '', signature: 'windows-signature', object_key: '', file_name: '', size: 0, sha256: '' },
      },
    })
    expect(desktopAPI.uploadUpdateArtifact).toHaveBeenCalledTimes(3)
    expect(desktopAPI.updateUpdateRelease).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('uses ordinary confirmations for publish and reason-required withdraw actions', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.openPublish(release)
    await vm.publishRelease()
    expect(desktopAPI.publishUpdateRelease).toHaveBeenCalledWith('upd_one')

    vm.openWithdraw({ ...release, status: 'published' })
    await vm.withdrawRelease()
    expect(desktopAPI.withdrawUpdateRelease).not.toHaveBeenCalled()
    vm.withdrawalReason = 'Rollback due to startup regression'
    await vm.withdrawRelease()
    expect(desktopAPI.withdrawUpdateRelease).toHaveBeenCalledWith('upd_one', 'Rollback due to startup regression')
    wrapper.unmount()
  })

  it('blocks publish confirmation until all artifacts are complete', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.openPublish({
      ...release,
      artifacts: {
        ...release.artifacts,
        'windows-x86_64': { url: '', signature: '', object_key: '', file_name: '', size: 0, sha256: '' },
      },
    })

    expect(appStore.showError).toHaveBeenCalledWith('admin.desktop.updates.validation.publishArtifacts')
    expect(vm.showPublish).toBe(false)
    wrapper.unmount()
  })
})
