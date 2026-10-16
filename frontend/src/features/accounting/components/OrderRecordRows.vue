<template>
  <ul v-if="records.length" class="divide-y divide-gray-100 dark:divide-dark-700">
    <li v-for="record in records" :key="record.id">
      <router-link class="flex flex-wrap items-center justify-between gap-2 py-2.5 text-sm hover:text-primary-600" :to="recordRoute(organizationId, kind, record.id)">
        <span class="min-w-0">
          <span class="font-medium">{{ recordLabel(record.details, record.id) }}</span>
          <span v-if="subtitle(record)" class="text-gray-500"> · {{ subtitle(record) }}</span>
          <span class="text-gray-500"> · {{ formatDate(record.occurred_at) }}</span>
        </span>
        <span class="flex items-center gap-2">
          <span v-if="kind === 'vendor-invoices' && Number(record.details.unallocated_cost) > 0" class="text-xs text-amber-700 dark:text-amber-300">
            {{ t('accounting.fields.unallocated_cost') }} {{ formatMoney(String(record.details.unallocated_cost), record.currency) }}
          </span>
          <span class="tabular-nums">{{ formatMoney(effectiveAmount(record), record.currency) }}</span>
          <span class="badge" :class="statusTone(record.status)">{{ t(`accounting.statuses.${record.status}`) }}</span>
        </span>
      </router-link>
    </li>
  </ul>
  <p v-else class="text-sm text-gray-500">{{ t('accounting.orders.empty') }}</p>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { statusTone } from '../actions'
import { formatMoney } from '../format'
import { effectiveAmount, recordLabel, recordSubtitle } from '../schema'
import { formatDate, recordRoute } from '../shared'
import type { AccountingKind, AccountingRecord } from '../types'

const props = defineProps<{ kind: AccountingKind; records: AccountingRecord[]; organizationId: string }>()
const { t, te } = useI18n()

function subtitle(record: AccountingRecord) {
  const parts = recordSubtitle(props.kind, record)
  const type = record.details.transaction_type
  if (props.kind === 'vcc-transactions' && type && te(`accounting.options.transaction_type.${type}`)) parts.unshift(t(`accounting.options.transaction_type.${type}`))
  return parts.join(' · ')
}
</script>
