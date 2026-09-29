<template>
  <div class="space-y-6 border-t border-gray-200 pt-5 dark:border-dark-700" data-testid="organization-usage-insights">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.desktop.usageStatistics.trend') }}</h4>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.selectedRange', { from: statistics.range_start, to: statistics.range_end }) }}</p>
      </div>
      <div class="inline-flex rounded border border-gray-200 p-0.5 dark:border-dark-700" role="group" :aria-label="t('admin.desktop.usageStatistics.trendMetric')">
        <button v-for="metric in metrics" :key="metric" type="button" class="rounded px-3 py-1.5 text-xs font-medium" :class="selectedMetric === metric ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900' : 'text-gray-600 dark:text-gray-300'" :aria-pressed="selectedMetric === metric" @click="selectedMetric = metric">
          {{ t(`admin.desktop.usageStatistics.${metric}`) }}
        </button>
      </div>
    </div>
    <dl class="grid grid-cols-2 gap-4 text-sm" data-testid="organization-last-30-days">
      <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.selectedCost') }}</dt><dd class="mt-1 font-semibold tabular-nums text-gray-900 dark:text-gray-100">${{ statistics.selected.actual_cost.toFixed(4) }}</dd><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.previousComparison', { amount: `$${statistics.previous.actual_cost.toFixed(4)}`, change: percentageChange(statistics.selected.actual_cost, statistics.previous.actual_cost) }) }}</p></div>
      <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.selectedTokens') }}</dt><dd class="mt-1 font-semibold tabular-nums text-gray-900 dark:text-gray-100" :title="statistics.selected.total_tokens.toLocaleString()">{{ formatCompactNumber(statistics.selected.total_tokens) }}</dd><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.previousComparison', { amount: formatCompactNumber(statistics.previous.total_tokens), change: percentageChange(statistics.selected.total_tokens, statistics.previous.total_tokens) }) }}</p></div>
    </dl>
    <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.comparisonRange', { from: statistics.previous_start, to: statistics.previous_end }) }}</p>
    <div v-if="hasUsage" class="h-56" role="img" :aria-label="t('admin.desktop.usageStatistics.trend')">
      <Line :data="chartData" :options="chartOptions" />
    </div>
    <p v-else class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.noUsage') }}</p>
    <div class="sr-only"><table>
      <caption>{{ t('admin.desktop.usageStatistics.trend') }}</caption>
      <thead><tr><th>{{ t('admin.desktop.usageStatistics.date') }}</th><th>{{ t('admin.desktop.usageStatistics.cost') }}</th><th>{{ t('admin.desktop.usageStatistics.tokens') }}</th></tr></thead>
      <tbody><tr v-for="day in statistics.daily" :key="day.date"><td>{{ day.date }}</td><td>{{ day.actual_cost }}</td><td>{{ day.total_tokens }}</td></tr></tbody>
    </table></div>

    <div class="grid gap-6 border-t border-gray-200 pt-5 dark:border-dark-700 lg:grid-cols-2">
      <section class="min-w-0" :aria-label="t('admin.desktop.usageStatistics.models')">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.desktop.usageStatistics.models') }}</h4>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.rankingHint') }}</p>
        <ol v-if="rankedModels.length" class="mt-4 space-y-3" data-testid="organization-model-ranking">
          <li v-for="model in rankedModels" :key="model.model">
            <div class="flex items-baseline justify-between gap-3 text-sm"><span class="min-w-0 break-all text-gray-800 dark:text-gray-200">{{ model.model }}</span><span class="shrink-0 tabular-nums">{{ formatValue(model) }}</span></div>
            <div class="mt-1 h-1.5 rounded bg-gray-100 dark:bg-dark-700"><div class="h-full rounded bg-teal-600 dark:bg-teal-400" :style="{ width: `${percentage(model)}%` }" /></div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.billingRecords', { count: model.requests.toLocaleString() }) }}</p>
          </li>
        </ol>
        <p v-else class="mt-4 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.noUsage') }}</p>
      </section>
      <section class="min-w-0" :aria-label="t('admin.desktop.usageStatistics.members')">
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.desktop.usageStatistics.members') }}</h4>
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.observedMembers', { count: statistics.observed_members.toLocaleString() }) }}</span>
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.memberHint') }}</p>
        <ol v-if="rankedMembers.length" class="mt-4 space-y-3" data-testid="organization-member-ranking">
          <li v-for="member in rankedMembers" :key="member.member_id">
            <div class="flex items-baseline justify-between gap-3 text-sm"><span class="min-w-0 break-words text-gray-800 dark:text-gray-200">{{ member.name }} <span v-if="member.deleted" class="text-xs text-gray-500">{{ t('admin.desktop.conversations.deletedMember') }}</span></span><span class="shrink-0 tabular-nums">{{ formatValue(member) }}</span></div>
            <div class="mt-1 h-1.5 rounded bg-gray-100 dark:bg-dark-700"><div class="h-full rounded bg-amber-500" :style="{ width: `${percentage(member)}%` }" /></div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.billingRecords', { count: member.requests.toLocaleString() }) }}</p>
          </li>
        </ol>
        <p v-else class="mt-4 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.noUsage') }}</p>
      </section>
    </div>

    <section class="border-t border-gray-200 pt-5 dark:border-dark-700" :aria-label="t('admin.desktop.usageStatistics.tokenBreakdown')">
      <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.desktop.usageStatistics.tokenBreakdown') }}</h4>
      <dl class="mt-3 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
        <div v-for="part in breakdownParts" :key="part.key"><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t(`admin.desktop.usageStatistics.${part.key}`) }}</dt><dd class="mt-1 font-medium tabular-nums text-gray-900 dark:text-gray-100" :title="part.value.toLocaleString()">{{ formatCompactNumber(part.value) }}</dd></div>
      </dl>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { DesktopOrganizationUsageStatistics, DesktopUsageRank } from '@/api/desktopOrganizationUsage'
