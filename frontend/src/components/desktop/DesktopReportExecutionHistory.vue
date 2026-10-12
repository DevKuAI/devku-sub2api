<template>
  <details v-if="executions.length" class="report-history" data-testid="report-execution-history">
    <summary class="min-h-11 cursor-pointer py-3 text-sm font-medium leading-relaxed">{{ t(`${key}.title`, { count: executions.length }) }}</summary>
    <p class="analytics-caption mt-1">{{ t(`${key}.hint`) }}</p>
    <ol class="mt-4 space-y-4" :aria-label="t(`${key}.label`)">
      <li v-for="execution in executions" :key="execution.id" class="execution-item">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="min-w-0"><p class="analytics-caption">{{ t(`${key}.model`) }}</p><p class="mt-1 break-words text-sm font-medium"><bdi>{{ execution.model }}</bdi></p></div>
          <span class="execution-status" :data-status="execution.status">{{ t(`${key}.statuses.${execution.status}`) }}</span>
        </div>
        <dl class="execution-metadata mt-4">
          <div><dt class="analytics-caption">{{ t(`${key}.tokens`) }}</dt><dd>{{ execution.total_tokens == null ? '—' : execution.total_tokens.toLocaleString() }}</dd></div>
          <div><dt class="analytics-caption">{{ t(`${key}.cost`) }}</dt><dd>{{ execution.actual_cost == null ? '—' : `$${execution.actual_cost.toFixed(6)}` }}</dd></div>
          <div><dt class="analytics-caption">{{ t(`${key}.revision`) }}</dt><dd>{{ execution.revision }}</dd></div>
          <div><dt class="analytics-caption">{{ t(`${key}.chunk`) }}</dt><dd>{{ execution.round + 1 }} / {{ execution.chunk }}</dd></div>
          <div><dt class="analytics-caption">{{ t(`${key}.attempt`) }}</dt><dd>{{ execution.attempt }}</dd></div>
          <div><dt class="analytics-caption">{{ t(`${key}.startedAt`) }}</dt><dd>{{ formatDateTime(execution.started_at) }}</dd></div>
        </dl>
      </li>
    </ol>
  </details>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import type { DesktopReportExecution } from '@/api/desktopReports'
defineProps<{ executions: DesktopReportExecution[] }>()
const { t } = useI18n()
const key = 'admin.desktop.reports.executions'
</script>
<style scoped>
.report-history { @apply min-w-0 border-t border-gray-200 pt-2 text-gray-900 dark:border-dark-700 dark:text-gray-100; }
.execution-item { @apply rounded-lg bg-gray-50 p-4 dark:bg-dark-800; overflow-wrap: anywhere; }
.execution-metadata { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.execution-metadata dd { @apply mt-1 text-sm leading-relaxed tabular-nums; }
.execution-status { @apply max-w-full rounded-md bg-gray-100 px-2.5 py-1 text-sm leading-normal text-gray-700 dark:bg-dark-700 dark:text-dark-200; }
.execution-status[data-status='completed'] { @apply bg-emerald-50 text-emerald-800 dark:bg-dark-900 dark:text-emerald-200; }
.execution-status[data-status='failed'] { @apply bg-red-50 text-red-700 dark:bg-dark-900 dark:text-red-300; }
.execution-status[data-status='running'] { @apply bg-primary-50 text-primary-700 dark:bg-dark-900 dark:text-primary-300; }
.execution-status[data-status='blocked'] { @apply bg-amber-50 text-amber-900 dark:bg-dark-900 dark:text-amber-200; }
@container (min-width: 36rem) { .execution-metadata { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
</style>
