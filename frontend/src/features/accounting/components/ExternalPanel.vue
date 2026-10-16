<template>
  <section class="space-y-4" data-testid="accounting-external">
    <p class="text-sm text-gray-500 dark:text-dark-300">{{ t('accounting.external.hint') }}</p>
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <div class="card overflow-x-auto">
      <table class="accounting-table">
        <thead>
          <tr>
            <th>{{ t('accounting.kinds.vcc-cards') }}</th>
            <th>{{ t('accounting.actionForm.externalAction') }}</th>
            <th class="text-right">{{ t('accounting.amount') }}</th>
            <th>{{ t('accounting.status') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>
              <router-link class="accounting-link" :to="recordRoute(organizationId, 'vcc-cards', row.card_id)">{{ row.card_name }}</router-link>
              <span class="text-xs text-gray-500"> •••• {{ row.last_four }}</span>
            </td>
            <td>
              {{ t(`accounting.options.external_action.${row.action}`) }}
              <p v-if="row.reference || row.reason" class="text-xs text-gray-500">{{ row.reference || row.reason }}</p>
            </td>
            <td class="text-right tabular-nums">{{ formatMoney(row.amount, row.currency) }}</td>
            <td><span class="badge" :class="row.status === 'PENDING' ? 'badge-warning' : 'badge-gray'">{{ t(`accounting.external.statuses.${row.status}`) }}</span></td>
            <td class="whitespace-nowrap text-right">
              <template v-if="row.status === 'PENDING'">
                <router-link class="btn btn-primary btn-sm" :to="workflowRoute(organizationId, 'external', row.id)">{{ t('accounting.actions.external') }}</router-link>
                <router-link v-if="administrator" class="btn btn-ghost btn-sm" :to="workflowRoute(organizationId, 'external-exempt', row.id)">
                  {{ t('accounting.actions.externalExempt') }}
                </router-link>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!rows.length && !loading" class="p-6 text-center text-sm text-gray-500">{{ t('accounting.external.empty') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import * as api from '../api'
import { formatMoney } from '../format'
import { errorMessage, recordRoute, workflowRoute } from '../shared'

interface ExternalRow {
  id: string
  card_id: string
  card_name: string
  last_four: string
  action: string
  amount: string
  currency: string
  status: string
  reference: string
  reason: string
}

const props = defineProps<{ organizationId: string; administrator?: boolean }>()
const { t, te } = useI18n()
const rows = ref<ExternalRow[]>([])
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    rows.value = await api.workflow<ExternalRow[]>('external-actions', props.organizationId)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
})
</script>
