<template>
  <div class="min-w-0 space-y-5">
    <DesktopConversationStatistics :organization-id="organizationId" :self-managed="selfManaged" :filters="appliedFilters" />
    <form class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4" @submit.prevent="applyFilters">
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.memberSearch') }}</span><input v-model="filters.member_search" class="input" type="search" maxlength="100" /></label>
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.client') }}</span><select v-model="filters.client" class="input"><option value="">{{ t('common.all') }}</option><option value="workbuddy">Workbuddy</option><option value="chatgpt_codex">ChatGPT Codex</option></select></label>
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.status') }}</span><select v-model="filters.capture_status" class="input"><option value="">{{ t('common.all') }}</option><option value="captured">{{ t('admin.desktop.conversations.captured') }}</option><option value="response_missing">{{ t('admin.desktop.conversations.responseMissing') }}</option></select></label>
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.recordID') }}</span><input v-model="filters.record_id" class="input font-mono" maxlength="36" /></label>
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.receivedFrom') }}</span><input v-model="filters.received_from" class="input" type="datetime-local" /></label>
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.receivedTo') }}</span><input v-model="filters.received_to" class="input" type="datetime-local" /></label>
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.sessionID') }}</span><input v-model="filters.source_session_id" class="input font-mono" maxlength="512" /></label>
      <label class="space-y-1 text-sm"><span>{{ t('admin.desktop.conversations.installationID') }}</span><input v-model="filters.installation_id" class="input font-mono" maxlength="36" /></label>
      <div class="flex flex-wrap gap-2 sm:col-span-2 lg:col-span-4"><button class="btn btn-primary" type="submit" :disabled="loading">{{ t('common.search') }}</button><button class="btn btn-secondary" type="button" @click="resetFilters">{{ t('common.reset') }}</button></div>
    </form>

    <div v-if="error" class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
      {{ error }} <button class="ml-3 underline" type="button" @click="loadRecords">{{ t('admin.desktop.conversations.retry') }}</button>
    </div>
    <div v-else :aria-busy="loading">
      <DataTable :columns="columns" :data="records" :loading="loading" row-key="record_id">
        <template #cell-member_name="{ row }"><div class="max-w-48 break-words">{{ row.member_name }}<span v-if="row.member_deleted" class="ml-2 text-xs text-gray-500">{{ t('admin.desktop.conversations.deletedMember') }}</span></div></template>
        <template #cell-client="{ value }">{{ clientLabel(value) }}</template>
        <template #cell-capture_status="{ value }"><span :class="value === 'response_missing' ? 'text-amber-700 dark:text-amber-400' : ''">{{ statusLabel(value) }}</span></template>
        <template #cell-received_at="{ value }"><span class="whitespace-nowrap">{{ formatDateTime(value) }}</span></template>
        <template #cell-source_session_id="{ value }"><span class="block max-w-48 truncate font-mono text-xs" :title="value">{{ value }}</span></template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1 whitespace-nowrap">
            <button class="btn btn-ghost btn-sm gap-1.5 text-primary-600 dark:text-primary-400" type="button" aria-haspopup="dialog" @click="openViewer(row, 'record')">
              <Icon name="eye" size="sm" aria-hidden="true" />{{ t('admin.desktop.conversations.viewDetail') }}
            </button>
            <button class="btn btn-ghost btn-sm gap-1.5" type="button" aria-haspopup="dialog" @click="openViewer(row, 'thread')">
              <Icon name="chat" size="sm" aria-hidden="true" />{{ t('admin.desktop.conversations.viewThread') }}
            </button>
          </div>
        </template>
        <template #empty><p class="py-4 text-center text-sm text-gray-500" role="status">{{ t(hasFilters ? 'admin.desktop.conversations.noMatches' : 'admin.desktop.conversations.empty') }}</p></template>
      </DataTable>
      <Pagination v-if="total > 0" :page="page" :page-size="pageSize" :total="total" :page-size-options="[20, 50, 100]" @update:page="changePage" @update:page-size="changePageSize" />
    </div>

    <DesktopConversationViewer
      :record="selected"
      :records="records"
      :mode="viewerMode"
      :organization-id="organizationId"
      :self-managed="selfManaged"
      @close="selected = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listDesktopConversations } from '@/api/desktopConversations'
