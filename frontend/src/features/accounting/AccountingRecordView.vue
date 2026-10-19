<template>
  <AppLayout>
    <main class="accounting-page mx-auto w-full max-w-5xl space-y-5 pb-10" data-testid="accounting-record-page">
      <router-link class="accounting-back" :to="workspaceRoute(organizationId, kind)">
        <Icon name="arrowLeft" size="sm" aria-hidden="true" />
        {{ organization?.name || t('accounting.title') }} · {{ t(`accounting.kinds.${kind}`) }}
      </router-link>

      <p v-if="loading && !record" role="status" class="text-sm text-gray-500">{{ t('accounting.loading') }}</p>
      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="route.query.upload_failed" role="status" class="accounting-warning">{{ t('accounting.files.uploadFailed') }}</p>

      <template v-if="record">
        <header class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="break-words text-2xl font-semibold text-gray-950 dark:text-white">{{ recordLabel(record.details, record.id) }}</h1>
              <span class="badge" :class="statusTone(record.status)">{{ t(`accounting.statuses.${record.status}`) }}</span>
            </div>
            <p class="mt-1 text-sm text-gray-500 [overflow-wrap:anywhere] dark:text-dark-300">
              {{ [subtitle, formatDateTime(record.occurred_at)].filter(Boolean).join(' · ') }}
            </p>
            <p v-if="record.details.order_id" class="mt-1 text-sm">
              {{ t('accounting.orders.belongsTo') }}
              <router-link class="accounting-link" :to="orderRoute(organizationId, String(record.details.order_id))" data-testid="record-order">
                {{ record.details.order_name }}
              </router-link>
            </p>
          </div>
          <div class="text-right">
            <p class="text-xs text-gray-500">{{ t(kind === 'vcc-cards' ? 'accounting.fields.available_balance' : 'accounting.amount') }}</p>
            <p class="text-2xl font-semibold tabular-nums" data-testid="record-amount">{{ formatMoney(displayAmount(kind, record), record.currency) }}</p>
          </div>
        </header>

        <div v-if="actions.length" class="flex flex-wrap gap-2" data-testid="record-actions">
          <button
            v-for="action in actions"
            :key="action.key"
            type="button"
            class="btn"
            :class="action.tone === 'primary' ? 'btn-primary' : action.tone === 'danger' ? 'btn-ghost text-red-600 dark:text-red-400' : 'btn-secondary'"
            :disabled="busy"
            :data-action="action.key"
            @click="run(action)"
          >
            {{ t(`accounting.actions.${action.key}`) }}
          </button>
        </div>

        <section class="card p-5">
          <RecordFields :kind="kind" :record="record" />
          <p v-if="record.details.has_difference" class="accounting-warning mt-4">{{ t('accounting.overdraft') }}</p>
        </section>

        <section v-if="allocations.length" class="card space-y-3 overflow-x-auto p-5">
          <h2 class="accounting-section-title">{{ t('accounting.detail.allocations') }}</h2>
          <table class="accounting-table">
            <thead>
              <tr>
                <th>{{ t(kind === 'transfers' ? 'accounting.kinds.vcc-cards' : 'accounting.detail.fundingSource') }}</th>
                <th class="text-right">{{ t('accounting.detail.allocated') }}</th>
                <th class="text-right">{{ t('accounting.detail.used') }}</th>
                <th class="text-right">{{ t('accounting.detail.released') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in allocations" :key="row.id">
                <td>
                  <router-link
                    class="accounting-link"
                    :to="recordRoute(organizationId, kind === 'transfers' ? 'vcc-cards' : 'transfers', kind === 'transfers' ? row.card_id : row.transfer_id)"
                  >
                    {{ kind === 'vcc-cards' && row.funding_type === 'TOPUP' ? t('accounting.detail.topUp') : (kind === 'transfers' ? row.card_name : row.transfer_name) || t('accounting.detail.open') }}
                  </router-link>
                </td>
                <td class="text-right tabular-nums">{{ formatMoney(row.amount, record.currency) }}</td>
                <td class="text-right tabular-nums">{{ formatMoney(row.used_amount, record.currency) }}</td>
                <td class="text-right tabular-nums">{{ formatMoney(row.released_amount, record.currency) }}</td>
              </tr>
            </tbody>
          </table>
        </section>

        <section v-if="settlements.length" class="card space-y-3 p-5">
          <h2 class="accounting-section-title">{{ t('accounting.detail.settlements') }}</h2>
          <ul class="divide-y divide-gray-100 dark:divide-dark-700">
            <li v-for="row in settlements" :key="row.id" class="flex flex-wrap items-center justify-between gap-2 py-2.5 text-sm">
              <span>
                {{ row.reference }} · <span class="tabular-nums">{{ formatMoney(row.amount, row.currency) }}</span>
                · {{ t(`accounting.statuses.${row.status}`) }}
              </span>
              <router-link
                v-if="row.status === 'ACTIVE'"
                class="btn btn-ghost btn-sm text-red-600 dark:text-red-400"
                :to="workflowRoute(organizationId, 'void-settlement', row.id, { return_to: route.fullPath })"
              >
                {{ t('accounting.actions.voidSettlement') }}
              </router-link>
            </li>
          </ul>
        </section>

        <section v-if="related.length" class="card space-y-3 p-5">
          <h2 class="accounting-section-title">{{ t('accounting.detail.related') }}</h2>
          <ul class="divide-y divide-gray-100 dark:divide-dark-700">
            <li v-for="row in related" :key="`${row.kind}-${row.id}`">
              <router-link class="flex flex-wrap items-center justify-between gap-2 py-2.5 text-sm hover:text-primary-600" :to="recordRoute(organizationId, row.kind, row.id)">
                <span class="min-w-0">
                  <span class="text-gray-500">{{ t(`accounting.kinds.${row.kind}`) }}</span> · {{ row.name }}
                </span>
                <span class="tabular-nums">
                  <template v-if="row.kind !== 'vcc-cards'">{{ formatMoney(row.amount, row.currency) }} · </template>{{ t(`accounting.statuses.${row.status}`) }}
                </span>
              </router-link>
            </li>
          </ul>
        </section>

        <section class="card p-5">
          <RecordFiles :key="record.id" :kind="kind" :record="record" />
        </section>

        <section class="card space-y-3 p-5">
          <h2 class="accounting-section-title">{{ t('accounting.detail.timeline') }}</h2>
          <ol class="space-y-3">
            <li v-for="row in activity" :key="row.id" class="border-s-2 border-gray-200 ps-4 dark:border-dark-600" :title="row.request_id">
              <p class="text-sm font-medium">
                {{ te(`accounting.activity.${row.action}`) ? t(`accounting.activity.${row.action}`) : row.action }}
                <span class="font-normal text-gray-500">· {{ formatDateTime(row.created_at) }} · #{{ row.actor_id }}</span>
              </p>
              <p v-if="activityText(row)" class="mt-1 break-words text-sm text-gray-600 dark:text-dark-300">{{ activityText(row) }}</p>
            </li>
          </ol>
          <p v-if="!activity.length" class="text-sm text-gray-500">{{ t('accounting.detail.noActivity') }}</p>
        </section>
      </template>

      <ConfirmDialog
        :show="Boolean(pending)"
        :title="pending ? t(`accounting.actions.${pending.key}`) : ''"
        :message="pending ? t(`accounting.confirmations.${pending.key}`) : ''"
        :confirm-text="pending ? t(`accounting.actions.${pending.key}`) : ''"
        :danger="pending?.tone === 'danger'"
        :loading="busy"
        @confirm="confirmPending"
        @cancel="pending = null"
      />
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { Icon } from '@/components/icons'
import { useAuthStore } from '@/stores/auth'
import RecordFields from './components/RecordFields.vue'
import RecordFiles from './components/RecordFiles.vue'
import * as api from './api'
import { confirmPath, recordActions, statusTone, type RecordAction } from './actions'
import { formatMoney } from './format'
import { displayAmount, isRecordKind, recordLabel, recordSubtitle } from './schema'
import {
  createRoute,
  editRoute,
  errorMessage,
  formatDateTime,
  orderRoute,
  recordActionRoute,
  recordRoute,
  workflowRoute,
  workspaceRoute,
} from './shared'
import { useOrganization } from './useOrganization'
import type { AccountingKind, AccountingRecord, ActivityRow, AllocationRow, RelatedRow, SettlementRow } from './types'
import '@/styles/accounting.css'

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const organizationId = computed(() => String(route.params.organizationId))
const kind = computed(() => String(route.params.kind) as AccountingKind)
const recordId = computed(() => String(route.params.id))
const { organization, loadOrganization } = useOrganization(() => organizationId.value)

const record = ref<AccountingRecord | null>(null)
const related = ref<RelatedRow[]>([])
const activity = ref<ActivityRow[]>([])
const allocations = ref<AllocationRow[]>([])
const settlements = ref<SettlementRow[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const pending = ref<RecordAction | null>(null)
const requestKey = api.stableRequest()

const actions = computed(() => (record.value ? recordActions(kind.value, record.value, auth.isAdmin) : []))
const subtitle = computed(() => (record.value ? recordSubtitle(kind.value, record.value).join(' · ') : ''))

function activityText(row: ActivityRow) {
  const summary = (row.summary || {}) as Record<string, unknown>
  const before = summary.before as Record<string, unknown> | undefined
  const after = summary.after as Record<string, unknown> | undefined
  const parts: string[] = []
  if (before?.status && after?.status && before.status !== after.status)
    parts.push(`${t(`accounting.statuses.${before.status}`)} → ${t(`accounting.statuses.${after.status}`)}`)
  const beforeAmount = before?.effective_amount || before?.amount
  const afterAmount = after?.effective_amount || after?.amount
  if (beforeAmount && afterAmount && beforeAmount !== afterAmount)
    parts.push(`${beforeAmount} → ${afterAmount} ${after?.currency || before?.currency || ''}`)
  if (summary.reason) parts.push(String(summary.reason))
  if (summary.reference) parts.push(String(summary.reference))
  return parts.join(' · ')
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    if (!isRecordKind(kind.value)) throw new Error(t('accounting.errors.referenceUnavailable'))
    const [current] = await Promise.all([api.getRecord(kind.value, recordId.value), organization.value ? null : loadOrganization()])
    if (current.organization_id !== organizationId.value) throw new Error(t('accounting.errors.organizationUnavailable'))
    record.value = current
    const path = `${kind.value}/${current.id}`
    const [links, events, history, receipts] = await Promise.all([
      api.workflow<RelatedRow[]>(`related/${path}`, organizationId.value),
      api.workflow<ActivityRow[]>(`activity/${path}`, organizationId.value),
      ['transfers', 'vcc-cards'].includes(kind.value) ? api.allocationHistory(kind.value, current.id) : [],
      kind.value === 'receivables' ? api.workflow<SettlementRow[]>(`receivables/${current.id}/settlements`, organizationId.value) : [],
    ])
    related.value = links.filter((row) => isRecordKind(row.kind))
    // An upload is logged as intent, upload and complete; only the completed upload matters to readers.
    activity.value = events.filter((row) => !['intent', 'upload'].includes(row.action))
    allocations.value = history
    settlements.value = receipts
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}

async function mutate(path: string, payload: unknown, method: 'post' | 'patch' = 'post') {
  busy.value = true
  error.value = ''
  try {
    await api.mutate(path, payload, requestKey(path, payload), method)
    requestKey.reset()
    await load()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}

function run(action: RecordAction) {
  const current = record.value!
  const back = { return_to: route.fullPath }
  if (action.mode === 'confirm') pending.value = action
  else if (action.mode === 'edit') void router.push(editRoute(organizationId.value, kind.value, current.id))
  else if (action.mode === 'create') void router.push(createRoute(organizationId.value, action.createKind!, { parent_id: current.id, currency: current.currency, ...back }))
  else if (action.mode === 'share') void mutate(`${kind.value}/${current.id}/sharing`, { customer_visible: !current.customer_visible }, 'patch')
  else void router.push(recordActionRoute(organizationId.value, kind.value, current.id, action.key))
}

async function confirmPending() {
  const action = pending.value
  if (!action || !record.value) return
  await mutate(confirmPath(kind.value, record.value.id, action.key), {})
  pending.value = null
}

// Related links reuse this page; reload only when the shown record changes.
watch(
  () => [route.name, route.params.kind, route.params.id],
  ([name], [previousName]) => {
    if (name === 'AdminAccountingRecord' && previousName === 'AdminAccountingRecord') void load()
  },
)
onMounted(load)
</script>
