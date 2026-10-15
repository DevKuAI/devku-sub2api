<template>
  <section class="card bi-panel" :aria-busy="loading" aria-labelledby="bi-report-statistics-title">
    <header class="bi-toolbar">
      <div class="bi-statistics-heading"><h2 id="bi-report-statistics-title" class="bi-heading">{{ t('biNative.reportStatistics') }}</h2><p class="bi-caption mt-2">{{ t('biNative.statisticsScope') }}</p></div>
      <div class="bi-statistics-controls"><slot name="filters" /><button class="btn btn-secondary" type="button" :disabled="loading" @click="$emit('refresh')">{{ t('common.refresh') }}</button></div>
    </header>
    <p v-if="loading" class="bi-caption" role="status">{{ t('common.loading') }}</p>
    <div v-if="error" class="bi-error" role="alert"><p>{{ error }}</p><button class="btn btn-secondary mt-3" type="button" @click="$emit('refresh')">{{ t('biNative.retry') }}</button></div>
    <template v-if="statistics">
      <dl class="bi-metrics bi-report-metrics">
        <div v-for="metric in metrics" :key="metric.key" class="bi-metric"><dt class="bi-caption">{{ t(`biNative.statistics.${metric.key}`) }}</dt><dd>{{ metric.value == null ? '-' : biNumber(metric.value) }}</dd><dd v-if="metric.missing && metric.missing !== '0'" class="bi-caption bi-metric-note">{{ t('biNative.missingUsage', { count: biNumber(metric.missing) }) }}</dd></div>
      </dl>
      <div class="bi-report-statuses">
        <div v-for="group in statusGroups" :key="group.key"><h3 class="bi-caption">{{ t(`biNative.statistics.${group.key}`) }}</h3><ul class="bi-status-list"><li v-for="[status, count] in Object.entries(group.values)" :key="status" class="bi-badge" :data-tone="biStatusTone(status)">{{ biStatusKey(status) ? t(biStatusKey(status)!) : status }} <span class="ms-2 tabular-nums">{{ biNumber(count) }}</span></li><li v-if="!Object.keys(group.values).length" class="bi-caption">-</li></ul></div>
      </div>
      <details class="bi-model-statistics"><summary>{{ t('biNative.modelStatistics') }} <span class="bi-caption ms-2">{{ statistics.models.length }}</span></summary><OperationsTable class="mt-4" :rows="statistics.models" :columns="['model','execution_count','total_tokens','actual_cost','missing_token_count','missing_cost_count']" :loading="false" error="" /></details>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BIReportStatistics } from '@/api/admin/bi'
import OperationsTable from './OperationsTable.vue'
import { biNumber, biStatusKey, biStatusTone } from './display'

const props = defineProps<{ statistics?: BIReportStatistics; loading: boolean; error: string }>()
defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const metrics = computed(() => props.statistics ? [
  { key: 'tasks', value: props.statistics.task_count },
  { key: 'organizations', value: props.statistics.organization_count },
  { key: 'models', value: props.statistics.models.filter(model => model.model.trim()).length },
  { key: 'executions', value: props.statistics.execution_count },
  { key: 'tokens', value: props.statistics.total_tokens, missing: props.statistics.missing_token_count },
  { key: 'cost', value: props.statistics.actual_cost, missing: props.statistics.missing_cost_count },
] : [])
const statusGroups = computed(() => props.statistics ? [
  { key: 'taskStatuses', values: props.statistics.task_statuses },
  { key: 'executionStatuses', values: props.statistics.execution_statuses },
] : [])
</script>
