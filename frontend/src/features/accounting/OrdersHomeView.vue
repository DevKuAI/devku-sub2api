<template>
  <AppLayout>
    <div class="accounting-page min-w-0 space-y-6" data-testid="accounting-orders-home">
      <AccountingNav current="AdminAccounting" />

      <div class="accounting-toolbar">
        <div class="min-w-0 flex-1 sm:max-w-72">
          <input v-model="search" type="search" class="input" :placeholder="t('accounting.home.search')" :aria-label="t('accounting.home.search')" />
        </div>
        <div class="w-full sm:w-56">
          <select v-model="filters.organization" class="input" :aria-label="t('accounting.home.organization')" data-testid="orders-organization">
            <option value="">{{ t('accounting.home.allOrganizations') }}</option>
            <option v-for="organization in organizations" :key="organization.id" :value="organization.id">{{ organization.name }}</option>
          </select>
        </div>
        <div class="flex flex-wrap gap-1.5" role="group" :aria-label="t('accounting.status')">
          <button v-for="option in states" :key="option" type="button" class="accounting-chip" :aria-pressed="filters.state === option" @click="filters.state = option">
            {{ t(option ? `accounting.orders.states.${option}` : 'accounting.orders.allStates') }}
          </button>
        </div>
        <router-link class="btn btn-primary ml-auto" :to="{ name: 'AdminAccountingOrderNew', query: filters.organization ? { organization: filters.organization } : {} }" data-testid="orders-new">
          <Icon name="plus" size="md" class="mr-1" aria-hidden="true" />{{ t('accounting.orders.create') }}
        </router-link>
      </div>

      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <div v-if="rows.length" class="card overflow-x-auto">
        <table class="accounting-table accounting-orders-table">
          <thead>
            <tr>
              <th>{{ t('accounting.orders.name') }}</th>
              <th>{{ t('accounting.orders.progress') }}</th>
              <th class="text-right">{{ t('accounting.orders.totals.received') }}</th>
              <th class="text-right">{{ t('accounting.orders.totals.spent') }}</th>
              <th>{{ t('accounting.status') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="order in rows" :key="order.id" class="accounting-clickable-row" @click="router.push(orderRoute(order.id))">
              <td>
                <router-link class="accounting-entry-link" :to="orderRoute(order.id)" @click.stop>{{ order.name }}</router-link>
                <p class="mt-0.5 text-xs text-gray-500">{{ order.organization_name }} · {{ formatDate(order.created_at) }}</p>
              </td>
              <td>
                <ol class="accounting-steps" :aria-label="t('accounting.orders.progressOf', { done: doneSteps(order), total: steps.length })">
                  <li v-for="step in steps" :key="step" :data-step="step" :data-done="order.progress[step]">{{ t(`accounting.orders.steps.${step}`) }}</li>
                </ol>
              </td>
              <td class="text-right tabular-nums" :data-label="t('accounting.orders.totals.received')">
                <p v-for="value in amounts(order, 'received')" :key="value">{{ value }}</p>
                <span v-if="!amounts(order, 'received').length" class="text-gray-400">—</span>
              </td>
              <td class="text-right tabular-nums" :data-label="t('accounting.orders.totals.spent')">
                <p v-for="value in amounts(order, 'spent')" :key="value">{{ value }}</p>
                <span v-if="!amounts(order, 'spent').length" class="text-gray-400">—</span>
              </td>
              <td><span class="badge" :class="tone(order.state)">{{ t(`accounting.orders.states.${order.state}`) }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else-if="!loading" class="card p-10 text-center">
        <p class="font-medium text-gray-900 dark:text-white">{{ t(filtered ? 'accounting.orders.noMatches' : 'accounting.home.empty') }}</p>
        <p v-if="!filtered" class="mt-2 text-sm text-gray-500">{{ t('accounting.home.emptyHint') }}</p>
      </div>
      <p v-else role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>
      <Pagination v-if="total > pageSize" v-model:page="page" :total="total" :page-size="pageSize" :show-page-size-selector="false" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { Icon } from '@/components/icons'
import AccountingNav from './components/AccountingNav.vue'
import * as api from './api'
import { formatMoney } from './format'
import { errorMessage, formatDate, isCanceled, orderRoute } from './shared'
import type { AccountingOrder, AccountingOrganization, OrderStep, OrderTotal } from './types'
import '@/styles/accounting.css'

// Every organization's orders in one list; the filters live in the address so links can point at them.
const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const steps: OrderStep[] = ['receipt', 'invoice', 'purchase', 'ai_invoice']
const states = ['', 'OPEN', 'COMPLETED', 'CLOSED']
const pageSize = 20
const query = (key: string) => (typeof route.query[key] === 'string' ? (route.query[key] as string) : '')
const filters = reactive({ organization: query('organization'), state: query('state') })
const search = ref(query('search'))
const page = ref(Number(query('page')) || 1)
const organizations = ref<AccountingOrganization[]>([])
const rows = ref<AccountingOrder[]>([])
const total = ref(0)
const loading = ref(true)
const error = ref('')
const filtered = computed(() => Boolean(filters.organization || filters.state || search.value.trim()))
let abort: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | undefined

function amounts(order: AccountingOrder, metric: keyof Omit<OrderTotal, 'currency'>) {
  return order.totals.filter((entry) => Number(entry[metric]) !== 0).map((entry) => formatMoney(entry[metric], entry.currency))
}
function doneSteps(order: AccountingOrder) {
  return steps.filter((step) => order.progress[step]).length
}
function tone(state: AccountingOrder['state']) {
  return { OPEN: 'badge-warning', COMPLETED: 'badge-success', CLOSED: 'badge-gray' }[state]
}
function syncQuery() {
  const next = Object.fromEntries(
    Object.entries({ organization: filters.organization, state: filters.state, search: search.value.trim(), page: page.value > 1 ? String(page.value) : '' }).filter(([, value]) => value),
  )
  void router.replace({ query: next })
}
async function load() {
  abort?.abort()
  const controller = new AbortController()
  abort = controller
  loading.value = true
  error.value = ''
  try {
    const result = await api.orders(
      { organization_id: filters.organization || undefined, state: filters.state || undefined, search: search.value.trim() || undefined, page: page.value, page_size: pageSize },
      controller.signal,
    )
    if (controller.signal.aborted) return
    rows.value = result.items
    total.value = result.total
  } catch (cause) {
    if (!controller.signal.aborted && !isCanceled(cause)) error.value = errorMessage(cause, t, te)
  } finally {
    if (abort === controller) loading.value = false
  }
}
function reload() {
  syncQuery()
  void load()
}

watch(
  () => [filters.organization, filters.state],
  () => {
    page.value = 1
    reload()
  },
)
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    page.value = 1
    reload()
  }, 300)
})
watch(page, reload)
onMounted(async () => {
  void load()
  try {
    organizations.value = await api.organizations({})
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
})
onBeforeUnmount(() => {
  abort?.abort()
  clearTimeout(searchTimer)
})
</script>
