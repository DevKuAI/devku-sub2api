<template>
  <div class="min-w-0 space-y-5">
    <div class="flex flex-wrap items-end justify-between gap-4">
      <label class="space-y-1 text-sm"><span class="block">{{ t(`${key}.date`) }}</span><input v-model="date" class="input" type="date" :max="yesterday" required /></label>
      <p class="text-sm text-gray-500">{{ t(`${key}.schedule`) }} · {{ t(`${key}.model`) }}: {{ analysisModel || t('admin.desktop.notConfigured') }}</p>
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-secondary" type="button" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
        <template v-if="!selfManaged">
          <button v-for="mode in modes" :key="mode" class="btn btn-secondary" type="button" :disabled="running || !enabled || Boolean(state?.reason && state.reason !== 'configuration_changed') || state?.status === 'running' || (state?.reason === 'configuration_changed' && mode !== 'regenerate')" @click="requestRun(mode)">{{ t(`${key}.${mode}`) }}</button>
        </template>
      </div>
    </div>
    <p v-if="error" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-if="state?.reason" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300" role="status">{{ reasonLabel(state.reason) }}</p>
    <div class="flex gap-2 border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t(`${key}.title`)">
      <button v-for="(tab, index) in reportTabs" :id="`daily-report-tab-${tab}`" :key="tab" class="border-b-2 px-4 py-3 text-sm" :class="activeTab === tab ? 'border-primary-500 text-primary-600' : 'border-transparent'" type="button" role="tab" :aria-selected="activeTab === tab" :aria-controls="`daily-report-panel-${tab}`" :tabindex="activeTab === tab ? 0 : -1" @click="activeTab = tab" @keydown="tabKeydown($event, index)">{{ t(`${key}.${tab}`) }}</button>
    </div>
    <section v-if="activeTab === 'summary'" id="daily-report-panel-summary" role="tabpanel" aria-labelledby="daily-report-tab-summary" class="space-y-4" :aria-busy="loading">
      <p v-if="summary" class="text-sm text-gray-500">{{ statusLabel(summary.summary.status || summary.status) }} · {{ t(`${key}.coverage`, { completed: summary.completed_members, expected: summary.expected_members }) }}</p>
      <p v-if="summary?.missing_members.length" class="text-sm text-amber-700 dark:text-amber-300">{{ t(`${key}.missing`) }}: {{ missingNames }}</p>
      <p v-if="summary?.summary.model" class="text-xs text-gray-500">{{ summary.summary.model }} · {{ summary.summary.generated_at ? formatDateTime(summary.summary.generated_at) : '—' }}</p>
      <DesktopReportExecutionHistory :executions="summary?.summary.executions || []" />
      <article v-if="summary?.summary.content" class="report-content max-w-none break-words" v-html="markdown(summary.summary.content)" />
      <p v-else class="py-8 text-center text-sm text-gray-500" role="status">{{ t(state?.reason === 'no_records' ? `${key}.noSummary` : `${key}.notGenerated`) }}</p>
      <div v-if="summary?.summary.content" class="flex flex-wrap gap-2"><button v-for="member in members?.members.filter(item => item.status === 'completed')" :key="member.member_id" class="btn btn-ghost btn-sm" type="button" @click="activeTab = 'members'; selectedMember = member.member_id">{{ member.name }}</button></div>
      <button v-if="members?.members.some(member => member.record_count > 0)" class="btn btn-secondary" type="button" @click="activeTab = 'members'">{{ t(`${key}.viewMembers`) }}</button>
    </section>
    <section v-else id="daily-report-panel-members" role="tabpanel" aria-labelledby="daily-report-tab-members" class="space-y-4" :aria-busy="loading">
      <label class="block max-w-sm space-y-1 text-sm"><span>{{ t('admin.desktop.member') }}</span><input v-model="search" class="input" type="search" :placeholder="t(`${key}.memberSearch`)" /></label>
      <p class="text-xs text-gray-500">{{ t(`${key}.noRecordsHint`) }}</p>
      <div class="overflow-x-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500" tabindex="0" :aria-label="t(`${key}.members`)">
        <table class="w-full min-w-[48rem] text-left text-sm">
          <thead><tr class="border-b border-gray-200 dark:border-dark-700"><th class="p-3">{{ t('admin.desktop.member') }}</th><th class="p-3">{{ t('common.status') }}</th><th class="p-3">{{ t(`${key}.model`) }}</th><th class="p-3">{{ t(`${key}.generatedAt`) }}</th><th class="p-3">{{ t('common.actions') }}</th></tr></thead>
          <tbody><tr v-for="member in filteredMembers" :key="member.member_id" class="border-b border-gray-100 dark:border-dark-700"><td class="p-3">{{ member.name }}<span v-if="member.deleted" class="ml-2 text-xs text-gray-500">{{ t('admin.desktop.conversations.deletedMember') }}</span></td><td class="p-3">{{ statusLabel(member.status) }}</td><td class="p-3">{{ member.model || '—' }}</td><td class="whitespace-nowrap p-3">{{ member.generated_at ? formatDateTime(member.generated_at) : '—' }}</td><td class="p-3"><button v-if="member.record_count > 0" class="btn btn-ghost btn-sm" type="button" @click="selectedMember = member.member_id">{{ t(`${key}.viewReport`) }}</button><span v-else>—</span></td></tr></tbody>
        </table>
      </div>
      <p v-if="!filteredMembers.length && !loading" class="text-sm text-gray-500" role="status">{{ t('admin.desktop.noMatchingMembers') }}</p>
      <div v-if="selected" class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <h3 class="font-semibold">{{ selected.name }}</h3>
        <DesktopReportExecutionHistory :executions="selected.executions || []" />
        <p v-if="selected.error" class="text-sm text-red-600">{{ reasonLabel(selected.error) }}</p>
        <article v-if="selected.content" class="report-content break-words" v-html="markdown(selected.content)" />
        <p v-else class="text-sm text-gray-500">{{ t(`${key}.notGenerated`) }}</p>
        <div class="flex flex-wrap gap-2"><button v-for="(id, index) in selected.source_ids" :key="id" class="btn btn-ghost btn-sm" type="button" :disabled="sourceLoading" @click="openSource(id)">{{ t(`${key}.source`, { index: index + 1 }) }}</button></div>
      </div>
    </section>
    <DesktopConversationViewer :record="source" :records="source ? [source] : []" mode="record" :organization-id="organizationId" :self-managed="selfManaged" @close="source = null" />
    <BaseDialog :show="confirmRegenerate" :title="t(`${key}.regenerate`)" @close="confirmRegenerate = false">
      <p class="text-sm">{{ t(`${key}.regenerateHint`) }}</p>
      <template #footer><div class="flex justify-end gap-2"><button class="btn btn-secondary" type="button" @click="confirmRegenerate = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="button" @click="execute('regenerate')">{{ t('common.confirm') }}</button></div></template>
    </BaseDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { getDesktopMemberReports, getDesktopSummaryReport, runDesktopReports } from '@/api/desktopReports'
