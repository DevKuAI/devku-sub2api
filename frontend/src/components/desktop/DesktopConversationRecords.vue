<template>
  <div class="min-w-0 space-y-5">
    <div v-if="thread" class="flex flex-wrap items-center gap-3 rounded-lg bg-primary-50 p-3 dark:bg-primary-900/20" data-testid="thread-scope">
      <p class="min-w-0 flex-1 break-all text-sm">{{ t('admin.desktop.conversations.threadScope', { member: thread.member_name, client: clientLabel(thread.client), session: thread.source_session_id }) }}</p>
      <button class="btn btn-secondary" type="button" @click="resetFilters">{{ t('admin.desktop.conversations.exitThread') }}</button>
    </div>
    <form v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4" @submit.prevent="applyFilters">
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
        <template #cell-actions="{ row }"><div class="flex flex-wrap gap-2"><button class="btn btn-secondary px-2 py-1 text-sm" type="button" @click="openRecord(row)">{{ t('admin.desktop.conversations.viewDetail') }}</button><button v-if="!thread" class="btn btn-secondary px-2 py-1 text-sm" type="button" @click="openThread(row)">{{ t('admin.desktop.conversations.viewThread') }}</button></div></template>
        <template #empty><p class="py-4 text-center text-sm text-gray-500" role="status">{{ t(hasFilters ? 'admin.desktop.conversations.noMatches' : 'admin.desktop.conversations.empty') }}</p></template>
      </DataTable>
      <Pagination v-if="total > 0" :page="page" :page-size="pageSize" :total="total" :page-size-options="[20, 50, 100]" @update:page="changePage" @update:page-size="changePageSize" />
    </div>

    <section v-if="selected" class="min-w-0 space-y-4 border-t border-gray-200 pt-5 dark:border-dark-700" :aria-busy="detailLoading" :aria-label="t('admin.desktop.conversations.detail')" data-testid="conversation-detail">
      <div class="flex items-start gap-3"><div class="min-w-0 flex-1"><h3 class="font-semibold">{{ t('admin.desktop.conversations.detail') }}</h3><p class="mt-1 break-all font-mono text-xs text-gray-500">{{ selected.record_id }}</p></div><button class="btn btn-secondary" type="button" @click="closeDetail">{{ t('common.close') }}</button></div>
      <p v-if="detailLoading" role="status">{{ t('common.loading') }}</p>
      <p v-else-if="detailError" role="alert" class="text-sm text-red-600">{{ detailError }} <button class="ml-2 underline" type="button" @click="openRecord(selected)">{{ t('admin.desktop.conversations.retry') }}</button></p>
      <template v-else-if="detail">
        <dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
          <div v-for="item in detailMetadata" :key="item.label" class="min-w-0"><dt class="text-gray-500">{{ item.label }}</dt><dd class="mt-1 whitespace-pre-wrap break-all">{{ item.value || '—' }}</dd></div>
        </dl>
        <article v-for="(prompt, index) in detail.prompts" :key="`${detail.record_id}-prompt-${index}`" class="space-y-2"><h4 class="text-sm font-medium">{{ t('admin.desktop.conversations.prompt', { index: index + 1 }) }}</h4><DesktopConversationText :segment="prompt" /></article>
        <article class="space-y-2"><h4 class="text-sm font-medium">{{ t('admin.desktop.conversations.response') }}</h4><DesktopConversationText v-if="detail.response" :key="`${detail.record_id}-response`" :segment="detail.response" /><p v-else class="rounded-lg bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ t('admin.desktop.conversations.responseMissing') }}</p></article>
      </template>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDesktopConversation, listDesktopConversations } from '@/api/desktopConversations'
import type { DesktopConversation, DesktopConversationDetail, DesktopConversationQuery } from '@/api/desktopConversations'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import type { Column } from '@/components/common/types'
import { formatDateTime } from '@/utils/format'
import DesktopConversationText from './DesktopConversationText.vue'

