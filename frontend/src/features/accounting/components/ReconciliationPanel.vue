<template>
  <section class="space-y-4" data-testid="accounting-reconciliation">
    <div class="accounting-toolbar">
      <div class="flex flex-wrap gap-1.5" role="group" :aria-label="t('accounting.status')">
        <button v-for="state in states" :key="state" type="button" class="accounting-chip" :aria-pressed="status === state" @click="status = state">
          {{ t(`accounting.statuses.${state}`) }}
        </button>
      </div>
      <p class="text-sm text-gray-500 dark:text-dark-300">{{ t('accounting.reconciliation.hint') }}</p>
    </div>
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <div class="card overflow-x-auto">
      <table class="accounting-table">
        <thead>
          <tr>
            <th>{{ t('accounting.record') }}</th>
            <th class="text-right">{{ t('accounting.amount') }}</th>
            <th>{{ t('accounting.status') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="`${row.object_kind}-${row.record.id}`">
            <td>
              <router-link class="accounting-link" :to="recordRoute(organizationId, row.object_kind, row.record.id)">
                {{ recordLabel(row.record.details, row.record.id) }}
              </router-link>
              <p class="text-xs text-gray-500">{{ t(`accounting.kinds.${row.object_kind}`) }} · {{ formatDate(row.record.occurred_at) }}</p>
            </td>
            <td class="text-right tabular-nums">{{ formatMoney(effectiveAmount(row.record), row.record.currency) }}</td>
            <td>
              <span class="badge" :class="statusTone(row.status)">{{ t(`accounting.statuses.${row.status}`) }}</span>
              <p v-if="row.reason" class="mt-1 max-w-xs break-words text-xs text-gray-500">{{ row.reason }}</p>
            </td>
            <td class="whitespace-nowrap text-right">
              <router-link class="btn btn-secondary btn-sm" :to="recordActionRoute(organizationId, row.object_kind, row.record.id, 'match')">{{ t('accounting.actions.match') }}</router-link>
              <router-link class="btn btn-ghost btn-sm" :to="recordActionRoute(organizationId, row.object_kind, row.record.id, 'exclude')">{{ t('accounting.actions.exclude') }}</router-link>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!rows.length && !loading" class="p-6 text-center text-sm text-gray-500">{{ t('accounting.reconciliation.empty') }}</p>
    </div>
    <Pagination v-if="total > pageSize" v-model:page="page" :total="total" :page-size="pageSize" :show-page-size-selector="false" />
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import * as api from '../api'
import { statusTone } from '../actions'
import { formatMoney } from '../format'
import { effectiveAmount, recordLabel } from '../schema'
import { errorMessage, formatDate, recordActionRoute, recordRoute } from '../shared'
import type { ReconciliationRow } from '../types'

const props = defineProps<{ organizationId: string }>()
const { t, te } = useI18n()
const states = ['UNMATCHED', 'DIFFERENCE', 'MATCHED', 'EXCLUDED']
const pageSize = 20
const status = ref('UNMATCHED')
const page = ref(1)
const total = ref(0)
const rows = ref<ReconciliationRow[]>([])
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await api.reconciliation({ organization_id: props.organizationId, status: status.value, page: page.value, page_size: pageSize })
    rows.value = result.items
    total.value = result.total
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}

watch(status, () => {
  if (page.value === 1) void load()
  else page.value = 1
})
watch(page, load, { immediate: true })
</script>
