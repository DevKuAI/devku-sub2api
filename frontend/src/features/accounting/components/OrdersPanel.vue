<template>
  <section class="space-y-3" data-testid="accounting-orders">
    <div v-if="!compact" class="flex flex-wrap gap-1.5" role="group" :aria-label="t('accounting.status')">
      <button v-for="option in states" :key="option" type="button" class="accounting-chip" :aria-pressed="state === option" @click="state = option">
        {{ t(option ? `accounting.orders.states.${option}` : 'accounting.orders.allStates') }}
      </button>
    </div>
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <div v-if="rows.length" class="overflow-x-auto" :class="{ card: !compact }">
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
          <tr v-for="order in rows" :key="order.id" class="accounting-clickable-row" @click="open(order)">
            <td>
              <router-link class="accounting-entry-link" :to="orderRoute(organizationId, order.id)" @click.stop>{{ order.name }}</router-link>
              <p class="mt-0.5 text-xs text-gray-500">{{ formatDate(order.created_at) }}</p>
            </td>
            <td>
              <ol class="accounting-steps" :aria-label="t('accounting.orders.progressOf', { done: doneSteps(order), total: steps.length })">
                <li v-for="step in steps" :key="step" :data-step="step" :data-done="order.progress[step]" :title="t(`accounting.orders.steps.${step}`)">
                  {{ t(`accounting.orders.steps.${step}`) }}
                </li>
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
    <p v-else-if="!loading" class="text-sm text-gray-500" :class="{ 'card p-6 text-center': !compact }">
      {{ t(compact ? 'accounting.orders.noneOpen' : state ? 'accounting.orders.noMatches' : 'accounting.orders.none') }}
    </p>
    <router-link v-if="compact && total > rows.length" class="accounting-link text-sm" :to="workspaceRoute(organizationId, 'orders')">
      {{ t('accounting.orders.viewAll', { total }) }}
    </router-link>
    <Pagination v-if="!compact && total > pageSize" v-model:page="page" :total="total" :page-size="pageSize" :show-page-size-selector="false" />
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import * as api from '../api'
import { formatMoney } from '../format'
import { errorMessage, formatDate, orderRoute, workspaceRoute } from '../shared'
import type { AccountingOrder, OrderStep } from '../types'

// compact shows only the open orders, for the overview.
const props = withDefaults(defineProps<{ organizationId: string; compact?: boolean }>(), { compact: false })
const { t, te } = useI18n()
const router = useRouter()
const steps: OrderStep[] = ['receipt', 'invoice', 'purchase', 'ai_invoice']
const states = ['', 'OPEN', 'COMPLETED', 'CLOSED']
const pageSize = 20
const state = ref(props.compact ? 'OPEN' : '')
const page = ref(1)
const total = ref(0)
const rows = ref<AccountingOrder[]>([])
const loading = ref(true)
const error = ref('')

// Only the currencies an order actually has, e.g. income in CNY and card spend in USD.
function amounts(order: AccountingOrder, metric: 'received' | 'spent') {
  return order.totals.filter((total) => Number(total[metric]) !== 0).map((total) => formatMoney(total[metric], total.currency))
}
function doneSteps(order: AccountingOrder) {
  return steps.filter((step) => order.progress[step]).length
}
function open(order: AccountingOrder) {
  void router.push(orderRoute(props.organizationId, order.id))
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
