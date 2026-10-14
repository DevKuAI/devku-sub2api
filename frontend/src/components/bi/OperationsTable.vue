<template>
  <div :aria-busy="loading" class="min-w-0 space-y-3">
    <p v-if="loading" role="status" class="bi-caption">{{ t('common.loading') }}</p>
    <div v-if="error" role="alert" class="bi-error"><p>{{ error }}</p><button class="btn btn-secondary mt-3" type="button" @click="$emit('retry')">{{ t('biNative.retry') }}</button></div>
    <p v-if="!loading && !error && !rows.length" class="bi-empty">{{ t('biNative.noData') }}</p>
    <template v-if="rows.length">
      <p class="bi-caption">{{ t('biNative.tableHint') }}</p>
      <div class="bi-table-scroll" tabindex="0" :aria-label="t('biNative.tableHint')">
        <table class="bi-table">
          <thead><tr><th v-for="column in columns" :key="column" scope="col">{{ t(`biNative.fields.${column}`) }}</th><th v-if="$slots.actions" scope="col" class="bi-cell-actions">{{ t('common.actions') }}</th></tr></thead>
          <tbody><tr v-for="(row, index) in rows" :key="String(row.id ?? row.task_id ?? index)">
            <td v-for="column in columns" :key="column" :class="cellClass(column)">
              <span v-if="typeof row[column] === 'boolean'" class="bi-badge" :data-tone="biStatusTone(row[column] as boolean)">{{ t(row[column] ? 'common.enabled' : 'biNative.notEnabled') }}</span>
              <span v-else-if="column === 'status'" class="bi-badge" :data-tone="biStatusTone(String(row[column]))">{{ biStatusKey(String(row[column])) ? t(biStatusKey(String(row[column]))!) : row[column] || '—' }}</span>
              <time v-else-if="isDate(column)" :datetime="String(row[column] || '')" :title="String(row[column] || '')">{{ biDate(row[column]) }}</time>
              <span v-else-if="column === 'kind' && ['day', 'week', 'month'].includes(String(row[column]))">{{ t(`biNative.${row[column]}`) }}</span>
              <span v-else-if="column === 'total_tokens' || column === 'actual_cost'" :title="String(row[column] ?? '')">{{ row[column] == null ? '—' : biNumber(row[column] as string | number) }}</span>
              <span v-else>{{ row[column] ?? '—' }}</span>
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
import { biDate, biNumber, biStatusKey, biStatusTone } from './display'
import '@/styles/biOperations.css'
defineProps<{ rows: BIOperationRow[]; columns: string[]; loading: boolean; error: string }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const isDate = (column: string) => column.endsWith('_at')
function cellClass(column: string) {
  if (isDate(column)) return 'bi-cell-date'
  if (column.endsWith('_id')) return 'bi-cell-id'
  if (column === 'name' || column === 'display_name' || column === 'organization_name') return 'bi-cell-name'
  if (['total_tokens', 'actual_cost', 'revision', 'users'].includes(column)) return 'bi-cell-number'
  return 'bi-cell-text'
}
</script>
