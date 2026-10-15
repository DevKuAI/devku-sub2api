<template>
  <div v-if="!selfManaged || enabled" class="desktop-analytics desktop-reports min-w-0 space-y-6">
    <header class="card analytics-panel report-toolbar">
      <label class="min-w-0 space-y-2 text-sm font-medium">
        <span class="block">{{ t(`${key}.date`) }}</span>
        <input v-model="date" class="input analytics-input" name="report-date" type="date" :max="yesterday" required :aria-invalid="!validDate || undefined" :aria-describedby="!validDate ? 'daily-report-date-error' : undefined" />
      </label>
      <div class="min-w-0 space-y-2">
        <p class="analytics-caption">{{ t(`${key}.schedule`) }}</p>
        <p class="analytics-caption">{{ t(`${key}.configuredModel`) }}: <bdi class="font-medium text-gray-900 dark:text-gray-100">{{ analysisModel || t('admin.desktop.notConfigured') }}</bdi></p>
      </div>
      <div class="report-actions">
        <button class="btn btn-secondary analytics-button" type="button" :disabled="loading || !validDate" @click="load">{{ t(loading ? `${key}.loading` : 'common.refresh') }}</button>
        <button v-if="state?.task && taskActive" class="btn btn-secondary analytics-button" type="button" @click="toggleRefresh">{{ t(autoRefresh ? `${key}.pauseRefresh` : `${key}.resumeRefresh`) }}</button>
        <template v-if="!selfManaged">
          <button v-for="mode in modes" :key="mode" class="btn btn-secondary analytics-button" type="button" :disabled="runDisabled(mode)" @click="requestRun(mode)">{{ t(`${key}.${mode}`) }}</button>
        </template>
      </div>
      <div v-if="summary?.queue && !selfManaged" class="report-control-note" role="status"><p>{{ t(`${key}.queue`, { pending: summary.queue.pending, running: summary.queue.running }) }}</p><p v-if="summary.queue.oldest_pending_at">{{ t(`${key}.oldestPending`) }}: {{ formatDateTime(summary.queue.oldest_pending_at) }}</p></div>
    </header>
    <p v-if="!validDate" id="daily-report-date-error" class="report-error" role="alert">{{ t(`${key}.invalidDate`) }}</p>
    <div v-if="error" class="report-notice report-error flex flex-wrap items-center justify-between gap-3" role="alert">
      <p>{{ error }}</p>
      <button class="btn btn-secondary analytics-button" type="button" :disabled="loading || !validDate" @click="load">{{ t('common.refresh') }}</button>
    </div>
    <div v-if="state?.reason" class="report-notice" :class="state.reason === 'no_records' ? 'report-neutral' : 'report-warning'">
      <p class="font-medium">{{ reasonLabel(state.reason) }}</p>
      <p v-if="state.reason === 'no_records'" class="mt-1 text-sm leading-relaxed">{{ t(`${key}.noRecordsHint`) }}</p>
      <p v-else-if="!['model_unavailable', 'configuration_changed', 'retries_exhausted'].includes(state.reason)" class="mt-1 text-sm leading-relaxed">{{ t(selfManaged ? `${key}.contactAdmin` : `${key}.checkConfiguration`) }}</p>
    </div>
    <div v-if="summary?.missing_dates?.length && !selfManaged" class="report-notice report-warning"><p>{{ t(`${key}.missingDates`) }}</p><div class="mt-2 flex flex-wrap gap-2"><button v-for="missing in summary.missing_dates" :key="missing" class="btn btn-secondary analytics-button" type="button" @click="date = missing">{{ missing }}</button></div></div>
    <p class="sr-only" role="status">{{ loading ? t(`${key}.loading`) : state ? statusLabel(state.status) : '' }}</p>
    <nav class="tabs report-tabs report-scope-tabs" role="tablist" :aria-label="t(`${key}.title`)">
      <button v-for="(tab, index) in reportTabs" :id="`daily-report-tab-${tab}`" :key="tab" class="tab report-tab" :class="{ 'tab-active': activeTab === tab }" type="button" role="tab" :aria-selected="activeTab === tab" :aria-controls="`daily-report-panel-${tab}`" :tabindex="activeTab === tab ? 0 : -1" @click="activeTab = tab" @keydown="tabKeydown($event, index)">{{ t(`${key}.${tab}`) }}</button>
    </nav>
    <section v-if="activeTab === 'summary'" id="daily-report-panel-summary" role="tabpanel" aria-labelledby="daily-report-tab-summary" class="min-w-0 space-y-5" :aria-busy="loading" tabindex="0">
      <div v-if="summary?.summary.content" class="card analytics-panel space-y-5">
        <header class="space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3"><h3 class="analytics-heading">{{ t(`${key}.summary`) }}</h3><div class="flex flex-wrap items-center gap-3"><span class="report-badge" :data-tone="statusTone(summary.summary.status)">{{ statusLabel(summary.summary.status) }}</span><button class="btn btn-primary analytics-button" :disabled="copyingSummary" @click="copySummary">{{ t('common.copy') }}</button></div></div>
          <p v-if="taskActive" class="analytics-caption">{{ t(`${key}.previousResultHint`) }}</p>
        </header>
        <DesktopReportContent :content="summary.summary.content" />
        <p v-if="summaryCopyStatus" class="analytics-caption" role="status">{{ summaryCopyStatus }}</p>
        <dl class="report-metadata"><div><dt class="analytics-caption">{{ t(`${key}.usedModel`) }}</dt><dd><bdi>{{ summary.summary.model || '—' }}</bdi></dd></div><div><dt class="analytics-caption">{{ t(`${key}.generatedAt`) }}</dt><dd>{{ summary.summary.generated_at ? formatDateTime(summary.summary.generated_at) : '—' }}</dd></div></dl>
        <DesktopReportExecutionHistory :executions="summary.summary.executions || []" /><div class="flex gap-3"><button v-if="executionPage > 1" class="btn btn-secondary analytics-button" @click="changeExecutionPage(executionPage - 1)">{{ t(`${key}.previousPage`) }}</button><button v-if="(summary.summary.executions?.length || 0) === 50" class="btn btn-secondary analytics-button" @click="changeExecutionPage(executionPage + 1)">{{ t(`${key}.nextPage`) }}</button></div>
      </div>
      <div v-else class="report-empty">
        <h3 class="analytics-heading">{{ emptyTitle('summary') }}</h3>
        <p class="analytics-caption mt-2">{{ emptyHint('summary') }}</p>
        <DesktopReportExecutionHistory class="mt-5 text-start" :executions="summary?.summary.executions || []" />
      </div>
      <dl v-if="summary" class="report-metrics">
        <div class="card report-metric"><dt class="analytics-caption">{{ t(`${key}.taskStatus`) }}</dt><dd class="report-metric-value"><span class="report-badge" :data-tone="statusTone(summary.task?.status || summary.status)">{{ statusLabel(summary.task?.status || summary.status) }}</span></dd></div>
        <div class="card report-metric"><dt class="analytics-caption">{{ t(`${key}.coverageTitle`) }}</dt><dd class="report-metric-value">{{ summary.expected_members ? `${summary.completed_members} / ${summary.expected_members}` : '—' }}</dd><dd class="analytics-caption report-metric-hint">{{ t(`${key}.coverageHint`) }}</dd></div>
        <div class="card report-metric"><dt class="analytics-caption">{{ t(`${key}.noConversationMembers`) }}</dt><dd class="report-metric-value">{{ summary.no_record_members ?? (members ? noRecordMembers.length : '—') }}</dd><dd class="analytics-caption report-metric-hint">{{ t(`${key}.noRecordsCountHint`) }}</dd></div>
      </dl>
      <div v-if="summary?.missing_members.length" class="report-notice report-warning space-y-3">
        <h3 class="text-sm font-semibold">{{ t(`${key}.missing`) }}</h3>
        <div class="flex flex-wrap gap-2"><button v-for="id in summary.missing_members" :key="id" class="btn btn-secondary analytics-button" type="button" @click="viewMember(id)"><bdi>{{ memberName(id) }}</bdi></button></div>
      </div>
      <div v-if="members?.members.some(member => member.record_count > 0)" class="flex flex-wrap items-center justify-between gap-3">
        <p class="analytics-caption">{{ t(`${key}.memberReportsHint`) }}</p>
        <button class="btn btn-secondary analytics-button" type="button" @click="viewMembers">{{ t(`${key}.viewMembers`) }}</button>
      </div>
    </section>
    <section v-else id="daily-report-panel-members" role="tabpanel" aria-labelledby="daily-report-tab-members" class="card analytics-panel report-member-panel space-y-5" :aria-busy="loading" tabindex="0">
      <div class="report-member-toolbar">
        <label class="report-member-filter space-y-2 text-sm font-medium"><span>{{ t(`${key}.memberFilter`) }}</span><input ref="memberSearch" v-model="search" class="input analytics-input" name="report-member" type="search" :placeholder="t(`${key}.memberSearch`)" /></label>
        <p v-if="members" class="report-member-count analytics-caption tabular-nums" role="status">{{ t(`${key}.memberCount`, { shown: filteredMembers.length, total: members.total_members ?? members.members.length }) }}</p>
      </div>
      <Pagination v-if="(members?.total_members || 0) > 20" v-model:page="memberPage" :page-size="20" :total="members?.total_members || 0" :show-page-size-selector="false" />
      <template v-if="filteredMembers.length">
        <div class="report-member-table analytics-data" tabindex="0" :aria-label="t(`${key}.members`)">
          <table>
            <colgroup><col class="report-member-name-column" /><col class="report-member-status-column" /><col /><col class="report-member-date-column" /><col class="report-member-action-column" /></colgroup>
            <thead><tr><th scope="col">{{ t('admin.desktop.member') }}</th><th scope="col">{{ t('common.status') }}</th><th scope="col">{{ t(`${key}.usedModel`) }}</th><th scope="col">{{ t(`${key}.generatedAt`) }}</th><th scope="col">{{ t('common.actions') }}</th></tr></thead>
            <tbody>
              <tr v-for="member in filteredMembers" :key="member.member_id">
                <th scope="row"><bdi class="report-member-name">{{ member.name }}</bdi><span v-if="member.deleted" class="mt-1 block text-xs">{{ t('admin.desktop.conversations.deletedMember') }}</span></th>
                <td><span class="report-badge" :data-tone="statusTone(member.status)">{{ statusLabel(member.status) }}</span></td>
                <td class="report-member-model"><bdi>{{ member.model || '—' }}</bdi></td>
                <td><time v-if="member.generated_at" :datetime="member.generated_at">{{ formatDateTime(member.generated_at) }}</time><span v-else>—</span></td>
                <td><button v-if="member.record_count > 0" class="btn btn-secondary analytics-button" type="button" :aria-label="t(`${key}.viewMemberReport`, { name: member.name })" @click="viewMember(member.member_id)">{{ t(`${key}.viewReport`) }}</button><span v-else>—</span></td>
              </tr>
            </tbody>
          </table>
        </div>
        <ul class="report-member-cards space-y-3">
          <li v-for="member in filteredMembers" :key="member.member_id" class="report-member-card space-y-4">
            <div class="report-member-card-heading"><h3 class="analytics-heading"><bdi>{{ member.name }}</bdi><span v-if="member.deleted" class="analytics-caption ms-2">{{ t('admin.desktop.conversations.deletedMember') }}</span></h3><span class="report-badge" :data-tone="statusTone(member.status)">{{ statusLabel(member.status) }}</span></div>
            <dl class="report-metadata"><div><dt class="analytics-caption">{{ t(`${key}.usedModel`) }}</dt><dd><bdi>{{ member.model || '—' }}</bdi></dd></div><div><dt class="analytics-caption">{{ t(`${key}.generatedAt`) }}</dt><dd>{{ member.generated_at ? formatDateTime(member.generated_at) : '—' }}</dd></div></dl>
            <button v-if="member.record_count > 0" class="btn btn-secondary analytics-button w-full" type="button" :aria-label="t(`${key}.viewMemberReport`, { name: member.name })" @click="viewMember(member.member_id)">{{ t(`${key}.viewReport`) }}</button>
          </li>
        </ul>
      </template>
      <div v-else class="report-empty">
        <h3 class="analytics-heading">{{ search && members ? t('admin.desktop.noMatchingMembers') : emptyTitle('members') }}</h3>
        <p class="analytics-caption mt-2">{{ search && members ? t(`${key}.filterEmptyHint`) : emptyHint('members') }}</p>
        <button v-if="search && members" class="btn btn-secondary analytics-button mt-4" type="button" @click="clearSearch">{{ t(`${key}.clearFilter`) }}</button>
      </div>
      <p class="report-member-note analytics-caption">{{ t(`${key}.noRecordsHint`) }}</p>
    </section>
    <BaseDialog content-class="desktop-detail" width="wide" :close-label="t('common.close')" :show="Boolean(selectedMember)" :title="`${selected?.name || memberName(selectedMember)} · ${t(`${key}.memberDetail`)}`" @close="closeMemberReport">
      <div class="desktop-analytics min-w-0 space-y-5" :aria-busy="memberDetailLoading">
        <p v-if="memberDetailLoading" class="analytics-caption" role="status">{{ t(`${key}.loading`) }}</p>
        <div v-if="memberDetailError" class="report-notice report-error flex flex-wrap items-center justify-between gap-3" role="alert">
          <p>{{ memberDetailError }}</p>
          <button class="btn btn-secondary analytics-button" type="button" :disabled="memberDetailLoading" @click="loadMemberDetail">{{ t('common.refresh') }}</button>
        </div>
        <section v-if="selected" class="space-y-5">
          <header class="space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-3"><span class="analytics-caption">{{ t(`${key}.memberDetail`) }}</span><span class="report-badge" :data-tone="statusTone(selected.status)">{{ statusLabel(selected.status) }}</span></div>
            <p v-if="taskActive && selected.content" class="analytics-caption">{{ t(`${key}.previousResultHint`) }}</p>
          </header>
          <div v-if="selected.error" class="report-notice report-error"><p>{{ reasonLabel(selected.error) }}</p><p class="mt-1">{{ t(selfManaged ? `${key}.contactAdmin` : `${key}.retryHint`) }}</p></div>
          <DesktopReportContent v-if="selected.content" :content="selected.content" />
          <p v-else class="analytics-caption">{{ statusLabel(selected.status) }}</p>
          <dl class="report-metadata"><div><dt class="analytics-caption">{{ t(`${key}.usedModel`) }}</dt><dd><bdi>{{ selected.model || '—' }}</bdi></dd></div><div><dt class="analytics-caption">{{ t(`${key}.generatedAt`) }}</dt><dd>{{ selected.generated_at ? formatDateTime(selected.generated_at) : '—' }}</dd></div></dl>
          <DesktopReportExecutionHistory :executions="selected.executions || []" /><div class="flex gap-3"><button v-if="memberExecutionPage > 1" class="btn btn-secondary analytics-button" type="button" :disabled="memberDetailLoading" @click="changeMemberExecutionPage(memberExecutionPage - 1)">{{ t(`${key}.previousPage`) }}</button><button v-if="(selected.executions?.length || 0) === 50" class="btn btn-secondary analytics-button" type="button" :disabled="memberDetailLoading" @click="changeMemberExecutionPage(memberExecutionPage + 1)">{{ t(`${key}.nextPage`) }}</button></div>
          <div v-if="selected.source_ids.length" class="space-y-3">
            <h4 class="text-sm font-semibold">{{ t(`${key}.sourcesTitle`) }}</h4>
            <div class="flex flex-wrap gap-2"><button v-for="(id, index) in selected.source_ids" :key="id" class="btn btn-secondary analytics-button" type="button" :disabled="sourceLoading" @click="openSource(id, $event)">{{ t(`${key}.source`, { index: index + 1 }) }}</button></div>
            <p v-if="sourceError" class="report-error" role="alert">{{ sourceError }}</p>
            <p v-if="sourceLoading" class="analytics-caption" role="status">{{ t(`${key}.sourceLoading`) }}</p>
          </div>
        </section>
      </div>
    </BaseDialog>
    <DesktopConversationViewer :record="source" :records="source ? [source] : []" mode="record" :organization-id="organizationId" :self-managed="selfManaged" @close="source = null" />
    <BaseDialog content-class="desktop-detail" :close-label="t('common.close')" :show="confirmRegenerate" :title="t(`${key}.regenerate`)" :show-close-button="false" @close="confirmRegenerate = false">
      <p class="text-sm leading-relaxed">{{ t(`${key}.regenerateHint`) }}</p>
      <template #footer><div class="desktop-analytics flex flex-wrap justify-end gap-3"><button class="btn btn-secondary analytics-button" type="button" @click="confirmRegenerate = false">{{ t('common.cancel') }}</button><button class="btn btn-secondary analytics-button" type="button" :disabled="running" @click="execute('regenerate')">{{ t(`${key}.regenerate`) }}</button></div></template>
    </BaseDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DesktopReportContent from './DesktopReportContent.vue'
