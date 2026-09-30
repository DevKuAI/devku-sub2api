<template>
  <section class="space-y-3" :aria-label="t(view === 'usage' ? 'admin.desktop.conversations.statistics.usageTitle' : 'admin.desktop.conversations.statistics.title')" :aria-busy="loading" data-testid="conversation-statistics">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t(view === 'usage' ? 'admin.desktop.conversations.statistics.usageTitle' : 'admin.desktop.conversations.statistics.title') }}</h3>
      <button class="btn btn-ghost btn-sm gap-1.5" type="button" :disabled="loading" @click="load">
        <Icon name="refresh" size="sm" aria-hidden="true" />{{ t('common.refresh') }}
      </button>
    </div>
    <div v-if="error" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
      {{ t('admin.desktop.conversations.statistics.loadFailed') }}
      <button class="ml-2 underline" type="button" @click="load">{{ t('admin.desktop.conversations.retry') }}</button>
    </div>
    <div v-else-if="view === 'records'" class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <article v-for="period in periods" :key="period.key" class="min-w-0 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800" :data-testid="`statistics-${period.key}`">
        <div class="flex items-center justify-between gap-2 text-sm text-gray-500 dark:text-dark-400">
          <h4>{{ t(`admin.desktop.conversations.statistics.${period.key}`) }}</h4>
          <Icon :name="period.icon" size="md" class="shrink-0 text-primary-500 dark:text-primary-400" aria-hidden="true" />
        </div>
        <div class="mt-3 flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <span class="break-all text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100">{{ statistics ? statistics[period.key].record_count.toLocaleString() : '—' }}</span>
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.records') }}</span>
        </div>
        <p class="mt-2 break-words text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.prompts', { count: statistics ? statistics[period.key].prompt_count.toLocaleString() : '—' }) }}</p>
        <p class="mt-1 break-words text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.durationTotal', { value: statistics && statistics[period.key].duration_record_count ? formatDesktopConversationDuration(statistics[period.key].total_duration_ms) : '—' }) }}</p>
        <p class="mt-1 break-words text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.durationAverage', { value: formatDesktopConversationDuration(statistics?.[period.key].average_duration_ms), count: statistics ? statistics[period.key].duration_record_count.toLocaleString() : '—' }) }}</p>
      </article>
    </div>
    <div v-if="view === 'usage' && statistics && !error" class="space-y-4" data-testid="conversation-usage-analysis">
      <div class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3 lg:grid-cols-6">
        <div v-for="item in usageMetrics" :key="item.key"><p class="text-xs text-gray-500 dark:text-dark-400">{{ t(`admin.desktop.conversations.statistics.${item.key}`) }}</p><p class="mt-1 font-semibold tabular-nums text-gray-900 dark:text-gray-100">{{ item.display }}</p></div>
      </div>
      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.durationCoverage', { measured: statistics.total.duration_record_count.toLocaleString(), total: statistics.total.record_count.toLocaleString() }) }}</p>
      <div v-if="statistics.daily.some(day => day.record_count > 0)" class="h-48" role="img" :aria-label="t('admin.desktop.conversations.statistics.trend')"><Line :data="trendData" :options="trendOptions" /></div>
      <p v-else class="py-6 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.noRecords') }}</p>
      <div class="sr-only"><table><caption>{{ t('admin.desktop.conversations.statistics.trend') }}</caption><thead><tr><th>{{ t('admin.desktop.usageStatistics.date') }}</th><th>{{ t('admin.desktop.conversations.statistics.records') }}</th><th>{{ t('admin.desktop.conversations.statistics.promptsLabel') }}</th></tr></thead><tbody><tr v-for="day in statistics.daily" :key="day.date"><td>{{ day.date }}</td><td>{{ day.record_count }}</td><td>{{ day.prompt_count }}</td></tr></tbody></table></div>
      <div class="grid gap-4 border-t border-gray-200 pt-4 text-sm dark:border-dark-700 sm:grid-cols-2">
        <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.captureStatus') }}</p><p class="mt-1 tabular-nums">{{ t('admin.desktop.conversations.captured') }} {{ statistics.captured.toLocaleString() }} · {{ t('admin.desktop.conversations.responseMissing') }} {{ statistics.response_missing.toLocaleString() }}</p></div>
        <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.clients') }}</p><p class="mt-1 tabular-nums">ChatGPT Codex {{ statistics.chatgpt_codex.toLocaleString() }} · Workbuddy {{ statistics.workbuddy.toLocaleString() }}</p></div>
      </div>
    </div>
    <div v-if="view === 'records' && statistics && !error" class="grid gap-4 border-t border-gray-200 pt-4 text-sm dark:border-dark-700 sm:grid-cols-3" data-testid="conversation-reporting-breakdown">
      <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.last30Days') }}</p><p class="mt-1 font-semibold tabular-nums">{{ statistics.last_30_days.record_count.toLocaleString() }} {{ t('admin.desktop.conversations.statistics.records') }}</p></div>
      <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.captureStatus') }}</p><p class="mt-1 tabular-nums">{{ t('admin.desktop.conversations.captured') }} {{ statistics.captured_last_30_days.toLocaleString() }} · {{ t('admin.desktop.conversations.responseMissing') }} {{ statistics.response_missing_last_30_days.toLocaleString() }}</p></div>
      <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.clients') }}</p><p class="mt-1 tabular-nums">ChatGPT Codex {{ statistics.chatgpt_codex_last_30_days.toLocaleString() }} · Workbuddy {{ statistics.workbuddy_last_30_days.toLocaleString() }}</p></div>
    </div>
    <p v-if="loading" class="text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="view === 'records' && statistics" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">
      {{ t('admin.desktop.conversations.statistics.hint', { timezone: statistics.timezone }) }}
    </p>
    <p v-if="statistics && !error" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.reportingHint') }}</p>
    <p v-if="statistics && !error" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.durationHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useMutationObserver } from '@vueuse/core'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler } from 'chart.js'
