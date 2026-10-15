<template>
  <div :aria-busy="loading" class="min-w-0 space-y-3">
    <p v-if="loading" role="status" class="bi-caption">{{ t('common.loading') }}</p>
    <div v-if="error" role="alert" class="bi-error"><p>{{ error }}</p><button class="btn btn-secondary mt-3" type="button" @click="$emit('retry')">{{ t('biNative.retry') }}</button></div>
    <div v-if="!loading && !error && !rows.length" class="bi-empty"><p>{{ emptyText || t('biNative.noData') }}</p><slot name="empty-actions" /></div>
    <template v-if="rows.length">
      <div class="bi-table-meta"><span class="bi-caption">{{ t('biNative.loadedRecords', { count: rows.length }) }}</span><span class="bi-caption">{{ t('biNative.tableHint') }}</span></div>
      <div class="bi-table-scroll" tabindex="0" :aria-label="t('biNative.tableHint')">
        <table class="bi-table">
          <thead><tr><th v-for="column in columns" :key="column" scope="col">{{ t(`biNative.fields.${column}`) }}</th><th v-if="details.length" scope="col">{{ t('biNative.recordDetails') }}</th><th v-if="$slots.actions" scope="col" class="bi-cell-actions">{{ t('common.actions') }}</th></tr></thead>
          <tbody><tr v-for="(row, index) in rows" :key="String(row.id ?? row.task_id ?? index)">
            <td v-for="column in columns" :key="column" :class="cellClass(column)">
              <slot :name="`cell-${column}`" :row="row" :value="row[column]">
                <span v-if="biEmpty(row[column])" class="bi-placeholder">-</span>
                <span v-else-if="typeof row[column] === 'boolean'" class="bi-badge" :data-tone="biStatusTone(row[column] as boolean)">{{ t(row[column] ? 'common.enabled' : 'biNative.notEnabled') }}</span>
                <span v-else-if="column === 'status'" class="bi-badge" :data-tone="biStatusTone(String(row[column]))">{{ biStatusKey(String(row[column])) ? t(biStatusKey(String(row[column]))!) : biText(row[column]) }}</span>
                <time v-else-if="isDate(column)" :datetime="String(row[column])" :title="String(row[column])">{{ biDate(row[column]) }}</time>
                <span v-else-if="column === 'kind' && ['day', 'week', 'month'].includes(String(row[column]))">{{ t(`biNative.${row[column]}`) }}</span>
                <span v-else-if="isNumber(column)" :title="String(row[column])">{{ biNumber(row[column] as string | number) }}</span>
                <bdi v-else>{{ biText(row[column]) }}</bdi>
              </slot>
            </td>
            <td v-if="details.length" class="bi-cell-details">
              <details class="bi-row-details"><summary>{{ t('biNative.recordDetails') }}</summary><dl><div v-for="field in details" :key="field"><dt>{{ t(`biNative.fields.${field}`) }}</dt><dd><bdi>{{ biText(row[field]) }}</bdi></dd></div></dl></details>
            </td>
            <td v-if="$slots.actions" class="bi-cell-actions"><div class="bi-row-actions"><slot name="actions" :row="row" /></div></td>
          </tr></tbody>
        </table>
      </div>
      <p v-if="columns.some(isDate)" class="bi-caption">{{ t('biNative.timeZoneHint') }}</p>
    </template>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { BIOperationRow } from '@/api/admin/bi'
import { biDate, biEmpty, biNumber, biStatusKey, biStatusTone, biText } from './display'
import '@/styles/biOperations.css'
withDefaults(defineProps<{ rows: BIOperationRow[]; columns: string[]; details?: string[]; loading: boolean; error: string; emptyText?: string }>(), { details: () => [] })
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const isDate = (column: string) => column.endsWith('_at')
const isNumber = (column: string) => ['total_tokens', 'actual_cost', 'revision', 'users', 'authorization_version', 'available_sources', 'missing_sources', 'round', 'chunk', 'attempt'].includes(column)
function cellClass(column: string) {
  if (isDate(column)) return 'bi-cell-date'
  if (column.endsWith('_id')) return 'bi-cell-id'
  if (column === 'name' || column === 'display_name' || column === 'organization_name') return 'bi-cell-name'
  if (isNumber(column)) return 'bi-cell-number'
  return 'bi-cell-text'
}
</script>
