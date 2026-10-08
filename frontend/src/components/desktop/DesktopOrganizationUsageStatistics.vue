<template>
  <section class="space-y-4" :class="{ 'desktop-analytics': view === 'insights' }" :aria-label="t(view === 'insights' ? 'admin.desktop.usageStatistics.tab' : 'admin.desktop.usageStatistics.title')" :aria-busy="loading" data-testid="organization-usage-statistics">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="font-semibold text-gray-900 dark:text-gray-100" :class="view === 'insights' ? 'text-lg' : 'text-sm'">{{ t(view === 'insights' ? 'admin.desktop.usageStatistics.tab' : 'admin.desktop.usageStatistics.title') }}</h3>
      <button ref="refreshButton" class="btn btn-ghost btn-sm gap-1.5" :class="{ 'member-usage-refresh analytics-button': view === 'insights' }" type="button" :disabled="loading" @click="load">
        <Icon name="refresh" size="sm" aria-hidden="true" />{{ t('common.refresh') }}
      </button>
    </div>
    <div v-if="error" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
      {{ t('admin.desktop.usageStatistics.loadFailed') }}
      <button class="ms-2 underline" :class="{ 'member-usage-refresh analytics-button': view === 'insights' }" type="button" @click="load">{{ t('admin.desktop.usageStatistics.retry') }}</button>
    </div>
    <div v-else-if="view === 'summary'" class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <article v-for="period in periods" :key="period.key" class="min-w-0 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800" :data-testid="`organization-usage-${period.key}`">
        <div class="flex items-center justify-between gap-2 text-sm text-gray-500 dark:text-dark-400">
          <h4>{{ t(`admin.desktop.usageStatistics.${period.key}`) }}</h4>
          <Icon :name="period.icon" size="md" class="shrink-0 text-primary-500 dark:text-primary-400" aria-hidden="true" />
        </div>
        <p class="mt-3 break-all text-base font-semibold tabular-nums text-gray-900 dark:text-gray-100 sm:text-2xl">{{ statistics ? `$${statistics[period.key].actual_cost.toFixed(4)}` : '—' }}</p>
        <div class="mt-2 flex flex-wrap items-baseline justify-between gap-x-2 gap-y-1 text-xs">
          <span class="text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageTokens') }}</span>
          <span class="break-all font-medium tabular-nums text-gray-700 dark:text-dark-200" :title="statistics?.[period.key].total_tokens.toLocaleString()">{{ statistics ? formatCompactNumber(statistics[period.key].total_tokens) : '—' }}</span>
        </div>
      </article>
    </div>
    <div v-if="view === 'insights' && loading" class="space-y-6">
      <div class="analytics-grid" aria-hidden="true"><div v-for="tile in 2" :key="tile" class="analytics-panel"><p class="analytics-caption">{{ t(tile === 1 ? 'admin.desktop.usageStatistics.selectedCost' : 'admin.desktop.usageStatistics.selectedTokens') }}</p><p class="analytics-value">—</p></div></div>
      <DesktopMemberModelUsage :rows="[]" loading />
    </div>
    <DesktopOrganizationUsageInsights v-if="view === 'insights' && statistics && !error" :statistics="statistics" @refresh="load" />
    <p v-if="loading" class="text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="view === 'summary' && statistics" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.hint', { timezone: statistics.timezone }) }}</p>
  </section>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDesktopOrganizationUsageStatistics } from '@/api/desktopOrganizationUsage'
import type { DesktopOrganizationUsageStatistics } from '@/api/desktopOrganizationUsage'
import type { DesktopAnalyticsRange } from '@/api/desktopOrganizationUsage'
import Icon from '@/components/icons/Icon.vue'
import DesktopOrganizationUsageInsights from './DesktopOrganizationUsageInsights.vue'
import DesktopMemberModelUsage from './DesktopMemberModelUsage.vue'
import '@/styles/desktopAnalytics.css'
import { formatCompactNumber } from '@/utils/format'

const props = withDefaults(defineProps<{ organizationId: string; selfManaged: boolean; view?: 'summary' | 'insights'; range?: DesktopAnalyticsRange }>(), { view: 'summary' })
const { t } = useI18n()
const periods = [{ key: 'today', icon: 'clock' }, { key: 'week', icon: 'calendar' }, { key: 'month', icon: 'chartBar' }, { key: 'total', icon: 'database' }] as const
const statistics = ref<DesktopOrganizationUsageStatistics | null>(null)
const loading = ref(false)
const error = ref(false)
const refreshButton = ref<HTMLButtonElement | null>(null)
let controller: AbortController | undefined

async function load() {
  const restoreFocus = props.view === 'insights' && Boolean(document.activeElement?.closest('[data-testid="organization-usage-statistics"]'))
  controller?.abort()
  const request = new AbortController()
  controller = request
  statistics.value = null
  error.value = false
  loading.value = false
  if (!props.organizationId) return
  loading.value = true
  try {
    const result = props.view === 'insights'
      ? await getDesktopOrganizationUsageStatistics(props.organizationId, props.selfManaged, request.signal, props.range)
      : await getDesktopOrganizationUsageStatistics(props.organizationId, props.selfManaged, request.signal)
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
watch(() => [props.organizationId, props.selfManaged, props.range], load, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>

<style scoped>
.member-usage-refresh { min-height: 44px; transition: none; }
.member-usage-refresh:active { transform: none; }
.member-usage-refresh:focus-visible { outline: 2px solid theme('colors.primary.600'); outline-offset: 3px; }
html.dark .member-usage-refresh:focus-visible { outline-color: theme('colors.primary.400'); }
.member-usage-refresh:hover { background-color: transparent; }
@media (hover: hover) { .member-usage-refresh:hover { @apply bg-gray-100 dark:bg-dark-800; } }
@media (forced-colors: active) { .member-usage-refresh:focus-visible { outline-color: Highlight; } }
</style>
