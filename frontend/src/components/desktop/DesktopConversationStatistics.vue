<template>
  <section ref="analyticsRoot" class="space-y-4" :class="{ 'desktop-analytics': view === 'usage' }" :aria-label="t(view === 'usage' ? 'admin.desktop.conversations.statistics.usageTitle' : 'admin.desktop.conversations.statistics.title')" :aria-busy="loading" data-testid="conversation-statistics">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="font-semibold text-gray-900 dark:text-gray-100" :class="view === 'usage' ? 'text-lg' : 'text-sm'">{{ t(view === 'usage' ? 'admin.desktop.conversations.statistics.usageTitle' : 'admin.desktop.conversations.statistics.title') }}</h3>
      <button ref="refreshButton" class="btn btn-ghost btn-sm gap-1.5" :class="{ 'analytics-button': view === 'usage' }" type="button" :disabled="loading" @click="load">
        <Icon name="refresh" size="sm" aria-hidden="true" />{{ t('common.refresh') }}
      </button>
    </div>
    <div v-if="error" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
      {{ t('admin.desktop.conversations.statistics.loadFailed') }}
      <button class="ms-2 underline" :class="{ 'analytics-button': view === 'usage' }" type="button" @click="load">{{ t('admin.desktop.conversations.retry') }}</button>
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
    <div v-if="view === 'usage' && statistics && !error" class="space-y-6" data-testid="conversation-usage-analysis">
      <p class="analytics-caption">{{ t('admin.desktop.usageStatistics.periodDetails', { from: statistics.range_start, to: statistics.range_end, timezone: statistics.timezone }) }}</p>
      <dl class="analytics-tiles">
        <div v-for="item in usageMetrics" :key="item.key" class="analytics-panel"><dt class="analytics-caption">{{ t(`admin.desktop.conversations.statistics.${item.key}`) }}</dt><dd class="analytics-value"><bdi>{{ item.display }}</bdi></dd></div>
      </dl>
      <div class="analytics-panel space-y-2">
        <p class="analytics-caption font-medium">{{ t('admin.desktop.conversations.statistics.durationCoverage', { measured: statistics.total.duration_record_count.toLocaleString(), total: statistics.total.record_count.toLocaleString() }) }}</p>
        <p class="analytics-caption">{{ t('admin.desktop.conversations.statistics.durationHint') }}</p>
      </div>
      <section class="analytics-panel space-y-4" :aria-label="t('admin.desktop.conversations.statistics.trend')">
        <h4 class="analytics-heading">{{ t('admin.desktop.conversations.statistics.trend') }}</h4>
        <div v-if="statistics.daily.some(day => day.record_count > 0)" class="h-56 min-w-0" role="img" :aria-label="t('admin.desktop.conversations.statistics.trend')"><Line :data="trendData" :options="trendOptions" /></div>
        <div v-else class="space-y-2 py-8 text-center"><p class="analytics-heading">{{ t('admin.desktop.conversations.statistics.noRecords') }}</p><p class="analytics-caption">{{ t('admin.desktop.conversations.statistics.noRecordsHint') }}</p></div>
        <DesktopAnalyticsDailyData :caption="t('admin.desktop.conversations.statistics.trend')" :rows="statistics.daily" :columns="dailyColumns" />
      </section>
      <div class="analytics-grid">
        <section class="analytics-panel space-y-4" :aria-label="t('admin.desktop.conversations.statistics.captureStatus')">
          <h4 class="analytics-heading">{{ t('admin.desktop.conversations.statistics.captureStatus') }}</h4>
          <dl class="grid grid-cols-2 gap-4"><div><dt class="analytics-caption">{{ t('admin.desktop.conversations.captured') }}</dt><dd class="analytics-value">{{ statistics.captured.toLocaleString() }}</dd></div><div><dt class="analytics-caption">{{ t('admin.desktop.conversations.responseMissing') }}</dt><dd class="analytics-value">{{ statistics.response_missing.toLocaleString() }}</dd></div></dl>
          <p class="analytics-caption">{{ t('admin.desktop.conversations.statistics.reportingHint') }}</p>
        </section>
        <section class="analytics-panel space-y-4" :aria-label="t('admin.desktop.conversations.statistics.clients')">
          <h4 class="analytics-heading">{{ t('admin.desktop.conversations.statistics.clients') }}</h4>
          <dl class="grid grid-cols-2 gap-4"><div><dt class="analytics-caption">ChatGPT Codex</dt><dd class="analytics-value">{{ statistics.chatgpt_codex.toLocaleString() }}</dd></div><div><dt class="analytics-caption">Workbuddy</dt><dd class="analytics-value">{{ statistics.workbuddy.toLocaleString() }}</dd></div></dl>
        </section>
      </div>
    </div>
    <div v-if="view === 'usage' && loading" class="analytics-tiles" aria-hidden="true"><div v-for="tile in 6" :key="tile" class="analytics-panel"><p class="analytics-caption">{{ t(`admin.desktop.conversations.statistics.${metricKeys[tile - 1]}`) }}</p><p class="analytics-value">—</p></div></div>
    <div v-if="view === 'records' && statistics && !error" class="grid gap-4 border-t border-gray-200 pt-4 text-sm dark:border-dark-700 sm:grid-cols-3" data-testid="conversation-reporting-breakdown">
      <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.last30Days') }}</p><p class="mt-1 font-semibold tabular-nums">{{ statistics.last_30_days.record_count.toLocaleString() }} {{ t('admin.desktop.conversations.statistics.records') }}</p></div>
      <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.captureStatus') }}</p><p class="mt-1 tabular-nums">{{ t('admin.desktop.conversations.captured') }} {{ statistics.captured_last_30_days.toLocaleString() }} · {{ t('admin.desktop.conversations.responseMissing') }} {{ statistics.response_missing_last_30_days.toLocaleString() }}</p></div>
      <div><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.clients') }}</p><p class="mt-1 tabular-nums">ChatGPT Codex {{ statistics.chatgpt_codex_last_30_days.toLocaleString() }} · Workbuddy {{ statistics.workbuddy_last_30_days.toLocaleString() }}</p></div>
    </div>
    <p v-if="loading" class="text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="view === 'records' && statistics" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">
      {{ t('admin.desktop.conversations.statistics.hint', { timezone: statistics.timezone }) }}
    </p>
    <p v-if="view === 'records' && statistics && !error" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.reportingHint') }}</p>
    <p v-if="view === 'records' && statistics && !error" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.durationHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useDesktopAnalyticsColors } from '@/composables/useDesktopAnalyticsColors'
