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
        <input v-model="filters.from" type="date" class="input" :aria-label="t('accounting.list.from')" :title="t('accounting.list.from')" />
        <input v-model="filters.to" type="date" class="input" :aria-label="t('accounting.list.to')" :title="t('accounting.list.to')" />
        <input
          v-if="!customer"
          v-model="filters.search"
          type="search"
          class="input min-w-48 flex-1"
          :placeholder="t('accounting.list.search')"
          :aria-label="t('accounting.list.search')"
        />
      </div>
      <div v-if="!customer" class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="exportCSV">
          <Icon name="download" size="sm" aria-hidden="true" />{{ t('accounting.list.export') }}
        </button>
        <router-link v-if="creatableKinds.includes(kind) && !orderOnlyKinds.includes(kind)" class="btn btn-primary" :to="createRoute(organizationId, kind)">
          <Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.list.create', { kind: t(`accounting.kinds.${kind}`) }) }}
        </router-link>
      </div>
    </div>
    <p v-if="hint" class="text-sm text-gray-500 dark:text-dark-300">{{ hint }}</p>
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
      @row-click="open"
    >
      <template #cell-record="{ row }">
        <div class="max-w-sm">
          <button v-if="customer" type="button" class="accounting-entry-link" @click.stop="emit('open', row)">{{ recordLabel(row.details, row.id) }}</button>
          <router-link v-else class="accounting-entry-link" :to="recordRoute(organizationId, kind, row.id)" @click.stop>{{ recordLabel(row.details, row.id) }}</router-link>
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
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import { Icon } from '@/components/icons'
import * as api from '../api'
import { statusTone } from '../actions'
import { formatMoney } from '../format'
import { creatableKinds, customerStatusOptions, displayAmount, orderOnlyKinds, recordLabel, recordSubtitle, schemas, statusOptions } from '../schema'
import { createRoute, errorMessage, formatDate, isCanceled, recordRoute } from '../shared'
import type { AccountingFilter, AccountingKind, AccountingRecord } from '../types'

const props = withDefaults(defineProps<{ kind: AccountingKind; organizationId?: string; customer?: boolean; currencies?: string[] }>(), {
  organizationId: '',
  customer: false,
  currencies: () => [],
})
const emit = defineEmits<{ open: [AccountingRecord] }>()
const { t, te } = useI18n()
const router = useRouter()
const pageSize = 20
const filters = reactive({ status: '', currency: '', from: '', to: '', search: '' })
const page = ref(1)
const total = ref(0)
const rows = ref<AccountingRecord[]>([])
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const loadFailed = ref(false)
let abort: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | undefined

const remaining = computed(() => schemas[props.kind].remaining)
const states = computed(() => (props.customer ? customerStatusOptions[props.kind] || [] : statusOptions[props.kind]))
const hasFilters = computed(() => Object.values(filters).some(Boolean))
const hint = computed(() => (!props.customer && te(`accounting.list.hints.${props.kind}`) ? t(`accounting.list.hints.${props.kind}`) : ''))
const columns = computed(() => [
  { key: 'record', label: t(`accounting.kinds.${props.kind}`) },
  { key: 'occurred_at', label: t('accounting.occurredAt') },
  { key: 'amount', label: t(props.kind === 'vcc-cards' ? 'accounting.fields.available_balance' : 'accounting.amount'), class: 'text-right' },
  ...(remaining.value && props.kind !== 'vcc-cards' ? [{ key: 'remaining', label: t(`accounting.fields.${remaining.value}`), class: 'text-right' }] : []),
  { key: 'status', label: t('accounting.status') },
])

function subtitle(row: AccountingRecord) {
  return recordSubtitle(props.kind, row).join(' · ')
}
function query(): AccountingFilter {
  const dayAfter = (date: string) => new Date(new Date(`${date}T00:00`).getTime() + 86400000).toISOString()
  return {
    organization_id: props.customer ? undefined : props.organizationId,
    status: filters.status || undefined,
    currency: filters.currency || undefined,
    search: filters.search.trim() || undefined,
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
    const result = await api.listRecords(props.kind, props.customer, query(), controller.signal)
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
function open(row: AccountingRecord) {
  if (props.customer) emit('open', row)
  else void router.push(recordRoute(props.organizationId, props.kind, row.id))
}
async function exportCSV() {
  busy.value = true
  try {
    saveAs(await api.exportCSV(props.kind, { ...query(), page: undefined, page_size: undefined }), `accounting-${props.kind}.csv`)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}

watch(
  () => [filters.status, filters.currency, filters.from, filters.to],
  () => {
    page.value = 1
    void load()
  },
)
watch(
  () => filters.search,
  () => {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      page.value = 1
      void load()
    }, 300)
  },
)
watch(page, load)
watch(
  () => props.kind,
  () => {
    Object.assign(filters, { status: '', search: '' })
    page.value = 1
    void load()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  abort?.abort()
  clearTimeout(searchTimer)
})
defineExpose({ reload: load })
</script>
