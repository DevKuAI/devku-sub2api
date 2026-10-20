<template>
  <AppLayout>
    <main class="accounting-page mx-auto w-full max-w-5xl space-y-5 pb-10" data-testid="accounting-order-page">
      <PageHeader :back="{ to: workspaceRoute(organizationId, 'orders'), label: organization?.name || t('accounting.sections.orders') }">
        <template #title>{{ order?.name || t('accounting.sections.orders') }}</template>
        <template v-if="order" #badge>
          <span class="badge" :class="stateTone">{{ t(`accounting.orders.states.${order.state}`) }}</span>
        </template>
        <template v-if="order" #subtitle>{{ [t('accounting.orders.createdOn', { date: formatDate(order.created_at) }), order.remark].filter(Boolean).join(' · ') }}</template>
        <template v-if="order" #actions>
          <router-link class="btn btn-secondary" :to="workflowRoute(organizationId, 'order-edit', order.id)">{{ t('accounting.actions.edit') }}</router-link>
          <router-link v-if="order.status === 'OPEN' && order.state !== 'COMPLETED'" class="btn btn-ghost" :to="workflowRoute(organizationId, 'order-close', order.id)">
            {{ t('accounting.actions.orderClose') }}
          </router-link>
          <button v-if="order.status === 'CLOSED'" type="button" class="btn btn-secondary" :disabled="busy" @click="reopening = true">
            {{ t('accounting.actions.orderReopen') }}
          </button>
        </template>
      </PageHeader>

      <p v-if="loading && !order" role="status" class="text-sm text-gray-500">{{ t('accounting.loading') }}</p>
      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="route.query.upload_failed" role="status" class="accounting-warning">{{ t('accounting.files.uploadFailed') }}</p>

      <template v-if="order">
        <section class="accounting-order-summary" data-testid="order-summary">
          <div class="min-w-0">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-300">{{ t('accounting.orders.nextStep') }}</p>
            <p class="mt-1 text-base font-semibold text-gray-950 dark:text-white" data-testid="order-next-step">
              {{ order.status === 'CLOSED' ? t('accounting.orders.closedReason', { reason: order.close_reason }) : nextStep ? t(`accounting.orders.next.${nextStep.key}`) : t('accounting.orders.allDone') }}
            </p>
          </div>
          <router-link v-if="open && nextStep" class="btn btn-primary" :to="nextStep.to" data-testid="order-next-action">
            {{ t(`accounting.orders.nextAction.${nextStep.key}`) }}
          </router-link>
          <dl v-if="totals.length" class="accounting-order-totals" data-testid="order-totals">
            <div v-for="entry in totals" :key="entry.key">
              <dt>{{ t(`accounting.orders.totals.${entry.metric}`) }}</dt>
              <dd>{{ entry.values.join(' + ') }}</dd>
            </div>
          </dl>
        </section>

        <section
          v-for="(step, index) in steps"
          :key="step"
          class="card accounting-order-step"
          :data-testid="`order-section-${stepSections[step]}`"
          :data-step="step"
          :data-done="order.progress[step]"
        >
          <header class="accounting-order-step-header">
            <div class="flex min-w-0 items-center gap-3">
              <span class="accounting-step-marker" :data-state="stepState(step)" aria-hidden="true">
                <Icon v-if="order.progress[step]" name="check" size="sm" :stroke-width="2.5" />
                <template v-else>{{ index + 1 }}</template>
              </span>
              <div class="min-w-0">
                <h2 class="accounting-section-title">{{ t(`accounting.orders.steps.${step}`) }}</h2>
                <p class="text-xs text-gray-500 dark:text-dark-300">{{ t(`accounting.orders.stepHints.${step}`) }}</p>
              </div>
            </div>
            <div v-if="open" class="flex flex-wrap gap-2">
              <template v-if="step === 'purchase'">
                <router-link class="btn btn-ghost btn-sm" :to="addRoute('vcc-cards')"><Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.add.vcc-cards') }}</router-link>
                <router-link v-if="activeCards.length" class="btn btn-secondary btn-sm" :to="addRoute('vcc-transactions')">
                  <Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.add.vcc-transactions') }}
                </router-link>
              </template>
              <template v-else>
                <router-link class="btn btn-ghost btn-sm" :to="linkRoute(stepKinds[step])">{{ t('accounting.orders.linkExisting') }}</router-link>
                <router-link class="btn btn-secondary btn-sm" :to="addRoute(stepKinds[step])">
                  <Icon name="plus" size="sm" aria-hidden="true" />{{ t(`accounting.orders.add.${stepKinds[step]}`) }}
                </router-link>
              </template>
            </div>
          </header>

          <template v-if="step === 'purchase'">
            <ul v-if="items['vcc-cards'].length" class="accounting-card-list">
              <li v-for="card in items['vcc-cards']" :key="card.id" :data-card="card.id">
                <router-link class="min-w-0 hover:text-primary-600" :to="recordRoute(organizationId, 'vcc-cards', card.id)">
                  <span class="font-medium">{{ recordLabel(card.details, card.id) }}</span>
                  <span class="text-gray-500"> •••• {{ card.details.last_four }} · {{ card.currency }}</span>
                </router-link>
                <span class="flex items-center gap-3">
                  <span class="text-sm tabular-nums" :class="{ 'font-medium text-amber-700 dark:text-amber-300': Number(card.details.available_balance) < 0 }">
                    {{ t('accounting.fields.available_balance') }} {{ formatMoney(String(card.details.available_balance || '0'), card.currency) }}
                  </span>
                  <router-link v-if="card.status === 'ACTIVE'" class="btn btn-ghost btn-sm" :to="topUpRoute(card)">{{ t('accounting.actions.topUp') }}</router-link>
                </span>
              </li>
            </ul>
            <p v-if="negativeCards" class="accounting-warning">{{ t('accounting.orders.negativeCardHint') }}</p>
            <OrderRecordRows kind="vcc-transactions" :records="items['vcc-transactions']" :organization-id="organizationId" />
          </template>
          <OrderRecordRows v-else :kind="stepKinds[step]" :records="items[stepKinds[step]]" :organization-id="organizationId" />
          <p v-if="step === 'ai_invoice' && order.progress.purchase && !order.progress.ai_invoice && items['vendor-invoices'].length" class="accounting-field-hint">
            {{ t('accounting.orders.aiInvoiceHint') }}
          </p>
        </section>
      </template>

      <ConfirmDialog
        :show="reopening"
        :title="t('accounting.actions.orderReopen')"
        :message="t('accounting.confirmations.orderReopen')"
        :confirm-text="t('accounting.actions.orderReopen')"
        :loading="busy"
        @confirm="reopen"
        @cancel="reopening = false"
      />
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { Icon } from '@/components/icons'
import OrderRecordRows from './components/OrderRecordRows.vue'
import PageHeader from './components/PageHeader.vue'
import * as api from './api'
import { formatMoney } from './format'
import { recordLabel } from './schema'
import { createRoute, errorMessage, formatDate, recordActionRoute, recordRoute, workflowRoute, workspaceRoute } from './shared'
import { useOrganization } from './useOrganization'
import type { AccountingKind, AccountingOrder, AccountingRecord, OrderItemKind, OrderStep, OrderTotal } from './types'
import '@/styles/accounting.css'