import { Line } from 'vue-chartjs'
import { useI18n } from 'vue-i18n'
import { getDesktopConversationStatistics } from '@/api/desktopConversations'
import type { DesktopConversationFilters, DesktopConversationStatistics } from '@/api/desktopConversations'
import type { DesktopAnalyticsRange } from '@/api/desktopOrganizationUsage'
import Icon from '@/components/icons/Icon.vue'
import { formatDesktopConversationDuration } from '@/utils/desktopConversationDuration'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const props = withDefaults(defineProps<{ organizationId: string; selfManaged: boolean; filters: DesktopConversationFilters; range?: DesktopAnalyticsRange; view?: 'records' | 'usage' }>(), { view: 'records' })
const { t } = useI18n()
const isDark = ref(document.documentElement.classList.contains('dark'))
useMutationObserver(document.documentElement, () => { isDark.value = document.documentElement.classList.contains('dark') }, { attributes: true, attributeFilter: ['class'] })
const periods = [
  { key: 'today', icon: 'clock' },
  { key: 'week', icon: 'calendar' },
  { key: 'month', icon: 'chartBar' },
  { key: 'total', icon: 'chat' },
] as const
const statistics = ref<DesktopConversationStatistics | null>(null)
const usageMetrics = computed(() => statistics.value ? [
  { key: 'recordsLabel', display: statistics.value.total.record_count.toLocaleString() },
  { key: 'promptsLabel', display: statistics.value.total.prompt_count.toLocaleString() },
  { key: 'membersLabel', display: statistics.value.distinct_members.toLocaleString() },
  { key: 'sessionsLabel', display: statistics.value.distinct_sessions.toLocaleString() },
  { key: 'totalDurationLabel', display: statistics.value.total.duration_record_count ? formatDesktopConversationDuration(statistics.value.total.total_duration_ms) : '—' },
  { key: 'averageDurationLabel', display: formatDesktopConversationDuration(statistics.value.total.average_duration_ms) },
] : [])
const trendData = computed(() => ({
  labels: statistics.value?.daily.map(day => day.date.slice(5)) || [],
  datasets: [{ label: t('admin.desktop.conversations.statistics.recordsLabel'), data: statistics.value?.daily.map(day => day.record_count) || [], borderColor: '#0d9488', backgroundColor: 'rgba(13, 148, 136, 0.12)', fill: true, pointRadius: 0, pointHoverRadius: 4, tension: 0.25 }],
}))
const trendOptions = computed(() => {
  const text = isDark.value ? '#9ca3af' : '#4b5563'
  const grid = isDark.value ? 'rgba(148, 163, 184, 0.18)' : '#e5e7eb'
  return {
    animation: false as const,
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: false } },
    scales: {
      x: { grid: { display: false }, ticks: { color: text, maxTicksLimit: 6, maxRotation: 0 } },
      y: { beginAtZero: true, grid: { color: grid }, ticks: { color: text, precision: 0, maxTicksLimit: 5 } },
    },
  }
})
const loading = ref(false)
const error = ref(false)
let controller: AbortController | undefined

async function load() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  statistics.value = null
  error.value = false
  loading.value = false
  if (!props.organizationId) return
  loading.value = true
  try {
    const result = props.view === 'usage'
      ? await getDesktopConversationStatistics(props.organizationId, props.selfManaged, props.filters, request.signal, props.range)
      : await getDesktopConversationStatistics(props.organizationId, props.selfManaged, props.filters, request.signal)
    if (!request.signal.aborted) statistics.value = result
  } catch {
    if (!request.signal.aborted) error.value = true
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
}
watch(() => [props.organizationId, props.selfManaged, props.filters, props.range], load, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>
