<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <form class="flex flex-wrap items-center gap-3" @submit.prevent="resetAndLoad">
          <input v-model="search" class="input w-56" :placeholder="t('admin.desktop.resources.search')" :aria-label="t('admin.desktop.resources.search')" maxlength="200" />
          <Select v-model="kind" class="w-36" :options="kindOptions" :aria-label="t('admin.desktop.resources.kind')" @change="resetAndLoad" />
          <Select v-model="status" class="w-36" :options="statusOptions" :aria-label="t('common.status')" @change="resetAndLoad" />
          <button class="btn btn-secondary" type="submit">{{ t('common.search') }}</button>
          <button class="btn btn-primary ml-auto" type="button" @click="showUpload = true">{{ t('admin.desktop.resources.upload') }}</button>
        </form>
      </template>
      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading" row-key="id">
          <template #cell-name="{ row }">
            <div class="max-w-80">
              <button class="text-left font-medium text-primary-600 hover:underline" type="button" @click="openDetail(row.id)">{{ row.name }}</button>
              <p class="mt-1 break-all font-mono text-xs text-gray-500">{{ row.key }}</p>
              <p class="mt-1 line-clamp-2 text-xs text-gray-500">{{ row.description }}</p>
            </div>
          </template>
          <template #cell-kind="{ value }">{{ value === 'mcp' ? 'MCP' : 'Skill' }}</template>
          <template #cell-artifacts="{ row }"><div class="flex max-w-64 flex-wrap gap-1"><span v-for="artifact in row.artifacts" :key="artifact.platform" class="rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ artifact.platform }}</span></div></template>
          <template #cell-status="{ row }"><StatusBadge :status="row.status === 'active' ? 'success' : 'disabled'" :label="t(`admin.desktop.resources.${row.status}`)" /></template>
          <template #cell-updatedAt="{ value }">{{ formatDateTime(value) }}</template>
          <template #cell-actions="{ row }">
            <button type="button" class="btn btn-secondary btn-sm" @click="statusTarget = row; statusReason = ''">{{ t(`admin.desktop.resources.${row.status === 'active' ? 'disable' : 'restore'}`) }}</button>
          </template>
          <template #empty><EmptyState :title="t('admin.desktop.resources.empty')" /></template>
        </DataTable>
      </template>
      <template #pagination><Pagination v-if="total > 0" :page="page" :page-size="pageSize" :total="total" @update:page="changePage" @update:page-size="changePageSize" /></template>
    </TablePageLayout>

    <BaseDialog :show="showUpload" :title="t('admin.desktop.resources.upload')" width="wide" @close="closeUpload">
      <div class="space-y-4">
        <p class="text-sm text-gray-500">{{ t('admin.desktop.resources.uploadHint') }}</p>
        <input class="max-w-full" type="file" accept=".zip,application/zip" :disabled="busy" :aria-label="t('admin.desktop.resources.selectZIP')" @change="selectFile" />
        <button v-if="selectedFile && !preview && !busy" type="button" class="btn btn-secondary btn-sm" @click="validateSelectedFile">{{ t('admin.desktop.resources.retryValidation') }}</button>
        <div v-if="busy" class="space-y-1">
          <p class="text-sm" role="status">{{ t(`admin.desktop.resources.${phase}`) }} · {{ progress }}%</p>
          <progress class="h-2 w-full" max="100" :value="progress" :aria-label="t(`admin.desktop.resources.${phase}`)"></progress>
        </div>
        <div v-if="preview" class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <p class="font-semibold">{{ preview.manifest.name }} <span class="font-mono">{{ preview.manifest.version }}</span></p>
          <p class="text-sm">{{ preview.manifest.key }} · {{ preview.manifest.kind }} · {{ preview.manifest.platform }} · {{ bytes(preview.sizeBytes) }}</p>
          <p class="text-sm text-gray-500">{{ preview.manifest.description }}</p>
          <p class="text-sm">{{ t('admin.desktop.resources.targets') }}: {{ preview.manifest.targets.join(', ') }}</p>
          <p class="break-all font-mono text-xs">SHA-256: {{ preview.sha256 }}</p>
          <p v-if="preview.current" class="text-sm">{{ t('admin.desktop.resources.currentVersion') }}: {{ preview.current.version }} · {{ t(`admin.desktop.resources.${preview.current.status}`) }}</p>
          <p class="text-sm text-amber-700 dark:text-amber-300">{{ t('admin.desktop.resources.versionHint') }}</p>
          <p v-if="preview.current?.status === 'disabled'" class="text-sm text-amber-700 dark:text-amber-300">{{ t('admin.desktop.resources.disabledUploadHint') }}</p>
        </div>
        <p v-if="uploadError" class="break-words text-sm text-red-600" role="alert">{{ uploadError }}</p>
        <p v-if="pending" class="text-sm text-amber-700 dark:text-amber-300" role="status">{{ t('admin.desktop.resources.pending') }}</p>
        <p v-if="published" class="text-sm text-green-700 dark:text-green-300" role="status">{{ t('admin.desktop.resources.published') }}</p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="closeUpload">{{ t('common.close') }}</button>
          <button v-if="pending" type="button" class="btn btn-primary" :disabled="busy" @click="verifyPublication">{{ t('admin.desktop.resources.verify') }}</button>
          <button v-else-if="!published" type="button" class="btn btn-primary" :disabled="busy || !preview" @click="publish">{{ t('admin.desktop.resources.confirmPublish') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showDetail" :title="t('admin.desktop.resources.detail')" width="extra-wide" @close="showDetail = false">
      <div v-if="detail" class="space-y-5">
        <div class="space-y-1">
          <h3 class="font-semibold">{{ detail.name }} · {{ detail.version }}</h3>
          <p class="break-all font-mono text-xs">{{ detail.id }} · {{ detail.key }}</p>
          <p class="text-sm">{{ detail.description }}</p>
          <p class="text-sm">{{ t('common.status') }}: {{ t(`admin.desktop.resources.${detail.status}`) }}</p>
          <p v-if="detail.statusReason" class="text-sm text-gray-500">{{ detail.statusReason }}</p>
        </div>
        <p class="text-sm text-amber-700 dark:text-amber-300">{{ t('admin.desktop.resources.privateHint') }}</p>
        <p v-if="detailLoading" role="status">{{ t('common.loading') }}</p>
        <section v-for="version in versions" :key="version.version" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700">
          <h4 class="font-medium">{{ version.version }} <span class="text-xs font-normal text-gray-500">{{ formatDateTime(version.createdAt) }} · {{ t('admin.desktop.resources.publisher') }} #{{ version.createdBy }}</span></h4>
          <div v-for="artifact in version.artifacts" :key="artifact.platform" class="space-y-2 rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class="text-sm font-medium">{{ artifact.platform }} · {{ bytes(artifact.sizeBytes) }}</span>
              <button class="btn btn-secondary btn-sm" type="button" :disabled="detail.status !== 'active' || downloadPending" @click="downloadArtifact(detail.id, version.version, artifact.platform)">{{ t('admin.desktop.resources.download') }}</button>
            </div>
            <p class="break-all font-mono text-xs">SHA-256: {{ artifact.sha256 }}</p>
            <p class="text-xs">{{ t('admin.desktop.resources.targets') }}: {{ artifact.manifest.targets.join(', ') }}</p>
            <details><summary class="cursor-pointer text-sm">Manifest</summary><pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all text-xs">{{ JSON.stringify(artifact.manifest, null, 2) }}</pre></details>
          </div>
        </section>
        <Pagination v-if="versionTotal > 20" :page="versionPage" :page-size="20" :total="versionTotal" :show-page-size-selector="false" @update:page="loadVersions" />
      </div>
    </BaseDialog>

    <BaseDialog :show="!!statusTarget" :title="t(`admin.desktop.resources.${statusTarget?.status === 'active' ? 'disable' : 'restore'}`)" width="narrow" @close="closeStatus">
      <div class="space-y-4">
        <p class="font-medium">{{ statusTarget?.name }}</p>
        <p class="text-sm text-gray-500">{{ t('admin.desktop.resources.statusHint') }}</p>
        <label v-if="statusTarget?.status === 'active'" class="block text-sm">{{ t('admin.desktop.resources.reason') }}<textarea v-model="statusReason" class="input mt-2 w-full" maxlength="500" rows="3" :disabled="statusSaving"></textarea></label>
      </div>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" :disabled="statusSaving" @click="closeStatus">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="button" :disabled="statusSaving || (statusTarget?.status === 'active' && !statusReason.trim())" @click="saveStatus">{{ t('common.confirm') }}</button></div></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import * as api from '@/api/admin/desktopResources'
import type { ResourceKind, ResourceStatus, ResourceRecord, ResourceValidation, ResourceVersion } from '@/api/admin/desktopResources'
import { useAppStore } from '@/stores/app'
import { formatBytes, formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import Select from '@/components/common/Select.vue'

const { t } = useI18n()
const app = useAppStore()
const search = ref(''), kind = ref<ResourceKind | ''>(''), status = ref<ResourceStatus | ''>('')
const page = ref(1), pageSize = ref(20), total = ref(0), loading = ref(false)
const items = ref<ResourceRecord[]>([])
const showUpload = ref(false), busy = ref(false), progress = ref(0), phase = ref('validating')
const selectedFile = ref<File | null>(null), preview = ref<ResourceValidation | null>(null)
const uploadError = ref(''), pending = ref(false), published = ref(false)
const showDetail = ref(false), detail = ref<ResourceRecord | null>(null), detailLoading = ref(false)
const versions = ref<ResourceVersion[]>([]), versionPage = ref(1), versionTotal = ref(0)
const downloadPending = ref(false)
const statusTarget = ref<ResourceRecord | null>(null), statusReason = ref(''), statusSaving = ref(false)
let listController: AbortController | undefined
let detailRequest = 0
const kindOptions = computed(() => [{ value: '', label: t('admin.desktop.resources.allKinds') }, { value: 'mcp', label: 'MCP' }, { value: 'skill', label: 'Skill' }])
const statusOptions = computed(() => [{ value: '', label: t('admin.desktop.resources.allStatuses') }, ...(['active', 'disabled'] as const).map(value => ({ value, label: t(`admin.desktop.resources.${value}`) }))])
const columns = computed(() => [
  { key: 'name', label: t('common.name') }, { key: 'kind', label: t('admin.desktop.resources.kind') },
  { key: 'version', label: t('admin.desktop.resources.currentVersion') }, { key: 'artifacts', label: t('admin.desktop.resources.platforms') },
  { key: 'status', label: t('common.status') }, { key: 'updatedAt', label: t('admin.desktop.resources.updatedAt') }, { key: 'actions', label: t('common.actions') },
])
function bytes(value: number) { return formatBytes(value) }
function errorMessage(error: unknown) { return api.resourceError(error).message || t('admin.desktop.resources.failed') }
async function load() {
  listController?.abort()
  const controller = new AbortController(); listController = controller; loading.value = true
  try {
    const result = await api.listResources({ page: page.value, page_size: pageSize.value, search: search.value, kind: kind.value, status: status.value }, controller.signal)
    if (!controller.signal.aborted) { items.value = result.items; total.value = result.total }
  } catch (error) { if (!controller.signal.aborted) app.showError(errorMessage(error)) }
  finally { if (!controller.signal.aborted) loading.value = false }
}
function resetAndLoad() { page.value = 1; void load() }
function changePage(value: number) { page.value = value; void load() }
function changePageSize(value: number) { pageSize.value = Math.min(value, 100); resetAndLoad() }
function closeUpload() { if (!busy.value) showUpload.value = false }
async function selectFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  selectedFile.value = file ?? null; preview.value = null; pending.value = false; published.value = false; uploadError.value = ''
  if (!file) return
  if (file.size > 64 * 1024 * 1024 || file.size === 0) { uploadError.value = t('admin.desktop.resources.sizeError'); return }
  await validateSelectedFile()
}
async function validateSelectedFile() {
  const file = selectedFile.value
  if (!file || busy.value || file.size === 0 || file.size > 64 * 1024 * 1024) return
  uploadError.value = ''
  busy.value = true; phase.value = 'validating'; progress.value = 0
  try { preview.value = await api.validateResource(file, value => { progress.value = value }) }
  catch (error) { uploadError.value = errorMessage(error) }
  finally { busy.value = false }
}
async function verifyPublication() {
  if (!preview.value) return
  busy.value = true; phase.value = 'verifying'; uploadError.value = ''
  try {
    const found = await api.findPublishedResource(preview.value)
    published.value = !!found; pending.value = !found
    if (found) { app.showSuccess(t('admin.desktop.resources.published')); await load() }
  } catch (error) { pending.value = true; uploadError.value = errorMessage(error) }
  finally { busy.value = false }
}
async function publish() {
  if (!selectedFile.value || !preview.value || busy.value || published.value || pending.value) return
  busy.value = true; phase.value = 'publishing'; progress.value = 0; uploadError.value = ''
  try {
    await api.publishResource(selectedFile.value, value => { progress.value = value })
    published.value = true; app.showSuccess(t('admin.desktop.resources.published')); await load()
  } catch (error) {
    const failure = api.resourceError(error)
    if (failure.status === 0 || failure.status === 409 || failure.status >= 500) { pending.value = true; await verifyPublication() }
    else uploadError.value = errorMessage(error)
  } finally { busy.value = false }
}
async function openDetail(id: string) {
  const request = ++detailRequest
  try {
    const result = await api.getResource(id)
    if (request !== detailRequest) return
    detail.value = result; versions.value = []; versionTotal.value = 0; showDetail.value = true; await loadVersions(1)
  } catch (error) { app.showError(errorMessage(error)) }
}
async function loadVersions(value: number) {
  if (!detail.value) return
  const id = detail.value.id, request = ++detailRequest
  detailLoading.value = true
  try {
    const result = await api.listResourceVersions(id, value)
    if (request !== detailRequest) return
    versions.value = result.items; versionTotal.value = result.total; versionPage.value = value
  } catch (error) { if (request === detailRequest) app.showError(errorMessage(error)) }
  finally { if (request === detailRequest) detailLoading.value = false }
}
function closeStatus() { if (!statusSaving.value) statusTarget.value = null }
async function saveStatus() {
  if (!statusTarget.value || statusSaving.value) return
  statusSaving.value = true
  try {
    const result = await api.setResourceStatus(statusTarget.value.id, statusTarget.value.status === 'active' ? 'disabled' : 'active', statusReason.value)
    if (detail.value?.id === result.id) detail.value = result
    statusTarget.value = null; app.showSuccess(t('admin.desktop.resources.statusSaved')); await load()
  } catch (error) { app.showError(errorMessage(error)) }
  finally { statusSaving.value = false }
}
async function downloadArtifact(id: string, version: string, platform: string) {
  if (downloadPending.value || detail.value?.status !== 'active') return
  downloadPending.value = true
  try {
    const link = await api.getResourceDownloadURL(id, version, platform)
    const anchor = document.createElement('a')
    anchor.href = link.url
    anchor.referrerPolicy = 'no-referrer'
    anchor.rel = 'noreferrer'
    anchor.click()
  } catch (error) { app.showError(errorMessage(error)) }
  finally { downloadPending.value = false }
}
onMounted(load)
onBeforeUnmount(() => { listController?.abort(); detailRequest++ })
</script>