import type { DesktopMemberReports, DesktopSummaryReport, DesktopReportMode } from '@/api/desktopReports'
import { getDesktopConversation } from '@/api/desktopConversations'
import type { DesktopConversation } from '@/api/desktopConversations'
import { formatDateTime } from '@/utils/format'
import DesktopConversationViewer from './DesktopConversationViewer.vue'
import DesktopReportExecutionHistory from './DesktopReportExecutionHistory.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ organizationId: string; selfManaged: boolean; enabled: boolean; analysisModel: string }>()
const { t, te } = useI18n()
const key = 'admin.desktop.reports'
const yesterday = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(Date.now() - 86400000))
const date = ref(yesterday)
const reportTabs = ['summary', 'members'] as const
const activeTab = ref<'summary' | 'members'>('summary')
const modes: DesktopReportMode[] = ['generate', 'retry', 'regenerate']
const members = ref<DesktopMemberReports>()
const summary = ref<DesktopSummaryReport>()
const state = computed(() => summary.value || members.value)
const selectedMember = ref('')
const search = ref('')
const filteredMembers = computed(() => (members.value?.members || []).filter(member => member.name.toLowerCase().includes(search.value.toLowerCase())))
const selected = computed(() => members.value?.members.find(member => member.member_id === selectedMember.value))
const missingNames = computed(() => summary.value?.missing_members.map(id => members.value?.members.find(member => member.member_id === id)?.name || id).join('、'))
const loading = ref(false)
const running = ref(false)
const error = ref('')
const confirmRegenerate = ref(false)
const source = ref<DesktopConversation | null>(null)
const sourceLoading = ref(false)
let controller: AbortController | undefined
let sourceController: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false
function markdown(text: string) { return DOMPurify.sanitize(marked.parse(text, { async: false }) as string) }
function statusLabel(status: string) { return te(`${key}.statuses.${status}`) ? t(`${key}.statuses.${status}`) : status }
function reasonLabel(reason: string) { return te(`${key}.reasons.${reason}`) ? t(`${key}.reasons.${reason}`) : t(`${key}.reasons.model_unavailable`) }
function cancel() { controller?.abort(); sourceController?.abort(); clearTimeout(timer) }
async function load() {
  cancel()
  if (disposed || (props.selfManaged && !props.enabled) || !date.value || date.value > yesterday) return
  const current = new AbortController(); controller = current
  loading.value = true; error.value = ''
  const results = await Promise.allSettled([
    getDesktopMemberReports(props.organizationId, props.selfManaged, date.value, current.signal),
    getDesktopSummaryReport(props.organizationId, props.selfManaged, date.value, current.signal),
  ])
  if (current.signal.aborted || disposed) return
  if (results[0].status === 'fulfilled') members.value = results[0].value
  if (results[1].status === 'fulfilled') summary.value = results[1].value
  if (results.some(result => result.status === 'rejected')) error.value = t(`${key}.loadFailed`)
  loading.value = false
  if (state.value?.task && ['pending', 'waiting', 'running'].includes(state.value.status)) timer = setTimeout(load, 15000)
}
function requestRun(mode: DesktopReportMode) { if (mode === 'regenerate') confirmRegenerate.value = true; else void execute(mode) }
async function execute(mode: DesktopReportMode) {
  confirmRegenerate.value = false; running.value = true; error.value = ''
  try { await runDesktopReports(props.organizationId, date.value, mode); if (!disposed) await load() }
  catch { if (!disposed) error.value = t(`${key}.runFailed`) }
  finally { running.value = false }
}
async function openSource(id: string) {
  sourceController?.abort(); const current = new AbortController(); sourceController = current; sourceLoading.value = true
  try { const record = await getDesktopConversation(props.organizationId, props.selfManaged, id, current.signal); if (!current.signal.aborted && !disposed) source.value = record }
  catch { if (!current.signal.aborted && !disposed) error.value = t(`${key}.loadFailed`) }
  finally { if (!current.signal.aborted) sourceLoading.value = false }
}
function tabKeydown(event: KeyboardEvent, index: number) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault(); activeTab.value = reportTabs[event.key === 'Home' ? 0 : event.key === 'End' ? 1 : 1 - index]
  void nextTick(() => document.getElementById(`daily-report-tab-${activeTab.value}`)?.focus())
}
watch([date, () => props.organizationId], () => { members.value = undefined; summary.value = undefined; selectedMember.value = ''; source.value = null; void load() }, { immediate: true })
watch(() => props.enabled, () => { if (props.selfManaged && !props.enabled) cancel(); else void load() })
onBeforeUnmount(() => { disposed = true; cancel() })
</script>
<style scoped>
.report-content :deep(h1), .report-content :deep(h2), .report-content :deep(h3) { margin: 1rem 0 0.5rem; font-weight: 600; }
.report-content :deep(p), .report-content :deep(ul), .report-content :deep(ol) { margin: 0.5rem 0; }
.report-content :deep(ul) { list-style: disc; padding-left: 1.5rem; }
.report-content :deep(ol) { list-style: decimal; padding-left: 1.5rem; }
.report-content :deep(pre) { overflow-x: auto; white-space: pre-wrap; }
</style>
