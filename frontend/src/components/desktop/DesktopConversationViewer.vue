<template>
  <BaseDialog :show="Boolean(record)" :title="t(isThread ? 'admin.desktop.conversations.threadTitle' : 'admin.desktop.conversations.detail')" width="extra-wide" @close="close">
    <div v-if="record" class="flex h-[min(68dvh,52rem)] min-h-0 flex-col gap-4" data-testid="conversation-viewer">
      <div class="flex shrink-0 flex-wrap items-center justify-between gap-3">
        <div class="min-w-0 text-sm">
          <p class="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span class="break-words font-semibold text-gray-900 dark:text-gray-100">{{ (selected || record).member_name }}</span>
            <span v-if="(selected || record).member_deleted" class="text-xs text-gray-500">{{ t('admin.desktop.conversations.deletedMember') }}</span>
            <span class="text-gray-500 dark:text-dark-400">{{ clientLabel((selected || record).client) }}</span>
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t(isThread ? 'admin.desktop.conversations.threadHint' : 'admin.desktop.conversations.recordHint') }}</p>
        </div>
        <button v-if="!isThread && selected" class="btn btn-secondary btn-sm gap-1.5" type="button" @click="browseThread(selected)">
          <Icon name="chat" size="sm" aria-hidden="true" />{{ t('admin.desktop.conversations.viewThread') }}
        </button>
      </div>

      <div class="flex min-h-0 flex-1 flex-col gap-4 md:flex-row">
        <aside v-if="isThread" class="flex min-h-0 shrink-0 flex-col gap-3 md:w-60 md:border-r md:border-gray-200 md:pr-4 md:dark:border-dark-700" :aria-label="t('admin.desktop.conversations.threadTitle')" :aria-busy="threadLoading">
          <div class="flex items-center justify-between text-sm">
            <h4 class="font-medium">{{ t('admin.desktop.conversations.threadRecords') }}</h4>
            <span class="tabular-nums text-gray-500">{{ threadTotal }}</span>
          </div>
          <p v-if="threadLoading" class="py-4 text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
          <div v-else-if="threadError" class="text-sm text-red-600 dark:text-red-400" role="alert">
            {{ threadError }}
            <button class="mt-2 block underline" type="button" @click="loadThread(threadPage)">{{ t('admin.desktop.conversations.retry') }}</button>
          </div>
          <p v-else-if="!threadRecords.length" class="text-sm text-gray-500" role="status">{{ t('admin.desktop.conversations.empty') }}</p>
          <template v-else>
            <select class="input md:hidden" :value="selected?.record_id" :aria-label="t('admin.desktop.conversations.selectRecord')" @change="selectThreadRecord">
              <option v-for="(item, index) in threadRecords" :key="item.record_id" :value="item.record_id">{{ recordNumber(index) }} · {{ formatDateTime(item.received_at) }}</option>
            </select>
            <nav class="hidden min-h-0 flex-1 space-y-1 overflow-y-auto overscroll-contain p-1 md:block" :aria-label="t('admin.desktop.conversations.selectRecord')">
              <button
                v-for="(item, index) in threadRecords"
                :key="item.record_id"
                type="button"
                class="w-full rounded-xl border p-3 text-left text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
                :class="selected?.record_id === item.record_id ? 'border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-800 dark:bg-primary-900/20 dark:text-primary-300' : 'border-transparent text-gray-600 hover:bg-gray-50 dark:text-dark-300 dark:hover:bg-dark-700'"
                :aria-current="selected?.record_id === item.record_id ? 'true' : undefined"
                @click="openRecord(item)"
              >
                <span class="block font-medium">{{ t('admin.desktop.conversations.recordNumber', { index: recordNumber(index) }) }}</span>
                <span class="mt-1 block text-xs tabular-nums">{{ formatDateTime(item.received_at) }}</span>
                <span v-if="item.capture_status === 'response_missing'" class="mt-1 block text-xs text-amber-700 dark:text-amber-400">{{ t('admin.desktop.conversations.responseMissing') }}</span>
              </button>
            </nav>
          </template>
          <div v-if="threadTotal > threadPageSize" class="flex shrink-0 items-center justify-between gap-2">
            <button class="btn btn-ghost btn-sm" type="button" :disabled="threadLoading || threadPage <= 1" :aria-label="t('pagination.previous')" @click="loadThread(threadPage - 1)"><Icon name="chevronLeft" size="sm" aria-hidden="true" /></button>
            <span class="text-xs tabular-nums text-gray-500">{{ t('pagination.pageOf', { page: threadPage, total: threadPages }) }}</span>
            <button class="btn btn-ghost btn-sm" type="button" :disabled="threadLoading || threadPage >= threadPages" :aria-label="t('pagination.next')" @click="loadThread(threadPage + 1)"><Icon name="chevronRight" size="sm" aria-hidden="true" /></button>
          </div>
        </aside>

        <section ref="readingPane" class="min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-contain rounded-xl border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/30 sm:p-5" :aria-busy="detailLoading" :aria-label="t('admin.desktop.conversations.detail')" tabindex="0" data-testid="conversation-detail">
          <p v-if="detailLoading" class="py-12 text-center text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
          <div v-else-if="detailError" class="py-8 text-center text-sm text-red-600 dark:text-red-400" role="alert">
            <p>{{ detailError }}</p>
            <button v-if="selected" class="btn btn-secondary mt-3" type="button" @click="openRecord(selected)">{{ t('admin.desktop.conversations.retry') }}</button>
          </div>
          <div v-else-if="detail" :key="detail.record_id" class="mx-auto max-w-3xl space-y-5">
            <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-dark-400">
              <span>{{ t('admin.desktop.conversations.receivedAt') }} · {{ formatDateTime(detail.received_at) }}</span>
              <span :class="detail.capture_status === 'response_missing' ? 'text-amber-700 dark:text-amber-400' : ''">{{ t(detail.capture_status === 'captured' ? 'admin.desktop.conversations.captured' : 'admin.desktop.conversations.responseMissing') }}</span>
            </div>
            <article v-for="(prompt, index) in detail.prompts" :key="index" class="space-y-2">
              <h4 class="flex items-center gap-2 text-sm font-semibold text-primary-700 dark:text-primary-300"><Icon name="user" size="sm" :stroke-width="2" aria-hidden="true" />{{ t('admin.desktop.conversations.prompt', { index: index + 1 }) }}</h4>
              <DesktopConversationText :segment="prompt" />
            </article>
            <article class="space-y-2">
              <h4 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-gray-100"><Icon name="chat" size="sm" :stroke-width="2" aria-hidden="true" />{{ t('admin.desktop.conversations.response') }}</h4>
              <DesktopConversationText v-if="detail.response" :segment="detail.response" />
              <p v-else class="rounded-lg bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">{{ t('admin.desktop.conversations.responseMissing') }}</p>
            </article>
            <details class="border-t border-gray-200 pt-4 text-sm dark:border-dark-700">
              <summary class="cursor-pointer rounded text-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-dark-400">{{ t('admin.desktop.conversations.metadata') }}</summary>
              <dl class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div v-for="item in detailMetadata" :key="item.label" class="min-w-0"><dt class="text-xs text-gray-500 dark:text-dark-400">{{ item.label }}</dt><dd class="mt-1 whitespace-pre-wrap break-all text-gray-700 dark:text-dark-200">{{ item.value || '—' }}</dd></div>
              </dl>
            </details>
          </div>
          <p v-else class="py-12 text-center text-sm text-gray-500">{{ t('admin.desktop.conversations.selectRecord') }}</p>
        </section>
      </div>
    </div>
    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <button class="btn btn-secondary btn-sm gap-1" type="button" :disabled="!canPrevious" @click="navigate(-1)"><Icon name="chevronLeft" size="sm" aria-hidden="true" />{{ t('admin.desktop.conversations.previousRecord') }}</button>
          <span class="text-xs tabular-nums text-gray-500" aria-live="polite">{{ position }} / {{ isThread ? threadTotal : records.length }}</span>
          <button class="btn btn-secondary btn-sm gap-1" type="button" :disabled="!canNext" @click="navigate(1)">{{ t('admin.desktop.conversations.nextRecord') }}<Icon name="chevronRight" size="sm" aria-hidden="true" /></button>
        </div>
        <button class="btn btn-secondary btn-sm" type="button" @click="close">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDesktopConversation, listDesktopConversations } from '@/api/desktopConversations'
