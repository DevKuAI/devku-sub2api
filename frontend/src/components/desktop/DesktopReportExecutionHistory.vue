<template>
  <details v-if="executions.length" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700" data-testid="report-execution-history">
    <summary class="cursor-pointer text-sm font-medium">{{ t(`${key}.title`, { count: executions.length }) }}</summary>
    <p class="mt-2 text-xs text-gray-500">{{ t(`${key}.hint`) }}</p>
    <div class="mt-3 overflow-x-auto" tabindex="0" :aria-label="t(`${key}.label`)">
      <table class="w-full min-w-[48rem] text-left text-xs">
        <thead><tr><th class="p-2">{{ t(`${key}.model`) }}</th><th class="p-2">{{ t(`${key}.revision`) }}</th><th class="p-2">{{ t(`${key}.chunk`) }}</th><th class="p-2">{{ t(`${key}.attempt`) }}</th><th class="p-2">{{ t(`${key}.startedAt`) }}</th><th class="p-2">{{ t('common.status') }}</th></tr></thead>
        <tbody><tr v-for="execution in executions" :key="execution.id" class="border-t border-gray-100 dark:border-dark-700"><td class="p-2">{{ execution.model }}</td><td class="p-2">{{ execution.revision }}</td><td class="p-2">{{ execution.round + 1 }} / {{ execution.chunk }}</td><td class="p-2">{{ execution.attempt }}</td><td class="whitespace-nowrap p-2">{{ formatDateTime(execution.started_at) }}</td><td class="p-2">{{ t(`${key}.statuses.${execution.status}`) }}</td></tr></tbody>
      </table>
    </div>
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