import type { DesktopConversation, DesktopConversationFilters, DesktopConversationQuery } from '@/api/desktopConversations'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import type { Column } from '@/components/common/types'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import DesktopConversationViewer from './DesktopConversationViewer.vue'
import DesktopConversationStatistics from './DesktopConversationStatistics.vue'

const props = defineProps<{ organizationId: string; selfManaged: boolean }>()
const { t } = useI18n()
const emptyFilters = () => ({ member_search: '', client: '', capture_status: '', record_id: '', source_session_id: '', installation_id: '', received_from: '', received_to: '' })
const filters = reactive(emptyFilters())
const appliedFilters = ref<DesktopConversationFilters>({})
const records = ref<DesktopConversation[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const error = ref('')
const selected = ref<DesktopConversation | null>(null)
const viewerMode = ref<'record' | 'thread'>('record')
let listController: AbortController | undefined
const hasFilters = computed(() => Object.values(appliedFilters.value).some(Boolean))
const columns = computed<Column[]>(() => [
  { key: 'member_name', label: t('admin.desktop.member') },
  { key: 'client', label: t('admin.desktop.conversations.client') },
  { key: 'capture_status', label: t('admin.desktop.conversations.status') },
  { key: 'prompt_count', label: t('admin.desktop.conversations.promptCount') },
  { key: 'received_at', label: t('admin.desktop.conversations.receivedAt') },
  { key: 'source_session_id', label: t('admin.desktop.conversations.sessionID') },
  { key: 'actions', label: t('common.actions') },
])
function clientLabel(client: string) { return client === 'workbuddy' ? 'Workbuddy' : 'ChatGPT Codex' }
function statusLabel(status: string) { return t(status === 'captured' ? 'admin.desktop.conversations.captured' : 'admin.desktop.conversations.responseMissing') }
function requestError(cause: unknown) {
  const status = (cause as { response?: { status?: number } })?.response?.status
  if (status === 404) return t('admin.desktop.conversations.notFound')
  if (status === 422) return t('admin.desktop.conversations.invalidFilters')
  return t('admin.desktop.conversations.loadFailed')
}
function openViewer(record: DesktopConversation, mode: 'record' | 'thread') {
  viewerMode.value = mode
  selected.value = record
}
async function loadRecords() {
  listController?.abort()
  selected.value = null
  const controller = new AbortController()
  listController = controller
  records.value = []
  total.value = 0
  error.value = ''
  if (!props.organizationId) { loading.value = false; return }
  loading.value = true
  try {
    const query: DesktopConversationQuery = { ...appliedFilters.value, page: page.value, page_size: pageSize.value, sort_order: 'desc' }
    const result = await listDesktopConversations(props.organizationId, props.selfManaged, query, controller.signal)
    if (controller.signal.aborted) return
    records.value = result.items
    total.value = result.total
  } catch (cause) {
    if (!controller.signal.aborted) error.value = requestError(cause)
  } finally { if (!controller.signal.aborted) loading.value = false }
}
function applyFilters() {
  const next: DesktopConversationFilters = {}
  for (const [key, value] of Object.entries(filters)) {
    if (value.trim()) Object.assign(next, { [key]: key.startsWith('received_') ? new Date(value).toISOString() : value.trim() })
  }
  appliedFilters.value = next
  page.value = 1
  void loadRecords()
}
function resetFilters() { Object.assign(filters, emptyFilters()); applyFilters() }
function changePage(value: number) { page.value = value; void loadRecords() }
function changePageSize(value: number) { pageSize.value = value; page.value = 1; void loadRecords() }
watch(() => [props.organizationId, props.selfManaged], () => { resetFilters() }, { immediate: true })
onBeforeUnmount(() => { listController?.abort() })
</script>