import type { DesktopConversation, DesktopConversationDetail } from '@/api/desktopConversations'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import DesktopConversationText from './DesktopConversationText.vue'

const props = defineProps<{
  record: DesktopConversation | null
  records: DesktopConversation[]
  mode: 'record' | 'thread'
  organizationId: string
  selfManaged: boolean
}>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t } = useI18n()
const isThread = ref(false)
const threadAnchor = ref<DesktopConversation | null>(null)
const threadRecords = ref<DesktopConversation[]>([])
const threadTotal = ref(0)
const threadPage = ref(1)
const threadPageSize = 20
const threadPages = computed(() => Math.ceil(threadTotal.value / threadPageSize))
const threadLoading = ref(false)
const threadError = ref('')
const selected = ref<DesktopConversation | null>(null)
const detail = ref<DesktopConversationDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const readingPane = ref<HTMLElement | null>(null)
let threadController: AbortController | undefined
let detailController: AbortController | undefined
const visibleRecords = computed(() => isThread.value ? threadRecords.value : props.records)
const selectedIndex = computed(() => visibleRecords.value.findIndex(item => item.record_id === selected.value?.record_id))
const position = computed(() => selectedIndex.value < 0 ? 0 : recordNumber(selectedIndex.value))
const canPrevious = computed(() => !threadLoading.value && selectedIndex.value >= 0 && (selectedIndex.value > 0 || (isThread.value && threadPage.value > 1)))
const canNext = computed(() => !threadLoading.value && selectedIndex.value >= 0 && (selectedIndex.value < visibleRecords.value.length - 1 || (isThread.value && threadPage.value < threadPages.value)))
const detailMetadata = computed(() => {
  if (!detail.value) return []
  const value = detail.value
  return [
    { label: t('admin.desktop.conversations.recordID'), value: value.record_id },
    { label: t('admin.desktop.member'), value: `${value.member_name} (${value.member_id})` },
    { label: t('admin.desktop.conversations.startedAt'), value: formatDateTime(value.started_at) },
    { label: t('admin.desktop.conversations.stoppedAt'), value: formatDateTime(value.stopped_at) },
    { label: t('admin.desktop.conversations.installationID'), value: value.installation_id },
    { label: t('admin.desktop.conversations.sessionID'), value: value.source_session_id },
    { label: t('admin.desktop.conversations.turnID'), value: value.source_turn_id },
    { label: t('admin.desktop.conversations.cwd'), value: value.cwd },
  ]
})