import { getDesktopMemberReports, getDesktopMemberReport, getDesktopSummaryReport, runDesktopReports } from '@/api/desktopReports'
import Pagination from '@/components/common/Pagination.vue'
import type { DesktopMemberReports, DesktopSummaryReport, DesktopReportMode } from '@/api/desktopReports'
import { getDesktopConversation } from '@/api/desktopConversations'
import type { DesktopConversation } from '@/api/desktopConversations'
import { formatDateTime } from '@/utils/format'
import DesktopConversationViewer from './DesktopConversationViewer.vue'
import DesktopReportExecutionHistory from './DesktopReportExecutionHistory.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import '@/styles/desktopAnalytics.css'
import '@/styles/desktopReports.css'

const props = defineProps<{ organizationId: string; selfManaged: boolean; enabled: boolean; analysisModel: string; initialDate?: string }>()
const { t, te } = useI18n()
const key = 'admin.desktop.reports'
const yesterday = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(Date.now() - 86400000))
const date = ref(/^\d{4}-\d{2}-\d{2}$/.test(props.initialDate || '') ? props.initialDate! : yesterday)
const validDate = computed(() => Boolean(date.value) && date.value <= yesterday)
const reportTabs = ['summary', 'members'] as const
const activeTab = ref<'summary' | 'members'>('summary')
const modes: DesktopReportMode[] = ['generate', 'retry', 'regenerate']
const members = ref<DesktopMemberReports>()
const summary = ref<DesktopSummaryReport>()
const state = computed(() => summary.value || members.value)
const taskActive = computed(() => ['pending', 'waiting', 'running'].includes(state.value?.task?.status || state.value?.status || ''))
const memberPage = ref(1)
const executionPage = ref(1)
const memberExecutionPage = ref(1)
const memberDetail = ref<import('@/api/desktopReports').DesktopMemberReport>()
const memberDetailLoading = ref(false)
const memberDetailError = ref('')
let memberDetailController: AbortController | undefined
let executionController: AbortController | undefined
const selectedMember = ref('')
const search = ref('')
const filteredMembers = computed(() => (members.value?.members || []).filter(member => member.name.toLowerCase().includes(search.value.trim().toLowerCase())))
const noRecordMembers = computed(() => members.value?.members.filter(member => member.record_count === 0) || [])
const selected = computed(() => memberDetail.value?.member_id === selectedMember.value ? memberDetail.value : members.value?.members.find(member => member.member_id === selectedMember.value))
const memberSearch = ref<HTMLInputElement>()
const loading = ref(false)
const running = ref(false)
const autoRefresh = ref(true)
const error = ref('')
const failedPanels = ref<string[]>([])
const copyingSummary = ref(false), summaryCopyStatus = ref('')
const confirmRegenerate = ref(false)
const source = ref<DesktopConversation | null>(null)
const sourceLoading = ref(false)
const sourceError = ref('')
let controller: AbortController | undefined
let summaryCopyController: AbortController | undefined
let sourceController: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false
function statusLabel(status: string) { return te(`${key}.statuses.${status}`) ? t(`${key}.statuses.${status}`) : status }
function reasonLabel(reason: string) { return te(`${key}.reasons.${reason}`) ? t(`${key}.reasons.${reason}`) : t(`${key}.unknownReason`) }
function statusTone(status: string) {
  if (status === 'completed') return 'success'
  if (status === 'failed') return 'danger'
  if (['partial', 'waiting', 'not_in_snapshot'].includes(status)) return 'warning'
  if (['pending', 'running'].includes(status)) return 'info'
  return 'neutral'
}
function memberName(id: string) { return members.value?.members.find(member => member.member_id === id)?.name || id }
function runDisabled(mode: DesktopReportMode) {
  return loading.value || running.value || !validDate.value || !state.value || failedPanels.value.length > 0 || !props.enabled || Boolean(state.value.reason && state.value.reason !== 'configuration_changed') || state.value.status === 'running' || (state.value.reason === 'configuration_changed' && mode !== 'regenerate')
}
function emptyTitle(panel: string) {
  if (!validDate.value) return t(`${key}.invalidDate`)
  if (loading.value && !(panel === 'summary' ? summary.value : members.value)) return t(`${key}.loading`)
  if (failedPanels.value.includes(panel)) return t(`${key}.loadFailed`)
  if (panel === 'members' && members.value && !members.value.members.length) return t(`${key}.noMembers`)
  if (state.value?.reason === 'no_records') return t(`${key}.noSummary`)
  if (state.value?.reason) return t(`${key}.notExecuted`)
  if (taskActive.value || state.value?.status === 'failed') return statusLabel(state.value?.task?.status || state.value?.status || 'pending')
  return t(`${key}.notGenerated`)
}
function emptyHint(panel: string) {
  if (!validDate.value) return t(`${key}.date`)
  if (loading.value) return t(`${key}.loadingHint`)
  if (failedPanels.value.includes(panel)) return t(`${key}.refreshHint`)
  if (state.value?.reason === 'no_records') return t(`${key}.noRecordsHint`)
  if (taskActive.value) return t(`${key}.processingHint`)
  if (state.value?.reason) return t(props.selfManaged ? `${key}.contactAdmin` : `${key}.checkConfiguration`)
  if (state.value?.status === 'failed') return t(props.selfManaged ? `${key}.contactAdmin` : `${key}.retryHint`)
  return t(props.selfManaged ? `${key}.contactAdmin` : `${key}.generateHint`)
}
function viewMember(id: string) {
  closeMemberReport()
  selectedMember.value = id
  memberExecutionPage.value = 1
  void loadMemberDetail()
}
function closeMemberReport() {
  memberDetailController?.abort()
  sourceController?.abort()
  selectedMember.value = ''
  memberDetail.value = undefined
  memberDetailLoading.value = false
  memberDetailError.value = ''
  source.value = null
  sourceLoading.value = false
  sourceError.value = ''
}
async function loadMemberDetail() {
  if (!selectedMember.value) return
  memberDetailController?.abort()
  const current = new AbortController(); memberDetailController = current
  const id = selectedMember.value
  memberDetailLoading.value = true
  memberDetailError.value = ''
  try {
    const detail = await getDesktopMemberReport(props.organizationId, props.selfManaged, date.value, id, current.signal, memberExecutionPage.value)
    if (!current.signal.aborted && !disposed && selectedMember.value === id) memberDetail.value = detail
  } catch { if (!current.signal.aborted && !disposed) memberDetailError.value = t(`${key}.loadFailed`) }
  finally { if (!current.signal.aborted) memberDetailLoading.value = false }
}
async function changeMemberExecutionPage(next: number) {
  memberExecutionPage.value = next
  await loadMemberDetail()
}
async function changeExecutionPage(next: number) {
  executionPage.value = next
  executionController?.abort()
  const current = new AbortController(); executionController = current
  try { const result = await getDesktopSummaryReport(props.organizationId, props.selfManaged, date.value, current.signal, next); if (!current.signal.aborted && !disposed) summary.value = result }
  catch { if (!current.signal.aborted && !disposed) error.value = t(`${key}.loadFailed`) }
}
async function viewMembers() { activeTab.value = 'members'; await nextTick(); memberSearch.value?.focus() }
function clearSearch() { search.value = ''; memberSearch.value?.focus() }
function toggleRefresh() { autoRefresh.value = !autoRefresh.value; clearTimeout(timer); if (autoRefresh.value) void load() }
function cancel() { summaryCopyController?.abort(); copyingSummary.value = false; controller?.abort(); sourceController?.abort(); memberDetailController?.abort(); executionController?.abort(); memberDetailLoading.value = false; sourceLoading.value = false; clearTimeout(timer) }
async function load() {
  cancel()
  if (disposed || (props.selfManaged && !props.enabled) || !validDate.value) { loading.value = false; return }
  const current = new AbortController(); controller = current
  loading.value = true; error.value = ''; failedPanels.value = []
  const results = await Promise.allSettled([
    getDesktopMemberReports(props.organizationId, props.selfManaged, date.value, current.signal, { page: memberPage.value, page_size: 20, search: search.value.trim() }),
    getDesktopSummaryReport(props.organizationId, props.selfManaged, date.value, current.signal),
  ])
  if (current.signal.aborted || disposed) return
  if (results[0].status === 'fulfilled') members.value = results[0].value
  else failedPanels.value.push('members')
  if (results[1].status === 'fulfilled') summary.value = results[1].value
  else failedPanels.value.push('summary')
  if (failedPanels.value.length) error.value = t(`${key}.loadFailed`)
  loading.value = false
  if (selectedMember.value) void loadMemberDetail()
  if (autoRefresh.value && state.value?.task && taskActive.value) timer = setTimeout(load, 15000)
}
function requestRun(mode: DesktopReportMode) { if (runDisabled(mode)) return; if (mode === 'regenerate') confirmRegenerate.value = true; else void execute(mode) }
async function execute(mode: DesktopReportMode) {
  if (runDisabled(mode)) return
  confirmRegenerate.value = false; running.value = true; error.value = ''
  try { await runDesktopReports(props.organizationId, date.value, mode); if (!disposed) await load() }
  catch { if (!disposed) error.value = t(`${key}.runFailed`) }
  finally { running.value = false }
}
async function openSource(id: string, event: MouseEvent) {
  const trigger = event.currentTarget as HTMLButtonElement
  sourceController?.abort(); const current = new AbortController(); sourceController = current; sourceLoading.value = true; sourceError.value = ''
  try {
    const record = await getDesktopConversation(props.organizationId, props.selfManaged, id, current.signal)
    if (current.signal.aborted || disposed) return
    sourceLoading.value = false
    await nextTick()
    if (current.signal.aborted || disposed) return
    trigger.focus()
    source.value = record
  }
  catch { if (!current.signal.aborted && !disposed) sourceError.value = t(`${key}.sourceFailed`) }
  finally { if (!current.signal.aborted) sourceLoading.value = false }
}
async function copySummary() {
  if (copyingSummary.value || !summary.value?.summary.content) return
  const currentDate = date.value, org = props.organizationId, managed = props.selfManaged, shown = summary.value.summary.content
  summaryCopyController?.abort(); const current = new AbortController(); summaryCopyController = current
  copyingSummary.value = true; summaryCopyStatus.value = ''
  try {
    const value = await getDesktopSummaryReport(org, managed, currentDate, current.signal)
    if (current.signal.aborted || disposed || date.value !== currentDate || props.organizationId !== org || props.selfManaged !== managed || (managed && !props.enabled)) return
    if (!value.summary.content || value.summary.content !== shown) { summaryCopyStatus.value = t('biNative.reportTextChanged'); void load(); return }
    await navigator.clipboard.writeText(shown)
    if (!disposed && date.value === currentDate && props.organizationId === org) summaryCopyStatus.value = t('biNative.copied')
  } catch { if (!current.signal.aborted && !disposed && date.value === currentDate && props.organizationId === org && props.selfManaged === managed) summaryCopyStatus.value = t('biNative.copyFailed') }
  finally { if (summaryCopyController === current) copyingSummary.value = false }
}
function tabKeydown(event: KeyboardEvent, index: number) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault(); activeTab.value = reportTabs[event.key === 'Home' ? 0 : event.key === 'End' ? 1 : 1 - index]
  void nextTick(() => document.getElementById(`daily-report-tab-${activeTab.value}`)?.focus())
}
watch([date, () => props.organizationId], () => { summaryCopyController?.abort(); copyingSummary.value = false; summaryCopyStatus.value = ''; members.value = undefined; summary.value = undefined; memberPage.value = 1; executionPage.value = 1; closeMemberReport(); confirmRegenerate.value = false; void load() }, { immediate: true })
watch(search, () => { if (memberPage.value !== 1) memberPage.value = 1; else void load() })
watch(memberPage, () => { void load() })
watch(activeTab, () => { executionPage.value = 1; executionController?.abort() })
watch(() => props.enabled, () => { if (props.selfManaged && !props.enabled) { cancel(); loading.value = false; confirmRegenerate.value = false; closeMemberReport() } else void load() })
onBeforeUnmount(() => { disposed = true; cancel() })
</script>
<style scoped>
.report-metrics, .report-metadata { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px; }
.report-metrics { gap: 16px; }
.report-metric { @apply min-w-0 p-5; }
.report-metric-value { @apply mt-3 flex min-h-8 items-center text-2xl font-semibold leading-tight tabular-nums text-gray-900 dark:text-gray-100; overflow-wrap: anywhere; }
.report-metric-hint { margin-top: 8px; }
.report-metadata dd { @apply mt-1 text-sm leading-relaxed text-gray-900 dark:text-gray-100; overflow-wrap: anywhere; }
.report-notice { @apply rounded-xl p-4 text-sm leading-relaxed; overflow-wrap: anywhere; }
.report-warning { @apply bg-amber-50 text-amber-900 dark:bg-dark-800 dark:text-amber-200; }
.report-error { @apply text-red-700 dark:text-red-300; }
.report-notice.report-error { @apply bg-red-50 dark:bg-dark-800; }
.report-neutral { @apply bg-gray-50 text-gray-700 dark:bg-dark-800 dark:text-dark-200; }
.report-badge { @apply inline-block max-w-full rounded-md bg-gray-100 px-2.5 py-1 text-sm font-medium leading-normal text-gray-700 dark:bg-dark-700 dark:text-dark-200; overflow-wrap: anywhere; }
.report-badge[data-tone='success'] { @apply bg-emerald-50 text-emerald-800 dark:bg-dark-800 dark:text-emerald-200; }
.report-badge[data-tone='warning'] { @apply bg-amber-50 text-amber-900 dark:bg-dark-800 dark:text-amber-200; }
.report-badge[data-tone='danger'] { @apply bg-red-50 text-red-700 dark:bg-dark-800 dark:text-red-300; }
.report-badge[data-tone='info'] { @apply bg-primary-50 text-primary-700 dark:bg-dark-800 dark:text-primary-300; }
.report-empty { @apply rounded-xl bg-gray-50 px-4 py-10 text-center dark:bg-dark-800; }
.report-member-toolbar { display: grid; grid-template-columns: minmax(0, 1fr); align-items: end; gap: 12px; }
.report-member-filter { display: block; min-width: 0; }
.report-member-filter > span { display: block; }
.report-member-count { @apply rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800; justify-self: start; }
.report-member-table { @apply rounded-xl border border-gray-200 dark:border-dark-700; display: none; }
.report-member-table table { table-layout: fixed; }
.report-member-name-column { width: 22%; }
.report-member-status-column { width: 7rem; }
.report-member-action-column { width: 9rem; }
.report-member-date-column { width: 12rem; }
.report-member-table th, .report-member-table td { text-align: start; white-space: normal; overflow-wrap: anywhere; vertical-align: middle; }
.report-member-table thead { @apply bg-gray-50 dark:bg-dark-800; }
.report-member-table th:last-child, .report-member-table td:last-child { text-align: end; }
.report-member-table tbody tr:last-child > * { border-bottom-width: 0; }
.report-member-name { @apply font-medium text-gray-900 dark:text-gray-100; }
.report-member-model { @apply text-gray-600 dark:text-dark-300; }
.report-member-card { @apply min-w-0 rounded-xl border border-gray-200 p-4 dark:border-dark-700; overflow-wrap: anywhere; }
.report-member-card-heading { display: flex; flex-wrap: wrap; align-items: start; justify-content: space-between; gap: 12px; }
.report-member-card-heading h3 { flex: 1; min-width: 0; }
.report-member-note { @apply border-t border-gray-100 pt-4 dark:border-dark-700; }
[role='tabpanel']:focus-visible { outline: 2px solid theme('colors.primary.600'); outline-offset: 3px; }

:global(html.dark) [role='tabpanel']:focus-visible { outline-color: theme('colors.primary.400'); }
@container (min-width: 36rem) {
  .report-metrics { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .report-metadata { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .report-member-toolbar { grid-template-columns: minmax(0, 24rem) minmax(0, 1fr); }
  .report-member-count { justify-self: end; }
}
@container (min-width: 48rem) {
  .report-member-cards { display: none; }
  .report-member-table { display: block; }
}
@media (prefers-reduced-motion: reduce) {
  :global(html:has(.desktop-reports)) { scroll-behavior: auto; }
}
@media (forced-colors: active) {
  .report-tab[aria-selected='true'] { border-width: 2px; }
  [role='tabpanel']:focus-visible { outline-color: Highlight; }
}
</style>
