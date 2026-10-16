<template>
  <section class="space-y-3" data-testid="accounting-orders">
    <div class="accounting-toolbar">
      <div v-if="!compact" class="flex flex-wrap gap-1.5" role="group" :aria-label="t('accounting.status')">
        <button v-for="option in states" :key="option" type="button" class="accounting-chip" :aria-pressed="state === option" @click="state = option">
          {{ t(option ? `accounting.orders.states.${option}` : 'accounting.orders.allStates') }}
        </button>
      </div>
      <router-link class="btn btn-primary" :class="{ 'btn-sm': compact }" :to="{ name: 'AdminAccountingOrderNew', params: { organizationId } }">
        <Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.create') }}
      </router-link>
    </div>
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <ul v-if="rows.length" class="divide-y divide-gray-100 dark:divide-dark-700" :class="{ card: !compact }">
      <li v-for="order in rows" :key="order.id">
        <router-link class="flex flex-wrap items-center justify-between gap-3 hover:text-primary-600" :class="compact ? 'py-2.5' : 'px-4 py-3'" :to="orderRoute(organizationId, order.id)">
          <span class="min-w-0">
            <span class="font-medium">{{ order.name }}</span>
            <span class="ms-2 badge" :class="tone(order.state)">{{ t(`accounting.orders.states.${order.state}`) }}</span>
            <span class="mt-1 block text-xs text-gray-500">
              {{ [formatDate(order.created_at), ...amounts(order)].join(' · ') }}
            </span>
          </span>
          <span class="flex flex-wrap gap-1" :aria-label="t('accounting.orders.progress')">
            <span
              v-for="step in steps"
              :key="step"
              class="rounded px-1.5 py-0.5 text-xs"
              :class="order.progress[step] ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300' : 'bg-gray-100 text-gray-400 dark:bg-dark-800'"
              :data-step="step"
            >
              {{ t(`accounting.orders.steps.${step}`) }}
            </span>
          </span>
        </router-link>
      </li>
    </ul>
    <p v-else-if="!loading" class="text-sm text-gray-500" :class="{ 'card p-6 text-center': !compact }">
      {{ t(state || compact ? 'accounting.orders.noMatches' : 'accounting.orders.none') }}
    </p>
    <Pagination v-if="!compact && total > pageSize" v-model:page="page" :total="total" :page-size="pageSize" :show-page-size-selector="false" />
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import { Icon } from '@/components/icons'
import * as api from '../api'
import { formatMoney } from '../format'
import { errorMessage, formatDate, orderRoute } from '../shared'
import type { AccountingOrder, OrderStep } from '../types'

// compact shows only the open orders, for the overview.
const props = withDefaults(defineProps<{ organizationId: string; compact?: boolean }>(), { compact: false })
const { t, te } = useI18n()
const steps: OrderStep[] = ['receipt', 'invoice', 'purchase', 'ai_invoice']
const states = ['', 'OPEN', 'COMPLETED', 'CLOSED']
const pageSize = 20
const state = ref(props.compact ? 'OPEN' : '')
const page = ref(1)
const total = ref(0)
const rows = ref<AccountingOrder[]>([])
const loading = ref(true)
const error = ref('')

// Only the amounts an order actually has, e.g. income in CNY and card spend in USD.
function amounts(order: AccountingOrder) {
  return order.totals.flatMap((total) =>
    (['received', 'spent'] as const)
      .filter((metric) => Number(total[metric]) !== 0)
      .map((metric) => `${t(`accounting.orders.totals.${metric}`)} ${formatMoney(total[metric], total.currency)}`),
  )
}
function tone(value: string) {
  return { OPEN: 'badge-warning', COMPLETED: 'badge-success', CLOSED: 'badge-gray' }[value] || 'badge-gray'
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await api.orders(props.organizationId, { state: state.value || undefined, page: page.value, page_size: props.compact ? 5 : pageSize })
    rows.value = result.items
    total.value = result.total
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}

watch(state, () => {
  if (page.value === 1) void load()
  else page.value = 1
})
watch(page, load, { immediate: true })
</script>
