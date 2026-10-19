<template>
  <div class="accounting-page min-w-0 space-y-5" data-testid="accounting-directory">
    <header class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">{{ t('accounting.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ t('accounting.directory.description') }}</p>
      </div>
      <router-link class="btn btn-secondary" :to="{ name: 'AdminAccountingReceivingAccounts' }">
        {{ t('accounting.receivingAccounts.title') }}
      </router-link>
    </header>

    <div v-if="totals.length" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <section v-for="entry in totals" :key="entry.currency" class="card p-4">
        <p class="text-xs font-semibold text-gray-500">{{ entry.currency }}</p>
        <dl class="mt-2 grid grid-cols-2 gap-3">
          <div>
            <dt class="text-xs text-gray-500">{{ t('accounting.metrics.unallocated') }}</dt>
            <dd class="font-semibold tabular-nums">{{ formatMoney(entry.unallocated, entry.currency) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-500">{{ t('accounting.metrics.balance') }}</dt>
            <dd class="font-semibold tabular-nums">{{ formatMoney(entry.balance, entry.currency) }}</dd>
          </div>
        </dl>
      </section>
    </div>

    <input v-model="search" type="search" class="input w-full max-w-md" :placeholder="t('accounting.directory.search')" :aria-label="t('accounting.directory.search')" />
    <div v-if="error" role="alert" class="accounting-error flex flex-wrap items-center justify-between gap-3">
      <p>{{ error }}</p>
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('common.refresh') }}</button>
    </div>

    <DataTable v-else :columns="columns" :data="organizations" :loading="loading" row-key="id" clickable-rows class="card" @row-click="open">
      <template #cell-name="{ row }">
        <router-link class="accounting-entry-link" :to="workspaceRoute(row.id)" @click.stop>{{ row.name }}</router-link>
      </template>
      <template #cell-unallocated="{ row }">
        <p v-for="entry in row.summary" :key="entry.currency" class="tabular-nums">{{ formatMoney(entry.unallocated, entry.currency) }}</p>
        <span v-if="!row.summary.length" class="text-gray-400">—</span>
      </template>
      <template #cell-balance="{ row }">
        <p v-for="entry in row.summary" :key="entry.currency" class="tabular-nums" :class="{ 'text-amber-700 dark:text-amber-300': Number(entry.balance) < 0 }">
          {{ formatMoney(entry.balance, entry.currency) }}
        </p>
      </template>
      <template #cell-received="{ row }">
        <p v-for="entry in row.summary" :key="entry.currency" class="tabular-nums">{{ formatMoney(entry.received, entry.currency) }}</p>
      </template>
      <template #cell-unmatched="{ row }">
        <span v-if="unmatched(row)" class="badge badge-warning">{{ unmatched(row) }}</span>
        <span v-else class="text-gray-400">—</span>
      </template>
      <template #empty>
        <p class="py-6 text-center text-sm text-gray-500">{{ t(search ? 'accounting.directory.noMatches' : 'accounting.directory.empty') }}</p>
      </template>
    </DataTable>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import DataTable from '@/components/common/DataTable.vue'
import * as api from '../api'
import { formatMoney } from '../format'
import { errorMessage, isCanceled, workspaceRoute } from '../shared'
import type { AccountingOrganization, CurrencySummary } from '../types'

const { t, te } = useI18n()
const router = useRouter()
const organizations = ref<AccountingOrganization[]>([])
const totals = ref<CurrencySummary[]>([])
const search = ref('')
const loading = ref(true)
const error = ref('')
let abort: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined

const columns = computed(() => [
  { key: 'name', label: t('accounting.directory.organization') },
  { key: 'unallocated', label: t('accounting.metrics.unallocated'), class: 'text-right' },
  { key: 'balance', label: t('accounting.metrics.balance'), class: 'text-right' },
  { key: 'received', label: t('accounting.metrics.received'), class: 'text-right' },
  { key: 'unmatched', label: t('accounting.metrics.unmatched') },
])

function unmatched(row: AccountingOrganization) {
  return row.summary.reduce((sum, entry) => sum + Number(entry.unmatched || 0), 0)
}
function open(row: AccountingOrganization) {
  void router.push(workspaceRoute(row.id))
}
async function load() {
  abort?.abort()
  const controller = new AbortController()
  abort = controller
  loading.value = true
  error.value = ''
  try {
    const result = await api.organizations(search.value.trim() ? { search: search.value.trim() } : {}, controller.signal)
    if (!controller.signal.aborted) organizations.value = result
  } catch (cause) {
    if (!controller.signal.aborted && !isCanceled(cause)) error.value = errorMessage(cause, t, te)
  } finally {
    if (abort === controller) loading.value = false
  }
}

watch(search, () => {
  clearTimeout(timer)
  timer = setTimeout(load, 300)
})
onMounted(async () => {
  void load()
  try {
    totals.value = await api.summary(false, {})
  } catch {
    totals.value = []
  }
})
onBeforeUnmount(() => {
  abort?.abort()
  clearTimeout(timer)
})
</script>
