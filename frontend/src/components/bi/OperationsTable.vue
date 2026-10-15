<template>
  <div :aria-busy="loading" class="min-w-0 space-y-3">
    <p v-if="loading" role="status" class="bi-caption">{{ t('common.loading') }}</p>
    <div v-if="error" role="alert" class="bi-error"><p>{{ error }}</p><button class="btn btn-secondary mt-3" type="button" @click="$emit('retry')">{{ t('biNative.retry') }}</button></div>
    <div v-if="!loading && !error && !rows.length" class="bi-empty"><p>{{ emptyText || t('biNative.noData') }}</p><slot name="empty-actions" /></div>
    <template v-if="rows.length">
      <div class="bi-table-meta"><span class="bi-caption">{{ t('biNative.loadedRecords', { count: rows.length }) }}</span><span class="bi-caption">{{ t('biNative.tableHint') }}</span></div>
      <div class="bi-table-scroll" role="region" tabindex="0" :aria-label="t('biNative.tableHint')">
        <table class="bi-table">
          <thead><tr><th v-for="column in columns" :key="column" scope="col">{{ t(`biNative.fields.${column}`) }}</th><th v-if="details.length" scope="col">{{ t('biNative.recordDetails') }}</th><th v-if="$slots.actions" scope="col" class="bi-cell-actions">{{ t('common.actions') }}</th></tr></thead>
          <tbody><tr v-for="(row, index) in rows" :key="JSON.stringify([row.organization_id, row.kind, row.id ?? row.task_id ?? row.model ?? index])">
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
              <button class="btn btn-secondary" type="button" aria-haspopup="dialog" @click="selectedRow = row">{{ t('biNative.recordDetails') }}</button>
            </td>
            <td v-if="$slots.actions" class="bi-cell-actions"><div class="bi-row-actions"><slot name="actions" :row="row" /></div></td>
          </tr></tbody>
        </table>
      </div>
      <p v-if="columns.some(isDate)" class="bi-caption">{{ t('biNative.timeZoneHint') }}</p>
    </template>
    <BaseDialog v-if="details.length" :show="selectedRow !== null" :title="selectedRow?.organization_name ? t('biNative.organizationRecordDetails', { name: selectedRow.organization_name }) : t('biNative.recordDetails')" :close-label="t('common.close')" content-class="bi-record-dialog" width="wide" @close="selectedRow = null">
      <dl v-if="selectedRow" class="bi-record-details"><div v-for="field in detailFields" :key="field" :class="{ 'bi-record-details-wide': field === 'error' || field === 'reason' }"><dt>{{ t(`biNative.fields.${field}`) }}</dt><dd><bdi>{{ detailValue(field, selectedRow[field]) }}</bdi></dd></div></dl>
    </BaseDialog>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { BIOperationRow } from '@/api/admin/bi'
import { biDate, biEmpty, biNumber, biStatusKey, biStatusTone, biText } from './display'
import '@/styles/biOperations.css'
const props = withDefaults(defineProps<{ rows: BIOperationRow[]; columns: string[]; details?: string[]; loading: boolean; error: string; emptyText?: string }>(), { details: () => [] })
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const selectedRow = ref<BIOperationRow | null>(null)
const detailFields = computed(() => [...new Set([...props.columns, ...props.details])])
watch(() => props.rows, () => { selectedRow.value = null })
const isDate = (column: string) => column.endsWith('_at')
const isNumber = (column: string) => ['total_tokens', 'actual_cost', 'execution_count', 'missing_token_count', 'missing_cost_count', 'revision', 'members', 'users', 'authorization_version', 'available_sources', 'missing_sources', 'round', 'chunk', 'attempt'].includes(column)
function detailValue(field: string, value: BIOperationRow[string]) {
  if (biEmpty(value)) return '-'
  if (typeof value === 'boolean') return t(value ? 'common.enabled' : 'biNative.notEnabled')
  if (isDate(field)) return biDate(value)
  if (field === 'kind' && ['day', 'week', 'month'].includes(String(value))) return t(`biNative.${value}`)
  if (field === 'status') return biStatusKey(String(value)) ? t(biStatusKey(String(value))!) : biText(value)
  return isNumber(field) ? biNumber(value as string | number) : biText(value)
}
function cellClass(column: string) {
  if (isDate(column) || column === 'period_start' || column === 'period_end') return 'bi-cell-date'
  if (column.endsWith('_id')) return 'bi-cell-id'
  if (column === 'name' || column === 'display_name' || column === 'organization_name') return 'bi-cell-name'
  if (isNumber(column)) return 'bi-cell-number'
  return 'bi-cell-text'
}
</script>
