<template>
  <div ref="analyticsRoot" class="desktop-analytics space-y-6" data-testid="organization-usage-insights">
    <header>
      <p class="analytics-caption">{{ t('admin.desktop.usageStatistics.periodDetails', { from: statistics.range_start, to: statistics.range_end, timezone: statistics.timezone }) }}</p>
    </header>
    <dl class="analytics-grid" data-testid="organization-last-30-days">
      <div class="analytics-panel">
        <dt class="analytics-caption">{{ t('admin.desktop.usageStatistics.selectedCost') }} · USD</dt>
        <dd class="analytics-value"><bdi>{{ formatter.cost(statistics.selected.actual_cost) }}</bdi></dd>
        <dd class="analytics-caption mt-3">{{ t('admin.desktop.usageStatistics.previousComparison', { amount: formatter.cost(statistics.previous.actual_cost), change: percentageChange(statistics.selected.actual_cost, statistics.previous.actual_cost) }) }}</dd>
      </div>
      <div class="analytics-panel">
        <dt class="analytics-caption">{{ t('admin.desktop.usageStatistics.selectedTokens') }}</dt>
        <dd class="analytics-value"><bdi>{{ formatTokens(statistics.selected.total_tokens) }}</bdi></dd>
        <dd class="analytics-caption mt-3">{{ t('admin.desktop.usageStatistics.previousComparison', { amount: formatTokens(statistics.previous.total_tokens), change: percentageChange(statistics.selected.total_tokens, statistics.previous.total_tokens) }) }}</dd>
      </div>
    </dl>
    <section class="analytics-panel space-y-4" :aria-label="t('admin.desktop.usageStatistics.trend')">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <h4 class="analytics-heading">{{ t('admin.desktop.usageStatistics.trend') }}</h4>
          <p class="analytics-caption mt-1">{{ t('admin.desktop.usageStatistics.metricScope') }}</p>
        </div>
        <div class="flex flex-wrap gap-2" role="group" :aria-label="t('admin.desktop.usageStatistics.trendMetric')">
          <button v-for="metric in metrics" :key="metric" type="button" class="analytics-button" :aria-pressed="selectedMetric === metric" :data-testid="`usage-metric-${metric}`" @click="selectedMetric = metric">
            <Icon v-if="selectedMetric === metric" name="check" size="xs" aria-hidden="true" />{{ t(`admin.desktop.usageStatistics.${metric}`) }}
          </button>
        </div>
      </div>
      <p class="analytics-caption">{{ t('admin.desktop.usageStatistics.comparisonRange', { from: statistics.previous_start, to: statistics.previous_end }) }}</p>
      <div v-if="hasUsage" class="h-64 min-w-0" role="img" :aria-label="t('admin.desktop.usageStatistics.trend')">
        <Line :data="chartData" :options="chartOptions" />
      </div>
      <div v-else class="space-y-2 py-8 text-center">
        <p class="analytics-heading">{{ t('admin.desktop.usageStatistics.noUsage') }}</p>
        <p class="analytics-caption">{{ t('admin.desktop.usageStatistics.noMemberUsageHint') }}</p>
      </div>
      <DesktopAnalyticsDailyData :caption="t('admin.desktop.usageStatistics.trend')" :rows="statistics.daily" :columns="dailyColumns" />
    </section>

    <section class="analytics-panel space-y-4" :aria-label="t('admin.desktop.usageStatistics.tokenBreakdown')">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0"><h4 class="analytics-heading">{{ t('admin.desktop.usageStatistics.tokenBreakdown') }}</h4><p class="analytics-caption mt-1">{{ t('admin.desktop.usageStatistics.tokenUnitHint') }}</p></div>
        <div class="flex flex-wrap gap-2" role="group" :aria-label="t('admin.desktop.usageStatistics.numberDisplay')">
          <button v-for="mode in numberModes" :key="mode" type="button" class="analytics-button" :aria-pressed="numberMode === mode" :data-testid="`usage-token-${mode}`" @click="numberMode = mode"><Icon v-if="numberMode === mode" name="check" size="xs" aria-hidden="true" />{{ t(`admin.desktop.usageStatistics.${mode === 'compact' ? 'compactTokens' : 'exactTokens'}`) }}</button>
        </div>
      </div>
      <dl class="grid grid-cols-2 gap-5">
        <div v-for="part in breakdownParts" :key="part.key" class="min-w-0"><dt class="analytics-caption">{{ t(`admin.desktop.usageStatistics.${part.key}`) }}</dt><dd class="analytics-value"><bdi>{{ formatTokens(part.value) }}</bdi></dd></div>
      </dl>
    </section>

    <div class="analytics-grid">
      <section class="analytics-panel" :aria-label="t('admin.desktop.usageStatistics.models')">
        <h4 class="analytics-heading">{{ t('admin.desktop.usageStatistics.models') }}</h4>
        <p class="analytics-caption mt-1">{{ t('admin.desktop.usageStatistics.rankingHint') }} {{ t(`admin.desktop.usageStatistics.${selectedMetric}`) }}</p>
        <ol v-if="rankedModels.length" class="mt-5 space-y-5" data-testid="organization-model-ranking">
          <li v-for="model in rankedModels" :key="model.model">
            <div class="analytics-rank-row text-sm text-gray-900 dark:text-gray-100"><span class="min-w-0 break-all"><bdi>{{ model.model }}</bdi></span><span class="analytics-rank-value"><bdi>{{ formatValue(model) }}</bdi></span></div>
            <div class="analytics-bar" aria-hidden="true"><div :style="{ width: `${percentage(model)}%` }" /></div>
            <p class="analytics-caption mt-1">{{ t('admin.desktop.usageStatistics.shareAndRecords', { percent: percentage(model).toFixed(1), count: formatter.number(model.requests) }) }}</p>
          </li>
        </ol>
        <p v-else class="analytics-caption mt-4">{{ t('admin.desktop.usageStatistics.noUsage') }}</p>
      </section>
      <section class="analytics-panel" :aria-label="t('admin.desktop.usageStatistics.members')">
        <h4 class="analytics-heading">{{ t('admin.desktop.usageStatistics.members') }}</h4>
        <p class="analytics-caption mt-1">{{ t('admin.desktop.usageStatistics.observedMembers', { count: formatter.number(statistics.observed_members) }) }} · {{ t(`admin.desktop.usageStatistics.${selectedMetric}`) }}</p>
        <p class="analytics-caption mt-1">{{ t('admin.desktop.usageStatistics.memberHint') }} {{ t('admin.desktop.usageStatistics.rankingHint') }}</p>
        <ol v-if="rankedMembers.length" class="mt-5 space-y-5" data-testid="organization-member-ranking">
          <li v-for="member in rankedMembers" :key="member.member_id">
            <div class="analytics-rank-row text-sm text-gray-900 dark:text-gray-100"><span class="min-w-0 break-words"><bdi>{{ member.name }}</bdi> <span v-if="member.deleted" class="analytics-caption">{{ t('admin.desktop.conversations.deletedMember') }}</span></span><span class="analytics-rank-value"><bdi>{{ formatValue(member) }}</bdi></span></div>
            <div class="analytics-bar" aria-hidden="true"><div :style="{ width: `${percentage(member)}%` }" /></div>
            <p class="analytics-caption mt-1">{{ t('admin.desktop.usageStatistics.shareAndRecords', { percent: percentage(member).toFixed(1), count: formatter.number(member.requests) }) }}</p>
          </li>
        </ol>
        <p v-else class="analytics-caption mt-4">{{ t('admin.desktop.usageStatistics.noUsage') }}</p>
      </section>
    </div>
    <DesktopMemberModelUsage v-model:number-mode="numberMode" :rows="statistics.member_models" @refresh="emit('refresh')" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDesktopAnalyticsColors } from '@/composables/useDesktopAnalyticsColors'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { DesktopOrganizationUsageStatistics, DesktopUsageRank } from '@/api/desktopOrganizationUsage'
