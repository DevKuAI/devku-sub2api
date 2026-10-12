<template>
  <BaseDialog content-class="desktop-detail" :close-label="t('common.close')" :show="show" :title="t('admin.desktop.memberImport.button')" width="wide" :show-close-button="!importing" :close-on-escape="!importing" @close="close">
    <div class="space-y-4" :aria-busy="loading || importing">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <p class="min-w-0 flex-1 basis-full text-sm leading-relaxed text-gray-600 dark:text-dark-300 sm:basis-0">{{ t('admin.desktop.memberImport.hint') }}</p>
        <button class="btn btn-secondary shrink-0 gap-1.5" type="button" :disabled="templateLoading" data-testid="download-member-template" @click="downloadTemplate"><Icon name="download" size="md" />{{ t('admin.desktop.memberImport.downloadTemplate') }}</button>
      </div>
      <div class="space-y-1.5">
        <span id="member-import-file-label" class="input-label block">{{ t('admin.desktop.memberImport.file') }}</span>
        <div class="flex flex-wrap items-center gap-3">
          <button class="btn btn-secondary btn-sm shrink-0 gap-1.5" type="button" :disabled="loading || importing" aria-describedby="member-import-file-label member-import-file-name" data-testid="choose-member-import-file" @click="fileInput?.click()"><Icon name="upload" size="sm" :stroke-width="2" aria-hidden="true" />{{ t('common.chooseFile') }}</button>
          <span id="member-import-file-name" class="min-w-0 flex-1 break-all text-sm text-gray-500 dark:text-dark-400" role="status" aria-live="polite">{{ fileName || t('common.noFileSelected') }}</span>
        </div>
        <input ref="fileInput" type="file" accept=".xlsx,.csv" class="hidden" :disabled="loading || importing" aria-labelledby="member-import-file-label" data-testid="member-import-file" @change="selectFile" />
      </div>
      <p v-if="loading" class="text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
      <p v-if="fileError" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ fileError }}</p>
      <template v-if="rows.length">
        <p class="text-sm text-gray-600 dark:text-dark-300" role="status" aria-live="polite" data-testid="member-import-summary">{{ t('admin.desktop.memberImport.summary', { total: rows.length, ready: pendingRows.length, invalid: invalidCount, success: successCount, failed: failedCount }) }}</p>
        <p v-if="capacityExceeded" class="text-sm text-amber-700 dark:text-amber-300" role="alert">{{ t('admin.desktop.memberImport.capacityExceeded', { available: availableSlots, required: pendingRows.length }) }}</p>
        <p class="detail-table-hint">{{ t('admin.desktop.detail.tableScrollHint') }}</p>
        <DataTable tabindex="0" :aria-label="t('admin.desktop.memberImport.button')" :columns="columns" :data="pageRows" row-key="rowNumber">
          <template #cell-name="{ row }"><span class="block max-w-40 whitespace-normal break-words">{{ row.input.name }}</span></template>
          <template #cell-phone="{ row }">{{ row.input.phone }}</template>
          <template #cell-remark="{ row }"><span class="block max-w-56 whitespace-pre-wrap break-words">{{ row.input.remark || '—' }}</span></template>
          <template #cell-status="{ row }"><span class="block max-w-64 whitespace-normal break-words" :class="row.errors.length || row.status === 'failed' ? 'text-red-600 dark:text-red-400' : row.status === 'success' ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-500 dark:text-dark-400'">{{ row.errors.length ? row.errors.map((key: string) => t(`admin.desktop.memberImport.${key}`)).join('；') : row.error || t(`admin.desktop.memberImport.${row.status}`) }}</span></template>
        </DataTable>
        <Pagination dropdown-class="desktop-detail-select" v-if="rows.length > pageSize" v-model:page="page" :page-size="pageSize" :total="rows.length" :show-page-size-selector="false" />
      </template>
    </div>
    <template #footer>
      <div class="flex flex-wrap justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="close">{{ t('common.close') }}</button>
        <button class="btn btn-primary" type="button" :disabled="!canImport" data-testid="confirm-member-import" @click="runImport">{{ importing ? t('admin.desktop.memberImport.importing') : t(failedCount ? 'admin.desktop.memberImport.retry' : 'admin.desktop.memberImport.confirm', { count: pendingRows.length }) }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import * as adminDesktopAPI from '@/api/admin/desktop'
import managedDesktopAPI from '@/api/desktopOrganization'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { createDesktopMemberImportTemplate, parseDesktopMemberImportFile } from '@/utils/desktopMemberImport'
import type { DesktopMemberImportRow } from '@/utils/desktopMemberImport'

interface ImportRow extends DesktopMemberImportRow {
  status: 'ready' | 'success' | 'failed'
  error: string
  idempotencyKey: string
}
const props = defineProps<{ show: boolean; organizationId: string; selfManaged: boolean; availableSlots: number; organizationActive: boolean }>()
const emit = defineEmits<{ close: []; imported: [] }>()
const { t, te } = useI18n()
const rows = ref<ImportRow[]>([])
const loading = ref(false)
const importing = ref(false)
const templateLoading = ref(false)
const attempted = ref(false)
const fileError = ref('')
const fileName = ref('')
const fileInput = ref<HTMLInputElement>()
const page = ref(1)
const pageSize = 20
let generation = 0
const pendingRows = computed(() => rows.value.filter(row => !row.errors.length && row.status !== 'success'))
const invalidCount = computed(() => rows.value.filter(row => row.errors.length).length)
const successCount = computed(() => rows.value.filter(row => row.status === 'success').length)
const failedCount = computed(() => rows.value.filter(row => row.status === 'failed').length)
const capacityExceeded = computed(() => !attempted.value && pendingRows.value.length > props.availableSlots)
const canImport = computed(() => props.organizationActive && Boolean(props.organizationId) && !loading.value && !importing.value && !capacityExceeded.value && pendingRows.value.length > 0)
const pageRows = computed(() => rows.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const columns = computed<Column[]>(() => [
  { key: 'rowNumber', label: t('admin.desktop.memberImport.row') },
  { key: 'name', label: t('admin.desktop.memberName') },
  { key: 'phone', label: t('admin.desktop.phone') },
  { key: 'remark', label: t('admin.desktop.memberRemark') },
  { key: 'status', label: t('admin.desktop.memberImport.result') },
])

function close() { if (!importing.value) emit('close') }
function reset() {
  generation++
  rows.value = []
  loading.value = false
  importing.value = false
  attempted.value = false
  fileError.value = ''
  fileName.value = ''
  page.value = 1
}
async function downloadTemplate() {
  if (templateLoading.value) return
  templateLoading.value = true
  try { saveAs(await createDesktopMemberImportTemplate(), 'desktop_members_template.xlsx') }
  catch { fileError.value = t('admin.desktop.memberImport.templateFailed') }
  finally { templateLoading.value = false }
}
async function selectFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || importing.value) return
  reset()
  fileName.value = file.name
  const current = generation
  loading.value = true
  try {
    const parsed = await parseDesktopMemberImportFile(file)
    if (current !== generation) return
    const sessionID = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
    rows.value = parsed.map(row => ({ ...row, status: 'ready', error: '', idempotencyKey: `desktop-member-import-${sessionID}-${row.rowNumber}` }))
  } catch (error) {
    if (current === generation) {
      const key = `admin.desktop.memberImport.${(error as Error).message}`
      fileError.value = t(te(key) ? key : 'admin.desktop.memberImport.invalidFile')
    }
  } finally { if (current === generation) loading.value = false }
}
async function runImport() {
  if (!canImport.value) return
  const current = generation
  const organizationID = props.organizationId
  const api = props.selfManaged ? managedDesktopAPI : adminDesktopAPI
  const pending = [...pendingRows.value]
  attempted.value = true
  importing.value = true
  try {
    for (const row of pending) {
      if (current !== generation) break
      try {
        await api.createMember(organizationID, row.input, row.idempotencyKey)
        if (current !== generation) break
        row.status = 'success'
        row.error = ''
      } catch (error) {
        if (current !== generation) break
        row.status = 'failed'
        const reason = (error as { reason?: string }).reason
        const key = `admin.desktop.errors.${reason}`
        row.error = t(reason && te(key) ? key : 'admin.desktop.memberImport.createFailed')
      }
    }
  } finally {
    if (current === generation) {
      importing.value = false
      emit('imported')
    }
  }
}
watch(() => [props.show, props.organizationId, props.selfManaged], reset, { flush: 'sync' })
onBeforeUnmount(() => { generation++ })
</script>
