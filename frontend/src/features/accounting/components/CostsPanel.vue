<template>
  <section class="space-y-4" data-testid="accounting-costs">
    <div class="accounting-toolbar">
      <p class="text-sm text-gray-500 dark:text-dark-300">{{ t('accounting.costs.hint') }}</p>
      <router-link class="btn btn-primary" :to="workflowRoute(organizationId, 'cost')">
        <Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.actions.cost') }}
      </router-link>
    </div>
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <div v-if="summary.length" class="grid gap-3 lg:grid-cols-2">
      <dl v-for="row in summary" :key="row.currency" class="card grid grid-cols-2 gap-4 p-5">
        <div v-for="field in ['invoice_total', 'allocated_cost', 'unallocated_cost', 'book_cost']" :key="field">
          <dt class="text-xs text-gray-500">{{ t(`accounting.costs.${field}`) }}</dt>
          <dd class="mt-1 font-semibold tabular-nums">{{ formatMoney(String(row[field]), row.currency) }}</dd>
        </div>
      </dl>
    </div>
    <div class="card overflow-x-auto">
      <table class="accounting-table">
        <thead>
          <tr>
            <th>{{ t('accounting.kinds.vendor-invoices') }}</th>
            <th>{{ t('accounting.costs.source') }}</th>
            <th class="text-right">{{ t('accounting.amount') }}</th>
            <th>{{ t('accounting.status') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td><router-link class="accounting-link" :to="recordRoute(organizationId, 'vendor-invoices', row.invoice_id)">{{ row.invoice_name }}</router-link></td>
            <td>
              <router-link class="accounting-link" :to="recordRoute(organizationId, row.source_kind, row.source_id)">{{ row.source_name }}</router-link>
              <p v-if="row.source_currency && row.source_currency !== row.currency" class="text-xs text-gray-500">
                {{ formatMoney(row.source_amount, row.source_currency) }} × {{ Number(row.exchange_rate) }}
              </p>
            </td>
            <td class="text-right tabular-nums">{{ formatMoney(row.amount, row.currency) }}</td>
            <td><span class="badge" :class="statusTone(row.status)">{{ t(`accounting.statuses.${row.status}`) }}</span></td>
            <td class="text-right">
              <router-link v-if="row.status === 'ACTIVE'" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" :to="workflowRoute(organizationId, 'void-cost', row.id)">
                {{ t('accounting.actions.voidCost') }}
              </router-link>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!rows.length && !loading" class="p-6 text-center text-sm text-gray-500">{{ t('accounting.costs.empty') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import * as api from '../api'
import { statusTone } from '../actions'
import { formatMoney } from '../format'
import { errorMessage, recordRoute, workflowRoute } from '../shared'
import type { AccountingKind } from '../types'

interface CostSummary {
  currency: string
  [field: string]: string
}
interface CostRow {
  id: string
  invoice_id: string
  invoice_name: string
  source_kind: AccountingKind
  source_id: string
  source_name: string
  amount: string
  currency: string
  source_amount: string
  source_currency: string
  exchange_rate: string
  status: string
}

const props = defineProps<{ organizationId: string }>()
const { t, te } = useI18n()
const summary = ref<CostSummary[]>([])
const rows = ref<CostRow[]>([])
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    const [totals, allocations] = await Promise.all([
      api.workflow<CostSummary[]>('cost-summary', props.organizationId),
      api.workflow<CostRow[]>('cost-allocations', props.organizationId),
    ])
    summary.value = totals
    rows.value = allocations
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
})
</script>
