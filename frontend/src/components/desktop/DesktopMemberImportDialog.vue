<template>
  <section class="card structure-panel space-y-6 p-4 sm:p-6">
    <div class="space-y-4" :aria-busy="loading || importing">
      <h2 class="structure-section-title">{{ t('desktopStructure.importFileTitle') }}</h2>
      <div class="flex flex-wrap items-start justify-between gap-3">
        <p class="min-w-0 flex-1 basis-full text-sm leading-relaxed text-gray-600 dark:text-dark-300 sm:basis-0">{{ t('admin.desktop.memberImport.hint') }}</p>
        <button class="btn btn-secondary shrink-0 gap-1.5" type="button" :disabled="templateLoading" data-testid="download-member-template" @click="downloadTemplate"><Icon name="download" size="md" aria-hidden="true" />{{ t('admin.desktop.memberImport.downloadTemplate') }}</button>
      </div>
      <div class="space-y-1.5">
        <span id="member-import-file-label" class="input-label block">{{ t('admin.desktop.memberImport.file') }}</span>
        <div class="flex flex-wrap items-center gap-3">
          <button class="btn btn-secondary btn-sm shrink-0 gap-1.5" type="button" :disabled="loading || importing" aria-describedby="member-import-file-label member-import-file-name" data-testid="choose-member-import-file" @click="fileInput?.click()"><Icon name="upload" size="sm" :stroke-width="2" aria-hidden="true" />{{ t('common.chooseFile') }}</button>
          <span id="member-import-file-name" class="min-w-0 flex-1 break-all text-sm text-gray-500 dark:text-dark-400" role="status" aria-live="polite">{{ fileName || t('common.noFileSelected') }}</span>
        </div>
        <input ref="fileInput" type="file" accept=".xlsx,.csv" class="hidden" :disabled="loading || importing" aria-labelledby="member-import-file-label" data-testid="member-import-file" @change="selectFile" />
      </div>
      <p class="structure-caption">{{ t('desktopStructure.importHint') }}</p>
      <fieldset class="structure-section" :disabled="importing || structureLoading || !!structureError || !organizationActive">
        <legend class="structure-section-title mb-3">{{ t('desktopStructure.importAssignmentTitle') }}</legend>
        <p class="structure-caption">{{ t('desktopStructure.importFallback') }}</p>
        <div class="structure-grid">
          <label class="structure-field"><span>{{ t('desktopStructure.department') }}</span><select v-model="fallbackDepartment" name="import-department" class="input" aria-describedby="import-department-hint" @change="fallbackPosition = ''"><option value="">{{ t('desktopStructure.selectDepartment') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select><p id="import-department-hint" class="structure-caption">{{ departments.find(d => d.public_id === fallbackDepartment)?.name || (structureLoading || structureError ? '' : t(departments.length ? 'desktopStructure.selectDepartmentFirst' : 'desktopStructure.noDepartments')) }}</p></label>
          <label class="structure-field"><span>{{ t('desktopStructure.position') }}</span><select v-model="fallbackPosition" name="import-position" class="input" :disabled="!fallbackDepartment" aria-describedby="import-position-hint"><option value="">{{ t('desktopStructure.selectPosition') }}</option><option v-for="p in availablePositions" :key="p.public_id" :value="p.public_id">{{ p.name }}</option></select><p id="import-position-hint" class="structure-caption">{{ positions.find(p => p.public_id === fallbackPosition)?.name || (structureLoading || structureError ? '' : t(!fallbackDepartment ? 'desktopStructure.selectDepartmentFirst' : !availablePositions.length ? 'desktopStructure.noPositions' : 'desktopStructure.selectPosition')) }}</p></label>
        </div>
      </fieldset>
      <p class="structure-caption" role="status">{{ structureLoading ? t('common.loading') : '' }}</p>
      <div v-if="structureError" class="structure-error" role="alert"><p class="min-w-0 flex-1">{{ structureError }}</p><button type="button" class="btn btn-secondary" :disabled="importing" @click="loadStructure">{{ t('desktopStructure.reloadAction') }}</button></div>
      <p v-if="loading" class="text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
      <div v-if="fileError" class="structure-error" role="alert"><p>{{ fileError }} {{ t('desktopStructure.importStartHint') }}</p></div>
      <div v-if="!rows.length && !loading" class="structure-empty rounded-xl bg-gray-50 dark:bg-dark-900"><Icon name="upload" size="lg" aria-hidden="true" /><p class="structure-caption">{{ t('desktopStructure.importEmpty') }}</p></div>
      <template v-if="rows.length">
        <h2 class="structure-section-title">{{ t('desktopStructure.importPreviewTitle') }}</h2>
        <p class="text-sm text-gray-600 dark:text-dark-300" role="status" aria-live="polite" data-testid="member-import-summary">{{ t('admin.desktop.memberImport.summary', { total: rows.length, ready: pendingRows.length, invalid: invalidCount, success: successCount, failed: failedCount }) }}</p>
        <p v-if="capacityExceeded" class="text-sm text-amber-700 dark:text-amber-300" role="alert">{{ t('admin.desktop.memberImport.capacityExceeded', { available: availableSlots, required: pendingRows.length }) }}</p>
        <p class="detail-table-hint hidden md:block">{{ t('admin.desktop.detail.tableScrollHint') }}</p>
        <DataTable tabindex="0" :aria-label="t('admin.desktop.memberImport.button')" :columns="columns" :data="pageRows" row-key="rowNumber">
          <template #cell-name="{ row }"><span class="block max-w-40 whitespace-normal break-words">{{ row.input.name }}</span></template>
          <template #cell-phone="{ row }">{{ row.input.phone }}</template>
          <template #cell-department="{ row }"><span class="block max-w-48 whitespace-normal break-words">{{ positions.find(p => p.public_id === row.input.position_id)?.department?.name || row.departmentName || '—' }}</span></template>
          <template #cell-position="{ row }"><span class="block max-w-48 whitespace-normal break-words">{{ positions.find(p => p.public_id === row.input.position_id)?.name || row.positionName || '—' }}</span></template>
          <template #cell-remark="{ row }"><span class="block max-w-56 whitespace-pre-wrap break-words">{{ row.input.remark || '—' }}</span></template>
          <template #cell-status="{ row }"><span class="block max-w-64 whitespace-normal break-words" :class="row.errors.length || row.status === 'failed' ? 'text-red-600 dark:text-red-400' : row.status === 'success' ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-500 dark:text-dark-400'">{{ row.errors.length ? row.errors.map((key: string) => t(key === 'invalidDepartment' || key === 'invalidPosition' ? `desktopStructure.${key}` : `admin.desktop.memberImport.${key}`)).join('；') : row.error || t(`admin.desktop.memberImport.${row.status}`) }}</span></template>
        </DataTable>
        <Pagination dropdown-class="desktop-detail-select" v-if="rows.length > pageSize" v-model:page="page" :page-size="pageSize" :total="rows.length" :show-page-size-selector="false" />
      </template>
    </div>
    <footer class="border-t border-gray-200 pt-5 dark:border-dark-700">
      <div class="flex flex-wrap justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="close">{{ t('common.close') }}</button>
        <button class="btn btn-primary" type="button" :disabled="!canImport" data-testid="confirm-member-import" @click="runImport">{{ importing ? t('admin.desktop.memberImport.importing') : t(failedCount ? 'admin.desktop.memberImport.retry' : 'admin.desktop.memberImport.confirm', { count: pendingRows.length }) }}</button>
      </div>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { desktopStructure, type OrganizationUnit } from '@/api/desktopStructure'
import { normalizeDesktopStructureName as normalized } from '@/utils/desktopStructureName'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import * as adminDesktopAPI from '@/api/admin/desktop'
import managedDesktopAPI from '@/api/desktopOrganization'
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
  baseErrors: string[]
}
const props = defineProps<{ show: boolean; organizationId: string; selfManaged: boolean; availableSlots: number; organizationActive: boolean }>()
const emit = defineEmits<{ close: []; imported: []; busy: [value: boolean]; dirty: [value: boolean] }>()
const { t, te } = useI18n()
const rows = ref<ImportRow[]>([])
const loading = ref(false)
const importing = ref(false)
const templateLoading = ref(false)
const departments = ref<OrganizationUnit[]>([])
const positions = ref<OrganizationUnit[]>([])
const fallbackDepartment = ref('')
const fallbackPosition = ref('')
const structureLoading = ref(true), structureError = ref('')
const availablePositions = computed(() => positions.value.filter(p => p.department?.public_id === fallbackDepartment.value))
const attempted = ref(false)
const fileError = ref('')
const fileName = ref('')
const fileInput = ref<HTMLInputElement>()
const page = ref(1)
const pageSize = 20
let generation = 0, structureGeneration = 0
const pendingRows = computed(() => rows.value.filter(row => !row.errors.length && row.status !== 'success'))
const invalidCount = computed(() => rows.value.filter(row => row.errors.length).length)
const successCount = computed(() => rows.value.filter(row => row.status === 'success').length)
const failedCount = computed(() => rows.value.filter(row => row.status === 'failed').length)
const capacityExceeded = computed(() => !attempted.value && pendingRows.value.length > props.availableSlots)
const canImport = computed(() => props.organizationActive && Boolean(props.organizationId) && !structureLoading.value && !structureError.value && !loading.value && !importing.value && !capacityExceeded.value && pendingRows.value.length > 0)
const pageRows = computed(() => rows.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const columns = computed<Column[]>(() => [
  { key: 'rowNumber', label: t('admin.desktop.memberImport.row') },
  { key: 'name', label: t('admin.desktop.memberName') },
  { key: 'phone', label: t('admin.desktop.phone') },
  { key: 'department', label: t('desktopStructure.department') },
  { key: 'position', label: t('desktopStructure.position') },
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
    rows.value = parsed.map(row => ({ ...row, baseErrors: [...row.errors], status: 'ready', error: '', idempotencyKey: `desktop-member-import-${sessionID}-${row.rowNumber}` }))
    resolvePositions()
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
        await api.createMember(organizationID, { ...row.input, position_id: row.input.position_id! }, row.idempotencyKey)
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

function resolvePositions() {
  for (const row of rows.value) {
    if (row.status === 'success') continue
    row.errors = [...row.baseErrors]
    let position = fallbackPosition.value
    if (row.departmentName || row.positionName) {
      const ds = departments.value.filter(d => normalized(d.name) === normalized(row.departmentName || ''))
      if (ds.length !== 1) { row.errors.push('invalidDepartment'); position = '' }
      else {
        const ps = positions.value.filter(p => p.department?.public_id === ds[0].public_id && normalized(p.name) === normalized(row.positionName || ''))
        if (ps.length !== 1) { row.errors.push('invalidPosition'); position = '' }
        else position = ps[0].public_id
      }
    } else if (!position) row.errors.push('invalidPosition')
    if (position !== (row.input.position_id || '')) {
      row.input.position_id = position
      row.idempotencyKey = `desktop-member-import-${crypto.randomUUID()}`
      row.status = 'ready'; row.error = ''
    }
  }
}
watch(fallbackPosition, resolvePositions)
watch(importing, value => emit('busy', value))
watch(() => rows.value.some(row => row.status !== 'success'), value => emit('dirty', value))
async function loadStructure() {
  const id = props.organizationId, current = ++structureGeneration
  structureLoading.value = true; structureError.value = ''
  try {
    const api = desktopStructure(id, props.selfManaged)
    const [ds, ps] = await Promise.all([api.all('departments', { status: 'active' }), api.all('positions', { status: 'active' })])
    if (current === structureGeneration && id === props.organizationId) { departments.value = ds; positions.value = ps; resolvePositions() }
  } catch { if (current === structureGeneration) structureError.value = t('desktopStructure.failed') }
  finally { if (current === structureGeneration) structureLoading.value = false }
}
watch(() => [props.organizationId, props.selfManaged], () => { departments.value = []; positions.value = []; void loadStructure() }, { immediate: true })

onBeforeUnmount(() => { generation++; structureGeneration++ })
</script>