const props = defineProps<{ organizationId: string; selfManaged: boolean }>()
const { t } = useI18n()
const emptyFilters = () => ({ member_search: '', client: '', capture_status: '', record_id: '', source_session_id: '', installation_id: '', received_from: '', received_to: '' })
const filters = reactive(emptyFilters())
const records = ref<DesktopConversation[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const error = ref('')
const thread = ref<DesktopConversation | null>(null)
const selected = ref<DesktopConversation | null>(null)
const detail = ref<DesktopConversationDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
let listController: AbortController | undefined
let detailController: AbortController | undefined
const hasFilters = computed(() => Boolean(thread.value || Object.values(filters).some(Boolean)))
const columns = computed<Column[]>(() => [
  { key: 'member_name', label: t('admin.desktop.member') },
  { key: 'client', label: t('admin.desktop.conversations.client') },
  { key: 'capture_status', label: t('admin.desktop.conversations.status') },
  { key: 'prompt_count', label: t('admin.desktop.conversations.promptCount') },
  { key: 'received_at', label: t('admin.desktop.conversations.receivedAt') },
  { key: 'source_session_id', label: t('admin.desktop.conversations.sessionID') },
  { key: 'actions', label: t('common.actions') },
])
const detailMetadata = computed(() => {
  if (!detail.value) return []
  const value = detail.value
  return [
    { label: t('admin.desktop.member'), value: `${value.member_name} (${value.member_id})` },
    { label: t('admin.desktop.conversations.client'), value: clientLabel(value.client) },
    { label: t('admin.desktop.conversations.receivedAt'), value: formatDateTime(value.received_at) },
    { label: t('admin.desktop.conversations.startedAt'), value: formatDateTime(value.started_at) },
    { label: t('admin.desktop.conversations.stoppedAt'), value: formatDateTime(value.stopped_at) },
    { label: t('admin.desktop.conversations.installationID'), value: value.installation_id },
    { label: t('admin.desktop.conversations.sessionID'), value: value.source_session_id },
    { label: t('admin.desktop.conversations.turnID'), value: value.source_turn_id },
    { label: t('admin.desktop.conversations.cwd'), value: value.cwd },
  ]
})

function clientLabel(client: string) { return client === 'workbuddy' ? 'Workbuddy' : 'ChatGPT Codex' }
function statusLabel(status: string) { return t(status === 'captured' ? 'admin.desktop.conversations.captured' : 'admin.desktop.conversations.responseMissing') }
function requestError(cause: unknown) {
  const status = (cause as { response?: { status?: number } })?.response?.status
  if (status === 404) return t('admin.desktop.conversations.notFound')
  if (status === 422) return t('admin.desktop.conversations.invalidFilters')
  return t('admin.desktop.conversations.loadFailed')
}
function closeDetail() {
  detailController?.abort()
  selected.value = null
  detail.value = null
  detailError.value = ''
  detailLoading.value = false
}
async function loadRecords() {
  listController?.abort()
  closeDetail()
  const controller = new AbortController()
  listController = controller
  records.value = []
  total.value = 0
  error.value = ''
  if (!props.organizationId) { loading.value = false; return }
  loading.value = true
  try {
    const query: DesktopConversationQuery = { page: page.value, page_size: pageSize.value, sort_order: thread.value ? 'asc' : 'desc' }
    if (thread.value) {
      Object.assign(query, { member_id: thread.value.member_id, client: thread.value.client, installation_id: thread.value.installation_id, source_session_id: thread.value.source_session_id })
    } else {
      for (const [key, value] of Object.entries(filters)) {
        if (value) Object.assign(query, { [key]: key.startsWith('received_') ? new Date(value).toISOString() : value.trim() })
      }
    }
    const result = await listDesktopConversations(props.organizationId, props.selfManaged, query, controller.signal)
    if (controller.signal.aborted) return
    records.value = result.items
    total.value = result.total
  } catch (cause) {
    if (!controller.signal.aborted) error.value = requestError(cause)
  } finally { if (!controller.signal.aborted) loading.value = false }
}
async function openRecord(record: DesktopConversation) {
  detailController?.abort()
  const controller = new AbortController()
  detailController = controller
  selected.value = record
  detail.value = null
  detailError.value = ''
  detailLoading.value = true
  try {
    const result = await getDesktopConversation(props.organizationId, props.selfManaged, record.record_id, controller.signal)
    if (!controller.signal.aborted) detail.value = result
  } catch (cause) {
    if (!controller.signal.aborted) detailError.value = requestError(cause)
  } finally { if (!controller.signal.aborted) detailLoading.value = false }
}
function applyFilters() { page.value = 1; void loadRecords() }
function resetFilters() { Object.assign(filters, emptyFilters()); thread.value = null; applyFilters() }
function openThread(record: DesktopConversation) { thread.value = record; applyFilters() }
function changePage(value: number) { page.value = value; void loadRecords() }
function changePageSize(value: number) { pageSize.value = value; applyFilters() }
watch(() => [props.organizationId, props.selfManaged], () => { resetFilters() }, { immediate: true })
onBeforeUnmount(() => { listController?.abort(); detailController?.abort() })
</script>
