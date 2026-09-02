<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <Select v-model="status" class="w-44" :options="statusOptions" @change="resetAndLoad" />
          <div class="ml-auto flex items-center gap-2">
            <button class="btn btn-secondary" type="button" :title="t('common.refresh')" :aria-label="t('common.refresh')" :disabled="loading" @click="loadReleases">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" type="button" @click="openCreate">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.desktop.updates.create') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="releases" :loading="loading" row-key="public_id">
          <template #cell-version="{ row }">
            <div class="min-w-52 max-w-80">
              <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ row.version }}</div>
              <div v-if="row.notes" class="mt-1 line-clamp-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ row.notes }}</div>
            </div>
          </template>
          <template #cell-status="{ row }">
            <StatusBadge :status="statusVariant(row.status)" :label="statusLabel(row.status)" />
          </template>
          <template #cell-artifacts="{ row }">
            <div class="space-y-1.5">
              <div v-for="platform in platforms" :key="platform.key" class="flex min-h-6 items-center gap-1.5 text-xs">
                <span class="w-28 shrink-0 font-medium text-gray-600 dark:text-dark-300">{{ platform.shortLabel }}</span>
                <a
                  v-if="row.artifacts[platform.key].url"
                  :href="row.artifacts[platform.key].url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex min-w-0 items-center gap-1 text-primary-600 hover:text-primary-700 dark:text-primary-400"
                >
                  <span class="max-w-40 truncate">{{ row.artifacts[platform.key].file_name }}</span>
                  <Icon name="externalLink" size="xs" class="shrink-0" aria-hidden="true" />
                </a>
                <span v-else class="text-gray-400 dark:text-dark-500">{{ t('common.notAvailable') }}</span>
              </div>
            </div>
          </template>
          <template #cell-published_at="{ row }">
            <span class="whitespace-nowrap text-sm tabular-nums text-gray-500 dark:text-dark-400">
              {{ row.published_at ? formatDateTime(row.published_at) : t('common.notAvailable') }}
            </span>
          </template>
          <template #cell-updated_at="{ value }">
            <span class="whitespace-nowrap text-sm tabular-nums text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex min-w-28 items-center justify-end gap-1">
              <button v-if="row.status === 'draft'" class="action-button" type="button" :title="t('common.edit')" :aria-label="t('common.edit')" @click="openEdit(row)">
                <Icon name="edit" size="sm" />
              </button>
              <button v-if="row.status === 'draft'" class="action-button text-primary-600 dark:text-primary-400" type="button" :title="t('admin.desktop.updates.publish')" :aria-label="t('admin.desktop.updates.publish')" @click="openPublish(row)">
                <Icon name="upload" size="sm" />
              </button>
              <button v-if="row.status === 'published'" class="action-button text-red-600 dark:text-red-400" type="button" :title="t('admin.desktop.updates.withdraw')" :aria-label="t('admin.desktop.updates.withdraw')" @click="openWithdraw(row)">
                <Icon name="xCircle" size="sm" />
              </button>
            </div>
          </template>
          <template #empty>
            <EmptyState :title="t('admin.desktop.updates.empty')" :description="t('admin.desktop.updates.emptyDescription')" :action-text="t('admin.desktop.updates.create')" @action="openCreate" />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination v-if="pagination.total > 0" :page="pagination.page" :page-size="pagination.page_size" :total="pagination.total" @update:page="changePage" @update:page-size="changePageSize" />
      </template>
    </TablePageLayout>

    <BaseDialog :show="showDraft" :title="draftTitle" width="extra-wide" @close="closeDraft">
      <form id="desktop-update-draft-form" class="space-y-6" @submit.prevent="saveDraft">
        <div class="grid grid-cols-1 gap-4 lg:grid-cols-[14rem_minmax(0,1fr)]">
          <Input v-model="form.version" :label="t('admin.desktop.updates.version')" placeholder="0.3.0" required :error="errors.version" />
          <div>
            <label class="input-label mb-1.5 block" for="desktop-update-notes">{{ t('admin.desktop.updates.notes') }}</label>
            <textarea id="desktop-update-notes" v-model="form.notes" rows="3" maxlength="20000" class="input w-full resize-y" :placeholder="t('admin.desktop.updates.notesPlaceholder')"></textarea>
            <p v-if="errors.notes" class="input-error-text mt-1.5">{{ errors.notes }}</p>
          </div>
        </div>

        <div class="divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700">
          <section v-for="platform in platforms" :key="platform.key" class="grid grid-cols-1 gap-4 py-5 lg:grid-cols-[10rem_minmax(16rem,1fr)_minmax(18rem,0.8fr)]">
            <div>
              <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ platform.label }}</h4>
              <p class="mt-1 font-mono text-xs text-gray-500 dark:text-dark-400">{{ platform.bundle }}</p>
            </div>
            <div>
              <label class="input-label mb-1.5 block">{{ t('admin.desktop.updates.artifactFile') }}</label>
              <div class="rounded-md border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/60">
                <div class="flex min-w-0 items-center gap-3">
                  <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-white text-gray-500 shadow-sm dark:bg-dark-700 dark:text-dark-300">
                    <Icon name="document" size="sm" aria-hidden="true" />
                  </span>
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ artifactDisplayName(platform.key) }}</p>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ artifactDisplayMeta(platform.key) }}</p>
                  </div>
                  <label class="btn btn-secondary btn-sm shrink-0 cursor-pointer" :class="{ 'pointer-events-none opacity-50': saving }">
                    <input type="file" class="sr-only" :disabled="saving" @change="selectArtifactFile(platform.key, $event)" />
                    <Icon name="upload" size="sm" class="mr-1.5" aria-hidden="true" />
                    {{ form.artifacts[platform.key].url || pendingFiles[platform.key] ? t('admin.desktop.updates.replaceFile') : t('admin.desktop.updates.selectFile') }}
                  </label>
                  <button
                    v-if="form.artifacts[platform.key].url || pendingFiles[platform.key]"
                    type="button"
                    class="action-button shrink-0 text-red-600 dark:text-red-400"
                    :title="t('common.remove')"
                    :aria-label="t('common.remove')"
                    :disabled="saving"
                    @click="clearArtifactFile(platform.key)"
                  >
                    <Icon name="trash" size="sm" aria-hidden="true" />
                  </button>
                </div>
                <div v-if="uploadProgress[platform.key] !== null" class="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                  <div class="h-full bg-primary-600 transition-[width] duration-150" :style="{ width: `${uploadProgress[platform.key]}%` }"></div>
                </div>
              </div>
              <p v-if="errors[`${platform.key}.file`]" class="input-error-text mt-1.5">{{ errors[`${platform.key}.file`] }}</p>
            </div>
            <div>
              <label class="input-label mb-1.5 block" :for="`desktop-update-signature-${platform.key}`">{{ t('admin.desktop.updates.signature') }}</label>
              <textarea :id="`desktop-update-signature-${platform.key}`" v-model="form.artifacts[platform.key].signature" rows="3" maxlength="8192" class="input w-full resize-y font-mono text-xs"></textarea>
              <p v-if="errors[`${platform.key}.signature`]" class="input-error-text mt-1.5">{{ errors[`${platform.key}.signature`] }}</p>
            </div>
          </section>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="closeDraft">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="submit" form="desktop-update-draft-form" :disabled="saving">{{ saving ? t('admin.desktop.updates.uploading') : t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showPublish" :title="t('admin.desktop.updates.publish')" width="narrow" @close="closePublish">
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('admin.desktop.updates.publishConfirm', { version: selected?.version }) }}</p>
      <p class="mt-3 text-sm font-medium text-amber-700 dark:text-amber-300">{{ t('admin.desktop.updates.immutableHint') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="closePublish">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="button" :disabled="saving" @click="publishRelease">{{ t('admin.desktop.updates.publish') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showWithdraw" :title="t('admin.desktop.updates.withdraw')" width="narrow" @close="closeWithdraw">
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('admin.desktop.updates.withdrawConfirm', { version: selected?.version }) }}</p>
      <div class="mt-4">
        <label class="input-label mb-1.5 block" for="desktop-update-withdraw-reason">{{ t('admin.desktop.updates.withdrawalReason') }} <span class="text-red-500">*</span></label>
        <textarea id="desktop-update-withdraw-reason" v-model="withdrawalReason" rows="3" maxlength="500" class="input w-full resize-y" required></textarea>
        <p v-if="withdrawalError" class="input-error-text mt-1.5">{{ withdrawalError }}</p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="closeWithdraw">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger" type="button" :disabled="saving" @click="withdrawRelease">{{ t('admin.desktop.updates.withdraw') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { DesktopUpdateArtifact, DesktopUpdateArtifacts, DesktopUpdatePlatform, DesktopUpdateRelease, DesktopUpdateStatus } from '@/api/admin/desktop'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const releases = ref<DesktopUpdateRelease[]>([])
const loading = ref(false)
const saving = ref(false)
const status = ref<DesktopUpdateStatus | ''>('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const showDraft = ref(false)
const showPublish = ref(false)
const showWithdraw = ref(false)
const selected = ref<DesktopUpdateRelease | null>(null)
const withdrawalReason = ref('')
const withdrawalError = ref('')
const errors = reactive<Record<string, string>>({})
const pendingFiles = reactive<Record<DesktopUpdatePlatform, File | null>>({
  'darwin-aarch64': null,
  'darwin-x86_64': null,
  'windows-x86_64': null,
})
const uploadProgress = reactive<Record<DesktopUpdatePlatform, number | null>>({
  'darwin-aarch64': null,
  'darwin-x86_64': null,
  'windows-x86_64': null,
})
let listController: AbortController | undefined

const platforms: Array<{ key: DesktopUpdatePlatform; label: string; shortLabel: string; bundle: string }> = [
  { key: 'darwin-aarch64', label: 'macOS Apple Silicon', shortLabel: 'macOS ARM64', bundle: '.dmg / .app' },
  { key: 'darwin-x86_64', label: 'macOS Intel', shortLabel: 'macOS x64', bundle: '.dmg / .app' },
  { key: 'windows-x86_64', label: 'Windows x64', shortLabel: 'Windows x64', bundle: '.msi / .exe' },
]

function emptyArtifact(signature = ''): DesktopUpdateArtifact {
  return { url: '', signature, object_key: '', file_name: '', size: 0, sha256: '' }
}

function emptyArtifacts(): DesktopUpdateArtifacts {
  return {
    'darwin-aarch64': emptyArtifact(),
    'darwin-x86_64': emptyArtifact(),
    'windows-x86_64': emptyArtifact(),
  }
}

function cloneArtifacts(artifacts: DesktopUpdateArtifacts): DesktopUpdateArtifacts {
  return {
    'darwin-aarch64': { ...artifacts['darwin-aarch64'] },
    'darwin-x86_64': { ...artifacts['darwin-x86_64'] },
    'windows-x86_64': { ...artifacts['windows-x86_64'] },
  }
}

const form = reactive({ version: '', notes: '', artifacts: emptyArtifacts() })
const draftTitle = computed(() => selected.value ? t('admin.desktop.updates.edit') : t('admin.desktop.updates.create'))
const columns = computed<Column[]>(() => [
  { key: 'version', label: t('admin.desktop.updates.version') },
  { key: 'status', label: t('common.status') },
  { key: 'artifacts', label: t('admin.desktop.updates.artifacts') },
  { key: 'published_at', label: t('admin.desktop.updates.publishedAt') },
  { key: 'updated_at', label: t('admin.desktop.updatedAt') },
  { key: 'actions', label: t('common.actions') },
])
const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'draft', label: t('admin.desktop.updates.status.draft') },
  { value: 'published', label: t('admin.desktop.updates.status.published') },
  { value: 'withdrawn', label: t('admin.desktop.updates.status.withdrawn') },
])

