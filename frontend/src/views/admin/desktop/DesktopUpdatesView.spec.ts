import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const desktopAPI = vi.hoisted(() => ({
  listUpdateReleases: vi.fn(),
  createUpdateRelease: vi.fn(),
  updateUpdateRelease: vi.fn(),
  uploadUpdateArtifact: vi.fn(),
  publishUpdateRelease: vi.fn(),
  withdrawUpdateRelease: vi.fn(),
  promoteUpdateRelease: vi.fn(),
  listOrganizations: vi.fn(),
  republishUpdateRelease: vi.fn(),
  deleteUpdateRelease: vi.fn(),
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
    'darwin-aarch64': artifact('arm.dmg', 'arm-signature'),
    'darwin-x86_64': artifact('x64.app', 'x64-signature'),
    'windows-x86_64': artifact('app.exe', 'windows-signature'),
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
        DataTable: { props: ['data'], template: '<div><div v-for="row in data" :key="row.public_id"><slot name="cell-actions" :row="row" /></div></div>' },
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
    desktopAPI.uploadUpdateArtifact.mockResolvedValue(artifact('arm.dmg', ''))
    desktopAPI.publishUpdateRelease.mockResolvedValue({ ...release, status: 'published' })
    desktopAPI.listOrganizations.mockResolvedValue({ items: [], total: 0 })
    desktopAPI.promoteUpdateRelease.mockResolvedValue({ ...release, status: 'published', rollout_mode: 'full' })
    desktopAPI.republishUpdateRelease.mockResolvedValue({ ...release, status: 'published', rollout_mode: 'full' })
    desktopAPI.deleteUpdateRelease.mockResolvedValue(undefined)
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
    const file = new File(['artifact'], 'DevKu.dmg', { type: 'application/x-apple-diskimage' })
    vm.pendingFiles['darwin-aarch64'] = file
    await vm.saveDraft()

    expect(desktopAPI.uploadUpdateArtifact).toHaveBeenCalledWith('upd_one', 'darwin-aarch64', file, expect.any(Function))
    expect(desktopAPI.updateUpdateRelease).toHaveBeenCalledWith('upd_one', expect.objectContaining({
      artifacts: expect.objectContaining({
        'darwin-aarch64': expect.objectContaining({ file_name: 'arm.dmg', signature: 'arm-signature' }),
      }),
    }))
    wrapper.unmount()
  })

  it('blocks an older updater artifact before creating or uploading a release', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.version = '0.3.4'
    const file = new File(['old updater'], 'Devku.Desktop_0.3.3_x64-setup.nsis.zip')
    vm.selectArtifactFile('windows-x86_64', { target: { files: [file], value: file.name } })
    expect(vm.errors['windows-x86_64.file']).toBe('admin.desktop.updates.validation.artifactVersionMismatch')
    await vm.saveDraft()
    expect(desktopAPI.createUpdateRelease).not.toHaveBeenCalled()
    expect(desktopAPI.updateUpdateRelease).not.toHaveBeenCalled()
    expect(desktopAPI.uploadUpdateArtifact).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('rechecks pending artifact versions after editing the release version', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openCreate()
    vm.form.version = '0.3.3'
    const file = new File(['updater'], 'Devku.Desktop_0.3.3_x64-setup.nsis.zip')
    vm.selectArtifactFile('windows-x86_64', { target: { files: [file], value: file.name } })
    expect(vm.errors['windows-x86_64.file']).toBeUndefined()
    vm.form.version = '0.3.4'
    await vm.saveDraft()
    expect(vm.errors['windows-x86_64.file']).toBe('admin.desktop.updates.validation.artifactVersionMismatch')
    expect(desktopAPI.createUpdateRelease).not.toHaveBeenCalled()
    expect(desktopAPI.uploadUpdateArtifact).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('blocks publishing an existing draft with an older updater artifact', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openPublish({ ...release, version: '0.3.4', artifacts: {
      ...release.artifacts,
      'windows-x86_64': artifact('Devku.Desktop_0.3.3_x64-setup.nsis.zip', 'signature'),
    } })
    expect(appStore.showError).toHaveBeenCalledWith('admin.desktop.updates.validation.artifactVersionMismatch')
    expect(vm.showPublish).toBe(false)
    expect(desktopAPI.publishUpdateRelease).not.toHaveBeenCalled()
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
      const suffix = platform === 'windows-x86_64' ? '.msi' : '.app'
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

  it('does not restrict selected files by suffix', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    const file = new File(['artifact'], 'DevKu.custom')

    vm.selectArtifactFile('darwin-aarch64', { target: { files: [file], value: 'DevKu.custom' } })

    expect(vm.pendingFiles['darwin-aarch64']).toBe(file)
    expect(vm.errors['darwin-aarch64.file']).toBeUndefined()
    expect(wrapper.findAll('input[type="file"]')).toHaveLength(3)
    expect(wrapper.findAll('input[type="file"]').every((input) => input.attributes('accept') === undefined)).toBe(true)
    expect(wrapper.text()).toContain('.app.tar.gz')
    expect(wrapper.text()).toContain('.nsis.zip')
    wrapper.unmount()
  })

  it('uses ordinary confirmations for publish and reason-required withdraw actions', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.openPublish(release)
    await vm.publishRelease()
    expect(desktopAPI.publishUpdateRelease).toHaveBeenCalledWith('upd_one', { rollout_mode: 'full', gateway_user_ids: [] })

    vm.openWithdraw({ ...release, status: 'published' })
    await vm.withdrawRelease()
    expect(desktopAPI.withdrawUpdateRelease).not.toHaveBeenCalled()
    vm.withdrawalReason = 'Rollback due to startup regression'
    await vm.withdrawRelease()
    expect(desktopAPI.withdrawUpdateRelease).toHaveBeenCalledWith('upd_one', 'Rollback due to startup regression')
    wrapper.unmount()
  })

  it('blocks publication when an uploaded artifact has no signature', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any

    vm.openPublish({
      ...release,
      artifacts: {
        ...release.artifacts,
        'windows-x86_64': { ...release.artifacts['windows-x86_64'], signature: '' },
      },
    })

    expect(appStore.showError).toHaveBeenCalledWith('admin.desktop.updates.validation.publishArtifacts')
    expect(vm.showPublish).toBe(false)
    wrapper.unmount()
  })
  it('shows the enterprise audience only for gray release and submits selected users', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openPublish(release)
    await flushPromises()
    expect(wrapper.find('input[aria-label="admin.desktop.updates.searchAudience"]').exists()).toBe(false)
    vm.grayRelease = true
    await flushPromises()
    expect(wrapper.find('input[aria-label="admin.desktop.updates.searchAudience"]').exists()).toBe(true)
    await vm.publishRelease()
    expect(desktopAPI.publishUpdateRelease).not.toHaveBeenCalled()
    vm.audienceIDs = [7, 9]
    await vm.publishRelease()
    expect(desktopAPI.publishUpdateRelease).toHaveBeenCalledWith('upd_one', { rollout_mode: 'gray', gateway_user_ids: [7, 9] })
    wrapper.unmount()
  })

  it('publishes uploaded platforms and promotes gray release without uploading again', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    const partial = { ...release, artifacts: { 'windows-x86_64': release.artifacts['windows-x86_64'] } }
    vm.openPublish(partial)
    expect(vm.showPublish).toBe(true)
    await vm.publishRelease()
    vm.openPromote({ ...partial, status: 'published', rollout_mode: 'gray' })
    await vm.publishRelease()
    expect(desktopAPI.promoteUpdateRelease).toHaveBeenCalledWith('upd_one')
    expect(desktopAPI.uploadUpdateArtifact).not.toHaveBeenCalled()
    expect(desktopAPI.updateUpdateRelease).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('renders edit, delete and republish actions for published and withdrawn versions', async () => {
    desktopAPI.listUpdateReleases.mockResolvedValue({ items: [
      { ...release, status: 'published' }, { ...release, public_id: 'upd_other', status: 'withdrawn' },
    ], total: 2 })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('button[aria-label="common.edit"]')).toHaveLength(2)
    expect(wrapper.findAll('button[aria-label="common.delete"]')).toHaveLength(2)
    expect(wrapper.findAll('button[aria-label="admin.desktop.updates.republish"]')).toHaveLength(2)
    await wrapper.find('button[aria-label="common.edit"]').trigger('click')
    expect(wrapper.text()).toContain('admin.desktop.updates.editHint')
    wrapper.unmount()
  })

  it('saves a published release as a draft before replacing its artifacts with the same version', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openEdit({ ...release, status: 'published' })
    vm.form.notes = 'replacement'
    const file = new File(['new updater'], 'Devku.Desktop_1.2.3_x64-setup.nsis.zip')
    vm.selectArtifactFile('windows-x86_64', { target: { files: [file], value: file.name } })
    await vm.saveDraft()
    expect(desktopAPI.createUpdateRelease).not.toHaveBeenCalled()
    expect(desktopAPI.updateUpdateRelease).toHaveBeenCalledTimes(2)
    expect(desktopAPI.updateUpdateRelease.mock.invocationCallOrder[0]).toBeLessThan(desktopAPI.uploadUpdateArtifact.mock.invocationCallOrder[0])
    expect(desktopAPI.updateUpdateRelease.mock.calls[1][1]).toMatchObject({ version: '1.2.3', notes: 'replacement' })
    expect(desktopAPI.publishUpdateRelease).not.toHaveBeenCalled()
    expect(desktopAPI.republishUpdateRelease).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('republishes a withdrawn version with its previous gray audience and no reupload', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    vm.openPublish({ ...release, status: 'withdrawn', rollout_mode: 'gray', gateway_user_ids: [7, 9] }, true)
    await vm.publishRelease()
    expect(desktopAPI.republishUpdateRelease).toHaveBeenCalledWith('upd_one', { rollout_mode: 'gray', gateway_user_ids: [7, 9] })
    expect(desktopAPI.uploadUpdateArtifact).not.toHaveBeenCalled()
    expect(desktopAPI.publishUpdateRelease).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('requires confirmation to delete and keeps the dialog open on failure', async () => {
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as any
    await wrapper.find('button[aria-label="common.delete"]').trigger('click')
    expect(vm.showDelete).toBe(true)
    expect(desktopAPI.deleteUpdateRelease).not.toHaveBeenCalled()
    vm.closeDelete()
    expect(desktopAPI.deleteUpdateRelease).not.toHaveBeenCalled()
    vm.openDelete(release)
    desktopAPI.deleteUpdateRelease.mockRejectedValueOnce(new Error('failed'))
    await vm.deleteRelease()
    expect(vm.showDelete).toBe(true)
    expect(appStore.showError).toHaveBeenCalledWith('failed')
    await vm.deleteRelease()
    expect(desktopAPI.deleteUpdateRelease).toHaveBeenCalledWith('upd_one')
    expect(vm.showDelete).toBe(false)
    expect(desktopAPI.listUpdateReleases).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

})