import { formatCompactNumber } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const props = defineProps<{ statistics: DesktopOrganizationUsageStatistics }>()
const { t } = useI18n()
const metrics = ['cost', 'tokens'] as const
const selectedMetric = ref<(typeof metrics)[number]>('cost')
const hasUsage = computed(() => props.statistics.selected.actual_cost !== 0 || props.statistics.selected.total_tokens !== 0)
const rankingKey = computed(() => selectedMetric.value === 'cost' ? 'cost_rank' : 'token_rank')
const rankedModels = computed(() => props.statistics.models.filter(model => model[rankingKey.value] <= 10).sort((a, b) => a[rankingKey.value] - b[rankingKey.value]))
const rankedMembers = computed(() => props.statistics.members.filter(member => member[rankingKey.value] <= 10).sort((a, b) => a[rankingKey.value] - b[rankingKey.value]))
const chartData = computed(() => ({
  labels: props.statistics.daily.map(day => day.date.slice(5)),
  datasets: [{
    label: t(`admin.desktop.usageStatistics.${selectedMetric.value}`),
    data: props.statistics.daily.map(day => day[selectedMetric.value === 'cost' ? 'actual_cost' : 'total_tokens']),
    borderColor: selectedMetric.value === 'cost' ? '#0d9488' : '#d97706',
    backgroundColor: selectedMetric.value === 'cost' ? 'rgba(13, 148, 136, 0.12)' : 'rgba(217, 119, 6, 0.12)',
    fill: true,
    tension: 0.25,
    pointRadius: 0,
    pointHoverRadius: 4,
  }],
}))
const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: { legend: { display: false } },
  scales: {
    x: { grid: { display: false }, ticks: { maxTicksLimit: 6, maxRotation: 0 } },
    y: { beginAtZero: true, ticks: { maxTicksLimit: 5 } },
  },
}
const breakdownParts = computed(() => [
  { key: 'inputTokens', value: props.statistics.breakdown.input_tokens },
  { key: 'outputTokens', value: props.statistics.breakdown.output_tokens },
  { key: 'cacheCreationTokens', value: props.statistics.breakdown.cache_creation_tokens },
  { key: 'cacheReadTokens', value: props.statistics.breakdown.cache_read_tokens },
])
function formatValue(value: DesktopUsageRank) {
  return selectedMetric.value === 'cost' ? `$${value.actual_cost.toFixed(4)}` : formatCompactNumber(value.total_tokens)
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