function errorMessage(error: unknown): string {
  const reason = (error as { reason?: string })?.reason
  return reason ? t(`admin.desktop.errors.${reason}`) : (error as { message?: string })?.message || t('admin.desktop.errors.UNKNOWN')
}
function statusLabel(value: DesktopUpdateStatus) { return t(`admin.desktop.updates.status.${value}`) }
function statusVariant(value: DesktopUpdateStatus) { return value === 'published' ? 'success' : value === 'draft' ? 'warning' : 'disabled' }

async function loadReleases() {
  listController?.abort(); listController = new AbortController(); loading.value = true
  try {
    const result = await adminAPI.desktop.listUpdateReleases(pagination.page, pagination.page_size, status.value, listController.signal)
    releases.value = result.items; pagination.total = result.total
  } catch (error) {
    if ((error as { code?: string })?.code !== 'ERR_CANCELED') appStore.showError(errorMessage(error))
  } finally { loading.value = false }
}
function resetAndLoad() { pagination.page = 1; void loadReleases() }
function changePage(page: number) { pagination.page = page; void loadReleases() }
function changePageSize(pageSize: number) { pagination.page_size = pageSize; pagination.page = 1; void loadReleases() }

function openCreate() {
  selected.value = null; Object.assign(form, { version: '', notes: '', artifacts: emptyArtifacts() }); resetPendingUploads(); clearErrors(); showDraft.value = true
}
function openEdit(release: DesktopUpdateRelease) {
  selected.value = release
  Object.assign(form, { version: release.version, notes: release.notes, artifacts: cloneArtifacts(release.artifacts) })
  resetPendingUploads(); clearErrors(); showDraft.value = true
}
function closeDraft() { if (!saving.value) showDraft.value = false }
function clearErrors() { for (const key of Object.keys(errors)) delete errors[key] }