import { createDesktopUsageFormatter } from '@/utils/desktopUsageFormat'
import DesktopAnalyticsDailyData from './DesktopAnalyticsDailyData.vue'
import '@/styles/desktopAnalytics.css'
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
const { t, locale } = useI18n()
const analyticsRoot = ref<HTMLElement | null>(null)
const refreshButton = ref<HTMLButtonElement | null>(null)
const chartColors = useDesktopAnalyticsColors(analyticsRoot)
const formatter = computed(() => createDesktopUsageFormatter(locale?.value || 'zh-CN'))
const dailyColumns = computed(() => [
  { key: 'record_count' as const, label: t('admin.desktop.conversations.statistics.records'), format: formatter.value.number },
  { key: 'prompt_count' as const, label: t('admin.desktop.conversations.statistics.promptsLabel'), format: formatter.value.number },
])
const metricKeys = ['recordsLabel', 'promptsLabel', 'membersLabel', 'sessionsLabel', 'totalDurationLabel', 'averageDurationLabel']
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
  datasets: [{ label: t('admin.desktop.conversations.statistics.recordsLabel'), data: statistics.value?.daily.map(day => day.record_count) || [], borderColor: chartColors.value.line, borderWidth: 2, backgroundColor: chartColors.value.fill, fill: true, pointRadius: 0, pointHoverRadius: 4, tension: 0.25 }],
}))
const trendOptions = computed(() => {
  const { text, grid } = chartColors.value
  return {
    animation: false as const,
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: false } },
    scales: {
      x: { grid: { display: false }, ticks: { color: text, maxTicksLimit: 6, maxRotation: 0 } },
      y: { beginAtZero: true, grid: { color: grid }, ticks: { color: text, precision: 0, maxTicksLimit: 5, callback: (value: string | number) => formatter.value.tokens(Number(value)) } },
    },
  }
})
const loading = ref(false)
const error = ref(false)
let controller: AbortController | undefined

async function load() {
  const restoreFocus = props.view === 'usage' && Boolean(document.activeElement?.closest('[data-testid="conversation-statistics"]'))
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
    if (!request.signal.aborted) {
      loading.value = false
      if (restoreFocus) {
        await nextTick()
        const focused = document.activeElement
        if (!request.signal.aborted && (!focused || focused === document.body || focused === refreshButton.value)) refreshButton.value?.focus()
      }
    }
  }
}
watch(() => [props.organizationId, props.selfManaged, props.filters, props.range], load, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>