function clientLabel(client: string) { return client === 'workbuddy' ? 'Workbuddy' : 'ChatGPT Codex' }
function recordNumber(index: number) { return (isThread.value ? (threadPage.value - 1) * threadPageSize : 0) + index + 1 }
function requestError(cause: unknown) {
  return t((cause as { response?: { status?: number } })?.response?.status === 404 ? 'admin.desktop.conversations.notFound' : 'admin.desktop.conversations.loadFailed')
}
function clearDetail() {
  detailController?.abort()
  selected.value = null
  detail.value = null
  detailError.value = ''
  detailLoading.value = false
}
function reset() {
  threadController?.abort()
  clearDetail()
  threadAnchor.value = null
  threadRecords.value = []
  threadTotal.value = 0
  threadPage.value = 1
  threadError.value = ''
  threadLoading.value = false
}
function close() { reset(); emit('close') }
async function openRecord(record: DesktopConversation) {
  clearDetail()
  const controller = new AbortController()
  detailController = controller
  selected.value = record
  detailLoading.value = true
  if (readingPane.value) readingPane.value.scrollTop = 0
  try {
    const result = await getDesktopConversation(props.organizationId, props.selfManaged, record.record_id, controller.signal)
    if (!controller.signal.aborted) detail.value = result
  } catch (cause) {
    if (!controller.signal.aborted) detailError.value = requestError(cause)
  } finally { if (!controller.signal.aborted) detailLoading.value = false }
}
async function loadThread(page: number, selectLast = false) {
  if (!threadAnchor.value) return
  threadController?.abort()
  clearDetail()
  const controller = new AbortController()
  threadController = controller
  threadPage.value = page
  threadRecords.value = []
  threadError.value = ''
  threadLoading.value = true
  const anchor = threadAnchor.value
  try {
    const result = await listDesktopConversations(props.organizationId, props.selfManaged, {
      page, page_size: threadPageSize, sort_order: 'asc', member_id: anchor.member_id,
      client: anchor.client, installation_id: anchor.installation_id, source_session_id: anchor.source_session_id,
    }, controller.signal)
    if (controller.signal.aborted) return
    threadRecords.value = result.items
    threadTotal.value = result.total
    const next = selectLast ? result.items[result.items.length - 1] : result.items[0]
    if (next) void openRecord(next)
  } catch (cause) {
    if (!controller.signal.aborted) threadError.value = requestError(cause)
  } finally { if (!controller.signal.aborted) threadLoading.value = false }
}
function browseThread(record: DesktopConversation) {
  isThread.value = true
  threadAnchor.value = record
  void loadThread(1)
}
function selectThreadRecord(event: Event) {
  const item = threadRecords.value.find(item => item.record_id === (event.target as HTMLSelectElement).value)
  if (item) void openRecord(item)
}
function navigate(direction: -1 | 1) {
  if (direction === -1 ? !canPrevious.value : !canNext.value) return
  const next = visibleRecords.value[selectedIndex.value + direction]
  if (next) void openRecord(next)
  else if (isThread.value) void loadThread(threadPage.value + direction, direction === -1)
}
watch(() => [props.record, props.mode, props.organizationId, props.selfManaged], () => {
  reset()
  isThread.value = props.mode === 'thread'
  if (!props.record) return
  if (isThread.value) browseThread(props.record)
  else void openRecord(props.record)
}, { immediate: true })
onBeforeUnmount(reset)
</script>
