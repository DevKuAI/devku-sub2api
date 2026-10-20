<template>
  <section class="space-y-3" data-testid="record-list">
    <div class="accounting-toolbar">
      <div class="accounting-filters">
        <select v-if="states.length > 1" v-model="filters.status" class="input" :aria-label="t('accounting.status')">
          <option value="">{{ t('accounting.list.allStatuses') }}</option>
          <option v-for="state in states" :key="state" :value="state">{{ t(`accounting.statuses.${state}`) }}</option>
        </select>
        <select v-if="currencies.length > 1" v-model="filters.currency" class="input" :aria-label="t('accounting.currency')">
          <option value="">{{ t('accounting.list.allCurrencies') }}</option>
          <option v-for="code in currencies" :key="code" :value="code">{{ code }}</option>
        </select>
        <div class="accounting-date-range" role="group" :aria-label="t('accounting.list.dateRange')">
          <span class="text-gray-400">{{ t('accounting.list.dateRange') }}</span>
          <input v-model="filters.from" type="date" :max="filters.to || undefined" :aria-label="t('accounting.list.from')" />
          <span aria-hidden="true" class="text-gray-400">–</span>
          <input v-model="filters.to" type="date" :min="filters.from || undefined" :aria-label="t('accounting.list.to')" />
          <button v-if="filters.from || filters.to" type="button" class="accounting-date-clear" :aria-label="t('accounting.list.clearDates')" @click="clearDates">
            <Icon name="x" size="xs" aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>
    <div v-if="error" role="alert" class="accounting-error flex flex-wrap items-center justify-between gap-3">
      <p>{{ error }}</p>
      <button v-if="loadFailed" type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('common.refresh') }}</button>
    </div>
    <DataTable
      v-if="!loadFailed"
      :columns="columns"
      :data="rows"
      :loading="loading"
      row-key="id"
      clickable-rows
      class="card"
      @row-click="emit('open', $event)"
    >
      <template #cell-record="{ row }">
        <div class="max-w-sm">
          <button type="button" class="accounting-entry-link" @click.stop="emit('open', row)">{{ customerLabel(kind, row) }}</button>
          <p v-if="subtitle(row)" class="mt-0.5 break-words text-xs text-gray-500">{{ subtitle(row) }}</p>
        </div>
      </template>
      <template #cell-occurred_at="{ row }">{{ formatDate(row.occurred_at) }}</template>
      <template #cell-amount="{ row }"><span class="tabular-nums">{{ formatMoney(displayAmount(kind, row), row.currency) }}</span></template>
      <template #cell-remaining="{ row }"><span class="tabular-nums">{{ formatMoney(String(row.details[remaining!] ?? '0'), row.currency) }}</span></template>
      <template #cell-status="{ row }"><span class="badge" :class="statusTone(row.status)">{{ t(`accounting.statuses.${row.status}`) }}</span></template>
      <template #empty>
        <p class="py-6 text-center text-sm text-gray-500">{{ t(hasFilters ? 'accounting.list.noMatches' : 'accounting.list.empty') }}</p>
      </template>
    </DataTable>
    <Pagination v-if="!loadFailed && total > pageSize" v-model:page="page" :total="total" :page-size="pageSize" :show-page-size-selector="false" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import { Icon } from '@/components/icons'
import * as api from '../api'
import { statusTone } from '../actions'
import { formatMoney } from '../format'
import { customerLabel, customerStatusOptions, displayAmount, recordSubtitle, schemas } from '../schema'
import { errorMessage, formatDate, isCanceled } from '../shared'
import type { AccountingFilter, AccountingKind, AccountingRecord } from '../types'

// The organization's own records of one kind, read-only; rows open in the workspace's record view.
const props = withDefaults(defineProps<{ kind: AccountingKind; currencies?: string[] }>(), { currencies: () => [] })
const emit = defineEmits<{ open: [AccountingRecord] }>()
const { t, te } = useI18n()
const pageSize = 20
const filters = reactive({ status: '', currency: '', from: '', to: '' })
const page = ref(1)
const total = ref(0)
const rows = ref<AccountingRecord[]>([])
const loading = ref(false)
const error = ref('')
const loadFailed = ref(false)
let abort: AbortController | null = null

const remaining = computed(() => schemas[props.kind].remaining)
const states = computed(() => customerStatusOptions[props.kind] || [])
const hasFilters = computed(() => Object.values(filters).some(Boolean))
const columns = computed(() => [
  { key: 'record', label: t(`accounting.kinds.${props.kind}`) },
  { key: 'occurred_at', label: t('accounting.occurredAt') },
  { key: 'amount', label: t(props.kind === 'vcc-cards' ? 'accounting.fields.available_balance' : 'accounting.amount'), class: 'text-right' },
  ...(remaining.value && props.kind !== 'vcc-cards' ? [{ key: 'remaining', label: t(`accounting.fields.${remaining.value}`), class: 'text-right' }] : []),
  { key: 'status', label: t('accounting.status') },
])

function clearDates() {
  filters.from = ''
  filters.to = ''
}
function subtitle(row: AccountingRecord) {
  return recordSubtitle(props.kind, row).join(' · ')
}
function query(): AccountingFilter {
  const dayAfter = (date: string) => new Date(new Date(`${date}T00:00`).getTime() + 86400000).toISOString()
  return {
    status: filters.status || undefined,
    currency: filters.currency || undefined,
    from: filters.from ? new Date(`${filters.from}T00:00`).toISOString() : undefined,
    to: filters.to ? dayAfter(filters.to) : undefined,
    page: page.value,
    page_size: pageSize,
  }
}
async function load() {
  abort?.abort()
  const controller = new AbortController()
  abort = controller
  loading.value = true
  loadFailed.value = false
  error.value = ''
  try {
    const result = await api.listRecords(props.kind, true, query(), controller.signal)
    if (controller.signal.aborted) return
    rows.value = result.items
    total.value = result.total
  } catch (cause) {
    if (!controller.signal.aborted && !isCanceled(cause)) {
      loadFailed.value = true
      error.value = errorMessage(cause, t, te)
    }
  } finally {
    if (abort === controller) loading.value = false
  }
}
watch(
  () => [filters.status, filters.currency, filters.from, filters.to],
  () => {
    page.value = 1
    void load()
  },
)
watch(page, load)
watch(
  () => props.kind,
  () => {
    filters.status = ''
    page.value = 1
    void load()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  abort?.abort()
})
defineExpose({ reload: load })
</script>
