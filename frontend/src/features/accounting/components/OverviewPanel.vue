<template>
  <div class="space-y-5" data-testid="accounting-overview">
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <div v-if="summaries.length" class="grid gap-3 lg:grid-cols-2">
      <section v-for="entry in summaries" :key="entry.currency" class="card p-5" :aria-label="entry.currency">
        <h2 class="text-sm font-semibold text-gray-500">{{ entry.currency }}</h2>
        <dl class="mt-3 grid grid-cols-2 gap-4">
          <div v-for="metric in metrics" :key="metric">
            <dt class="text-xs text-gray-500 dark:text-dark-300">{{ t(`accounting.metrics.${metric}`) }}</dt>
            <dd class="mt-1 break-all text-lg font-semibold tabular-nums" :class="{ 'text-amber-700 dark:text-amber-300': metric === 'balance' && Number(entry.balance) < 0 }">
              {{ formatMoney(entry[metric], entry.currency) }}
            </dd>
          </div>
        </dl>
      </section>
    </div>
    <p v-else class="card p-8 text-center text-sm text-gray-500">{{ t(customer ? 'accounting.overview.emptyCustomer' : 'accounting.overview.empty') }}</p>

    <section v-if="!customer" class="card space-y-3 p-5" data-testid="overview-orders">
      <h2 class="accounting-section-title">{{ t('accounting.orders.inProgress') }}</h2>
      <OrdersPanel :organization-id="organizationId" compact />
    </section>

    <section v-if="!customer" class="card space-y-4 p-5" data-testid="accounting-todos">
      <header class="flex flex-wrap items-center justify-between gap-3">
        <h2 class="accounting-section-title">{{ t('accounting.overview.todo') }}</h2>
        <div class="flex flex-wrap gap-2">
          <router-link v-for="quick in quickCreate" :key="quick" class="btn btn-ghost btn-sm" :to="createRoute(organizationId, quick)">
            <Icon name="plus" size="sm" aria-hidden="true" />{{ t(`accounting.kinds.${quick}`) }}
          </router-link>
        </div>
      </header>
      <div v-if="workbench && workbench.total" class="flex flex-wrap gap-1.5" role="group" :aria-label="t('accounting.overview.todoType')">
        <button type="button" class="accounting-chip" :aria-pressed="!reason" @click="reason = ''">
          {{ t('accounting.overview.allTodos') }} {{ workbench.total }}
        </button>
        <button v-for="(count, key) in workbench.counts" :key="key" type="button" class="accounting-chip" :aria-pressed="reason === key" @click="reason = String(key)">
          {{ t(`accounting.reasons.${key}`) }} {{ count }}
        </button>
      </div>
      <p v-for="pending in workbench?.pending_consumption || []" :key="pending.currency" class="accounting-warning">
        {{ t('accounting.overview.pendingUsage', { amount: formatMoney(pending.amount, pending.currency) }) }}
      </p>
      <ul v-if="workbench?.items.length" class="divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="item in workbench.items" :key="`${item.kind}-${item.id}-${item.reason}`">
          <router-link class="flex flex-wrap items-center justify-between gap-2 py-2.5 hover:text-primary-600" :to="todoRoute(item)">
            <span class="min-w-0 text-sm">
              <span class="badge badge-warning me-2">{{ t(`accounting.reasons.${item.reason}`) }}</span>
              <span class="text-gray-500">{{ t(`accounting.kinds.${item.kind}`) }}</span> · {{ item.name }}
            </span>
            <span class="text-sm tabular-nums">{{ formatMoney(item.amount, item.currency) }}</span>
          </router-link>
        </li>
      </ul>
      <p v-else-if="workbench" class="text-sm text-gray-500">{{ t('accounting.overview.noTodo') }}</p>
      <p v-if="workbench && (workbench.matched_total ?? workbench.total) > workbench.items.length" class="text-xs text-gray-500">
        {{ t('accounting.overview.todoLimit', { count: workbench.items.length, total: workbench.matched_total ?? workbench.total }) }}
      </p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import OrdersPanel from './OrdersPanel.vue'
import * as api from '../api'
import { formatMoney } from '../format'
import { createRoute, errorMessage, recordRoute, workflowRoute } from '../shared'
import type { AccountingKind, CurrencySummary, Workbench, WorkflowTodo } from '../types'

const props = withDefaults(defineProps<{ summaries: CurrencySummary[]; organizationId?: string; customer?: boolean }>(), {
  organizationId: '',
  customer: false,
})
const { t, te } = useI18n()
const workbench = ref<Workbench | null>(null)
const reason = ref('')
const error = ref('')
// Deal records start from an order; these cover money outside any order.
const quickCreate: AccountingKind[] = ['expenses', 'contracts']
const metrics = computed(() =>
  props.customer ? (['received', 'allocated', 'consumed', 'balance'] as const) : (['unallocated', 'balance', 'received', 'consumed'] as const),
)

function todoRoute(item: WorkflowTodo) {
  return item.kind === 'external-actions'
    ? workflowRoute(props.organizationId, 'external', item.id)
    : recordRoute(props.organizationId, item.kind, item.id)
}
async function loadWorkbench() {
  if (props.customer) return
  error.value = ''
  try {
    const path = reason.value ? `workbench?reason=${encodeURIComponent(reason.value)}` : 'workbench'
    workbench.value = await api.workflow<Workbench>(path, props.organizationId)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}

watch(reason, loadWorkbench)
onMounted(loadWorkbench)
</script>