const { t, te } = useI18n()
const route = useRoute()
const organizationId = computed(() => String(route.params.organizationId))
const orderId = computed(() => String(route.params.orderId))
const { organization, loadOrganization } = useOrganization(() => organizationId.value)
const order = ref<AccountingOrder | null>(null)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const reopening = ref(false)
const requestKey = api.stableRequest()
const steps: OrderStep[] = ['receipt', 'invoice', 'purchase', 'ai_invoice']
const stepKinds: Record<OrderStep, OrderItemKind> = {
  receipt: 'transfers',
  invoice: 'invoices',
  purchase: 'vcc-transactions',
  ai_invoice: 'vendor-invoices',
}
const stepSections: Record<OrderStep, string> = { receipt: 'receipt', invoice: 'invoice', purchase: 'purchase', ai_invoice: 'ai-invoice' }
const totalMetrics: (keyof Omit<OrderTotal, 'currency'>)[] = ['received', 'invoiced', 'spent', 'ai_cost']
const itemKinds: OrderItemKind[] = ['transfers', 'invoices', 'vcc-cards', 'vcc-transactions', 'vendor-invoices']

const open = computed(() => order.value?.status === 'OPEN')
const items = computed(
  () => Object.fromEntries(itemKinds.map((kind) => [kind, order.value?.items?.[kind] || []])) as Record<OrderItemKind, AccountingRecord[]>,
)
const activeCards = computed(() => items.value['vcc-cards'].filter((card) => card.status === 'ACTIVE'))
const negativeCards = computed(() => items.value['vcc-cards'].some((card) => Number(card.details.available_balance) < 0))
// Only the amounts the order actually has, per metric and currency.
const totals = computed(() =>
  totalMetrics
    .map((metric) => ({
      key: metric,
      metric,
      values: (order.value?.totals || []).filter((total) => Number(total[metric]) !== 0).map((total) => formatMoney(total[metric], total.currency)),
    }))
    .filter((entry) => entry.values.length),
)
// The first thing still missing, with a direct link to finish it.
const nextStep = computed(() => {
  const current = order.value
  if (!current) return null
  const draft = (kind: OrderItemKind) => items.value[kind].find((record) => record.status === 'DRAFT')
  const review = (kind: OrderItemKind, key: string) => {
    const pending = draft(kind)
    return pending ? { key, to: recordRoute(organizationId.value, kind, pending.id) } : null
  }
  if (!current.progress.receipt) return review('transfers', 'confirmReceipt') || { key: 'receipt', to: addRoute('transfers') }
  if (!current.progress.invoice) return review('invoices', 'confirmInvoice') || { key: 'invoice', to: addRoute('invoices') }
  if (!current.progress.purchase) {
    if (!activeCards.value.length) return { key: 'card', to: addRoute('vcc-cards') }
    return review('vcc-transactions', 'confirmSpend') || { key: 'spend', to: addRoute('vcc-transactions') }
  }
  if (!current.progress.ai_invoice) {
    const covered = items.value['vendor-invoices'].some((record) => record.status === 'CONFIRMED')
    return review('vendor-invoices', 'confirmAiInvoice') || { key: covered ? 'aiInvoiceMore' : 'aiInvoice', to: addRoute('vendor-invoices') }
  }
  return null
})
function stepState(step: OrderStep) {
  if (order.value?.progress[step]) return 'done'
  return steps.find((item) => !order.value?.progress[item]) === step ? 'current' : 'pending'
}
const stateTone = computed(() => ({ OPEN: 'badge-warning', COMPLETED: 'badge-success', CLOSED: 'badge-gray' })[order.value?.state || 'OPEN'])

// Every record added from here belongs to this order and returns here once saved.
function addRoute(kind: AccountingKind) {
  return createRoute(organizationId.value, kind, { order_id: orderId.value, return_to: route.path })
}
function linkRoute(kind: AccountingKind) {
  return workflowRoute(organizationId.value, 'order-link', orderId.value, { kind })
}
function topUpRoute(card: AccountingRecord) {
  return { ...(recordActionRoute(organizationId.value, 'vcc-cards', card.id, 'top-up') as object), query: { return_to: route.path } }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [current] = await Promise.all([api.order(orderId.value), organization.value ? null : loadOrganization()])
    if (current.organization_id !== organizationId.value) throw new Error(t('accounting.errors.organizationUnavailable'))
    order.value = current
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}
async function reopen() {
  busy.value = true
  try {
    const path = `orders/${orderId.value}/reopen`
    await api.mutate(path, {}, requestKey(path, {}))
    requestKey.reset()
    reopening.value = false
    await load()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>