import { createDesktopUsageFormatter, type TokenNumberMode } from '@/utils/desktopUsageFormat'
import Icon from '@/components/icons/Icon.vue'
import DesktopAnalyticsDailyData from './DesktopAnalyticsDailyData.vue'
import '@/styles/desktopAnalytics.css'
import DesktopMemberModelUsage from './DesktopMemberModelUsage.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const props = defineProps<{ statistics: DesktopOrganizationUsageStatistics }>()
const emit = defineEmits<{ refresh: [] }>()
const { t, locale } = useI18n()
const numberMode = ref<TokenNumberMode>('compact')
const numberModes = ['compact', 'exact'] as const
const formatter = computed(() => createDesktopUsageFormatter(locale?.value || 'zh-CN'))
const formatTokens = (value: number) => formatter.value.tokens(value, numberMode.value)
const dailyColumns = computed(() => [
  { key: 'actual_cost' as const, label: `${t('admin.desktop.usageStatistics.cost')} · USD`, format: formatter.value.cost },
  { key: 'total_tokens' as const, label: 'Token', format: formatter.value.number },
])
const metrics = ['cost', 'tokens'] as const
const selectedMetric = ref<(typeof metrics)[number]>('cost')
const hasUsage = computed(() => props.statistics.selected.actual_cost !== 0 || props.statistics.selected.total_tokens !== 0)
const rankingKey = computed(() => selectedMetric.value === 'cost' ? 'cost_rank' : 'token_rank')
const rankedModels = computed(() => props.statistics.models.filter(model => model[rankingKey.value] <= 10).sort((a, b) => a[rankingKey.value] - b[rankingKey.value]))
const rankedMembers = computed(() => props.statistics.members.filter(member => member[rankingKey.value] <= 10).sort((a, b) => a[rankingKey.value] - b[rankingKey.value]))
const analyticsRoot = ref<HTMLElement | null>(null)
const chartColors = useDesktopAnalyticsColors(analyticsRoot)
const chartData = computed(() => ({
  labels: props.statistics.daily.map(day => day.date.slice(5)),
  datasets: [{
    label: t(`admin.desktop.usageStatistics.${selectedMetric.value}`),
    data: props.statistics.daily.map(day => day[selectedMetric.value === 'cost' ? 'actual_cost' : 'total_tokens']),
    borderColor: chartColors.value.line,
    borderWidth: 2,
    backgroundColor: chartColors.value.fill,
    fill: true,
    tension: 0.25,
    pointRadius: 0,
    pointHoverRadius: 4,
  }],
}))
const chartOptions = computed(() => {
  const { text, grid } = chartColors.value
  return {
    animation: false as const,
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: { legend: { display: false }, tooltip: { callbacks: { label: (item: { parsed: { y: number | null } }) => selectedMetric.value === 'cost' ? formatter.value.cost(item.parsed.y ?? 0) : `${formatter.value.number(item.parsed.y ?? 0)} Token` } } },
    scales: {
      x: { grid: { display: false }, ticks: { color: text, maxTicksLimit: 6, maxRotation: 0 } },
      y: { beginAtZero: true, grid: { color: grid }, ticks: { color: text, maxTicksLimit: 5, callback: (value: string | number) => selectedMetric.value === 'cost' ? `$${Number(value).toLocaleString(locale?.value || 'en', { maximumFractionDigits: 4 })}` : formatter.value.tokens(Number(value)) } },
    },
  }
})
const breakdownParts = computed(() => [
  { key: 'inputTokens', value: props.statistics.breakdown.input_tokens },
  { key: 'outputTokens', value: props.statistics.breakdown.output_tokens },
  { key: 'cacheCreationTokens', value: props.statistics.breakdown.cache_creation_tokens },
  { key: 'cacheReadTokens', value: props.statistics.breakdown.cache_read_tokens },
])
function formatValue(value: DesktopUsageRank) {
  return selectedMetric.value === 'cost' ? formatter.value.cost(value.actual_cost) : formatTokens(value.total_tokens)
}
function percentageChange(current: number, previous: number) {
  if (previous === 0) return t('admin.desktop.usageStatistics.noComparison')
  const change = ((current - previous) / previous * 100).toFixed(1)
  return `${Number(change) > 0 ? '+' : ''}${change}%`
}
function percentage(value: DesktopUsageRank) {
  const amount = selectedMetric.value === 'cost' ? value.actual_cost : value.total_tokens
  const total = props.statistics.selected[selectedMetric.value === 'cost' ? 'actual_cost' : 'total_tokens']
  return total > 0 ? Math.min(100, Math.max(0, amount / total * 100)) : 0
}
</script>
