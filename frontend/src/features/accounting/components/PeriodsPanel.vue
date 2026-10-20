<template>
  <section class="space-y-4" data-testid="accounting-periods">
    <p class="text-sm text-gray-500 dark:text-dark-300">{{ t('accounting.periods.customerHint') }}</p>
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <div class="card overflow-x-auto">
      <table class="accounting-table">
        <thead>
          <tr>
            <th>{{ t('accounting.periods.month') }}</th>
            <th>{{ t('accounting.status') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td class="font-medium">{{ row.month }}<span v-if="row.revision > 1" class="ms-2 text-xs text-gray-500">{{ t('accounting.periods.revision', { revision: row.revision }) }}</span></td>
            <td><span class="badge" :class="row.status === 'CLOSED' ? 'badge-success' : 'badge-warning'">{{ t(`accounting.periods.statuses.${row.status}`) }}</span></td>
            <td class="whitespace-nowrap text-right">
              <template v-if="row.statement_id">
                <button type="button" class="btn btn-ghost btn-sm" @click="openStatement(row.statement_id)">{{ t('accounting.periods.statement') }}</button>
                <button type="button" class="btn btn-ghost btn-sm" @click="download(row)">{{ t('accounting.list.export') }}</button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!rows.length && !loading" class="p-6 text-center text-sm text-gray-500">{{ t('accounting.periods.empty') }}</p>
    </div>

    <BaseDialog :show="Boolean(statement)" :title="t('accounting.periods.statementTitle', { month: statement?.month || '' })" width="extra-wide" @close="statement = null">
      <div v-if="statement" class="space-y-4">
        <p v-if="!statement.opening" class="accounting-warning">{{ t('accounting.periods.unknownOpening') }}</p>
        <table class="accounting-table">
          <thead>
            <tr>
              <th>{{ t('accounting.currency') }}</th>
              <th />
              <th class="text-right">{{ t('accounting.periods.opening') }}</th>
              <th class="text-right">{{ t('accounting.periods.movement') }}</th>
              <th class="text-right">{{ t('accounting.periods.closing') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="(closing, currency) in statement.closing" :key="currency">
              <tr v-for="side in sides" :key="`${currency}-${side}`">
                <td>{{ side === 'cash' ? currency : '' }}</td>
                <td class="text-gray-500">{{ t(`accounting.periods.${side}`) }}</td>
                <td class="text-right tabular-nums">{{ statement.opening ? formatMoney(statement.opening[currency]?.[side] || '0', String(currency)) : t('accounting.periods.unknown') }}</td>
                <td class="text-right tabular-nums">{{ statement.opening ? formatMoney(statement.movement[currency]?.[side] || '0', String(currency)) : t('accounting.periods.unknown') }}</td>
                <td class="text-right tabular-nums">{{ formatMoney(closing[side], String(currency)) }}</td>
              </tr>
            </template>
          </tbody>
        </table>
        <table class="accounting-table">
          <thead>
            <tr>
              <th>{{ t('accounting.record') }}</th>
              <th>{{ t('accounting.occurredAt') }}</th>
              <th class="text-right">{{ t('accounting.amount') }}</th>
              <th>{{ t('accounting.status') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in statementRows" :key="`${row.kind}-${row.record.id}`">
              <td><span class="text-gray-500">{{ t(`accounting.kinds.${row.kind}`) }}</span> · {{ recordLabel(row.record.details, row.record.id) }}</td>
              <td>{{ formatDate(row.record.occurred_at) }}</td>
              <td class="text-right tabular-nums">{{ formatMoney(effectiveAmount(row.record), row.record.currency) }}</td>
              <td>{{ t(`accounting.statuses.${row.record.status}`) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import BaseDialog from '@/components/common/BaseDialog.vue'
import * as api from '../api'
import { formatMoney } from '../format'
import { customerKinds, effectiveAmount, publicFields, recordLabel } from '../schema'
import { errorMessage, formatDate } from '../shared'
import type { Statement } from '../types'

interface PeriodRow {
  id: string
  month: string
  status: string
  revision: number
  reason: string
  statement_id?: string
}

// Closed months and their statements, as the organization sees them.
const props = withDefaults(defineProps<{ organizationName?: string }>(), { organizationName: '' })
const { t, te } = useI18n()
const sides = ['cash', 'card'] as const
const rows = ref<PeriodRow[]>([])
const statement = ref<Statement | null>(null)
const loading = ref(true)
const error = ref('')

// Customers only see their shareable record kinds and public fields, even inside a statement.
const statementRows = computed(() =>
  (statement.value?.records || [])
    .filter((row) => customerKinds.includes(row.kind))
    .map((row) => ({
      ...row,
      record: { ...row.record, details: Object.fromEntries(Object.entries(row.record.details).filter(([key]) => publicFields[row.kind]?.includes(key))) },
    })),
)

async function openStatement(id: string) {
  try {
    statement.value = await api.workflow<Statement>(`statements/${id}`, '', true)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}
async function download(row: PeriodRow) {
  try {
    saveAs(await api.statementCSV(row.statement_id!, true), `${props.organizationName || 'statement'}-${row.month}.csv`)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}

onMounted(async () => {
  try {
    rows.value = await api.workflow<PeriodRow[]>('periods', '', true)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
})
</script>