function resetPendingUploads() {
  for (const platform of platforms) {
    pendingFiles[platform.key] = null
    uploadProgress[platform.key] = null
  }
}

function selectArtifactFile(platform: DesktopUpdatePlatform, event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  delete errors[`${platform}.file`]
  if (!file) return
  pendingFiles[platform] = file
  uploadProgress[platform] = null
}

function clearArtifactFile(platform: DesktopUpdatePlatform) {
  pendingFiles[platform] = null
  uploadProgress[platform] = null
  form.artifacts[platform] = emptyArtifact(form.artifacts[platform].signature)
}

function formatFileSize(bytes: number): string {
  if (!bytes) return t('common.notAvailable')
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`
}

function artifactDisplayName(platform: DesktopUpdatePlatform): string {
  return pendingFiles[platform]?.name || form.artifacts[platform].file_name || t('admin.desktop.updates.noFileSelected')
}

function artifactDisplayMeta(platform: DesktopUpdatePlatform): string {
  const pending = pendingFiles[platform]
  if (pending) return `${formatFileSize(pending.size)} / ${t('admin.desktop.updates.pendingUpload')}`
  const artifact = form.artifacts[platform]
  if (artifact.url) return `${formatFileSize(artifact.size)} / SHA-256 ${artifact.sha256.slice(0, 12)}`
  return t('admin.desktop.updates.bundleHint', { type: platform === 'windows-x86_64' ? '.msi / .exe' : '.dmg / .app' })
}

function validateDraft(): boolean {
  clearErrors()
  const nextVersion = form.version.trim()
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(nextVersion)) errors.version = t('admin.desktop.updates.validation.version')
  if ([...form.notes].length > 20000) errors.notes = t('admin.desktop.updates.validation.notes')
  const versionChanged = selected.value !== null && nextVersion !== selected.value.version
  for (const platform of platforms) {
    const artifact = form.artifacts[platform.key]
    if (artifact.signature.trim().length > 8192) errors[`${platform.key}.signature`] = t('admin.desktop.updates.validation.signatureMax')
    if (versionChanged && artifact.url && !pendingFiles[platform.key]) {
      errors[`${platform.key}.file`] = t('admin.desktop.updates.validation.versionChangedReupload')
    }
  }
  return Object.keys(errors).length === 0
}

function draftInput(artifacts = form.artifacts) {
  return {
    version: form.version.trim(),
    notes: form.notes.trim(),
    artifacts: cloneArtifacts(artifacts),
  }
}

function versionTransitionArtifacts(): DesktopUpdateArtifacts {
  return {
    'darwin-aarch64': emptyArtifact(form.artifacts['darwin-aarch64'].signature),
    'darwin-x86_64': emptyArtifact(form.artifacts['darwin-x86_64'].signature),
    'windows-x86_64': emptyArtifact(form.artifacts['windows-x86_64'].signature),
  }
}

function hasPendingUploads(): boolean {
  return platforms.some(({ key }) => pendingFiles[key] !== null)
}

async function uploadPendingArtifacts(publicID: string) {
  for (const platform of platforms) {
    const file = pendingFiles[platform.key]
    if (!file) continue
    uploadProgress[platform.key] = 0
    const signature = form.artifacts[platform.key].signature
    const artifact = await adminAPI.desktop.uploadUpdateArtifact(publicID, platform.key, file, (percent) => {
      uploadProgress[platform.key] = percent
    })
    form.artifacts[platform.key] = { ...artifact, signature }
    pendingFiles[platform.key] = null
    uploadProgress[platform.key] = 100
  }
}

async function saveDraft() {
  if (!validateDraft()) return
  saving.value = true
  const wasCreate = !selected.value
  try {
    if (!selected.value) {
      selected.value = await adminAPI.desktop.createUpdateRelease(draftInput())
      if (hasPendingUploads()) {
        await uploadPendingArtifacts(selected.value.public_id)
        selected.value = await adminAPI.desktop.updateUpdateRelease(selected.value.public_id, draftInput())
      }
    } else {
      const versionChanged = form.version.trim() !== selected.value.version
      if (versionChanged) {
        selected.value = await adminAPI.desktop.updateUpdateRelease(selected.value.public_id, draftInput(versionTransitionArtifacts()))
      }
      const uploaded = hasPendingUploads()
      await uploadPendingArtifacts(selected.value.public_id)
      if (!versionChanged || uploaded) {
        selected.value = await adminAPI.desktop.updateUpdateRelease(selected.value.public_id, draftInput())
      }
    }
    appStore.showSuccess(t(wasCreate ? 'admin.desktop.updates.created' : 'admin.desktop.updates.updated'))
    showDraft.value = false; await loadReleases()
  } catch (error) { appStore.showError(errorMessage(error)) }
  finally { saving.value = false }
}

function hasCompleteArtifacts(release: DesktopUpdateRelease): boolean {
  return platforms.every(({ key }) => {
    const artifact = release.artifacts[key]
    return artifact.url.trim() !== '' && artifact.object_key.trim() !== '' && artifact.sha256.trim() !== '' && artifact.size > 0 && artifact.signature.trim() !== ''
  })
}

function openPublish(release: DesktopUpdateRelease) {
  if (!hasCompleteArtifacts(release)) {
    appStore.showError(t('admin.desktop.updates.validation.publishArtifacts'))
    return
  }
  selected.value = release
  showPublish.value = true
}
function closePublish() { if (!saving.value) showPublish.value = false }
async function publishRelease() {
  if (!selected.value) return
  saving.value = true
  try { await adminAPI.desktop.publishUpdateRelease(selected.value.public_id); appStore.showSuccess(t('admin.desktop.updates.published')); showPublish.value = false; await loadReleases() }
  catch (error) { appStore.showError(errorMessage(error)) }
  finally { saving.value = false }
}

function openWithdraw(release: DesktopUpdateRelease) { selected.value = release; withdrawalReason.value = ''; withdrawalError.value = ''; showWithdraw.value = true }
function closeWithdraw() { if (!saving.value) showWithdraw.value = false }
async function withdrawRelease() {
  if (!selected.value) return
  const reason = withdrawalReason.value.trim()
  if (!reason) { withdrawalError.value = t('admin.desktop.updates.validation.withdrawalReason'); return }
  saving.value = true
  try { await adminAPI.desktop.withdrawUpdateRelease(selected.value.public_id, reason); appStore.showSuccess(t('admin.desktop.updates.withdrawn')); showWithdraw.value = false; await loadReleases() }
  catch (error) { appStore.showError(errorMessage(error)) }
  finally { saving.value = false }
}

onMounted(loadReleases)
onBeforeUnmount(() => listController?.abort())
</script>

<style scoped>
.action-button {
  @apply flex h-10 w-10 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white;
}
</style>
