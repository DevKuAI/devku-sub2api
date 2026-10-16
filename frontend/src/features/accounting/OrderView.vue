<template>
  <AppLayout>
    <main class="accounting-page mx-auto w-full max-w-5xl space-y-5 pb-10" data-testid="accounting-order-page">
      <router-link class="accounting-back" :to="workspaceRoute(organizationId, 'orders')">
        <Icon name="arrowLeft" size="sm" aria-hidden="true" />
        {{ organization?.name || t('accounting.title') }} · {{ t('accounting.sections.orders') }}
      </router-link>

      <p v-if="loading && !order" role="status" class="text-sm text-gray-500">{{ t('accounting.loading') }}</p>
      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="route.query.upload_failed" role="status" class="accounting-warning">{{ t('accounting.files.uploadFailed') }}</p>

      <template v-if="order">
        <header class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="break-words text-2xl font-semibold text-gray-950 dark:text-white">{{ order.name }}</h1>
              <span class="badge" :class="stateTone">{{ t(`accounting.orders.states.${order.state}`) }}</span>
            </div>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">
              {{ [formatDate(order.created_at), order.remark].filter(Boolean).join(' · ') }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <router-link class="btn btn-secondary" :to="workflowRoute(organizationId, 'order-edit', order.id)">{{ t('accounting.actions.edit') }}</router-link>
            <router-link v-if="order.status === 'OPEN' && order.state !== 'COMPLETED'" class="btn btn-ghost" :to="workflowRoute(organizationId, 'order-close', order.id)">
              {{ t('accounting.actions.orderClose') }}
            </router-link>
            <button v-if="order.status === 'CLOSED'" type="button" class="btn btn-secondary" :disabled="busy" @click="reopening = true">
              {{ t('accounting.actions.orderReopen') }}
            </button>
          </div>
        </header>

        <p v-if="order.status === 'CLOSED'" class="accounting-warning">{{ t('accounting.orders.closedReason', { reason: order.close_reason }) }}</p>

        <ol class="grid gap-2 sm:grid-cols-4" data-testid="order-progress">
          <li
            v-for="step in steps"
            :key="step"
            class="flex items-center gap-2 rounded-lg border px-3 py-2.5 text-sm"
            :class="order.progress[step] ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200' : 'border-gray-200 text-gray-500 dark:border-dark-700'"
            :data-step="step"
            :data-done="order.progress[step]"
          >
            <span aria-hidden="true">{{ order.progress[step] ? '✓' : '○' }}</span>
            {{ t(`accounting.orders.steps.${step}`) }}
          </li>
        </ol>

        <dl v-if="order.totals.length" class="grid gap-3" data-testid="order-totals">
          <div v-for="total in order.totals" :key="total.currency" class="card grid grid-cols-2 gap-3 p-4 sm:grid-cols-4">
            <div v-for="metric in totalMetrics" :key="metric">
              <dt class="text-xs text-gray-500">{{ t(`accounting.orders.totals.${metric}`) }}</dt>
              <dd class="mt-0.5 font-semibold tabular-nums">{{ formatMoney(total[metric], total.currency) }}</dd>
            </div>
          </div>
        </dl>

        <!-- Receipt -->
        <section class="card space-y-3 p-5" data-testid="order-section-receipt">
          <header class="accounting-toolbar">
            <h2 class="accounting-section-title">{{ t('accounting.orders.steps.receipt') }}</h2>
            <div v-if="open" class="flex flex-wrap gap-2">
              <router-link class="btn btn-ghost btn-sm" :to="linkRoute('transfers')">{{ t('accounting.orders.linkExisting') }}</router-link>
              <router-link class="btn btn-secondary btn-sm" :to="addRoute('transfers')"><Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.add.transfers') }}</router-link>
            </div>
          </header>
          <OrderRecordRows kind="transfers" :records="items.transfers" :organization-id="organizationId" />
        </section>

        <!-- Sales invoice -->
        <section class="card space-y-3 p-5" data-testid="order-section-invoice">
          <header class="accounting-toolbar">
            <h2 class="accounting-section-title">{{ t('accounting.orders.steps.invoice') }}</h2>
            <div v-if="open" class="flex flex-wrap gap-2">
              <router-link class="btn btn-ghost btn-sm" :to="linkRoute('invoices')">{{ t('accounting.orders.linkExisting') }}</router-link>
              <router-link class="btn btn-secondary btn-sm" :to="addRoute('invoices')"><Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.add.invoices') }}</router-link>
            </div>
          </header>
          <OrderRecordRows kind="invoices" :records="items.invoices" :organization-id="organizationId" />
        </section>

        <!-- Card purchases -->
        <section class="card space-y-4 p-5" data-testid="order-section-purchase">
          <header class="accounting-toolbar">
            <h2 class="accounting-section-title">{{ t('accounting.orders.steps.purchase') }}</h2>
            <div v-if="open" class="flex flex-wrap gap-2">
              <router-link class="btn btn-ghost btn-sm" :to="addRoute('vcc-cards')"><Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.add.vcc-cards') }}</router-link>
              <router-link v-if="activeCards.length" class="btn btn-secondary btn-sm" :to="addRoute('vcc-transactions')">
                <Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.add.vcc-transactions') }}
              </router-link>
            </div>
          </header>
          <div>
            <h3 class="mb-1 text-sm font-medium text-gray-500">{{ t('accounting.kinds.vcc-cards') }}</h3>
            <ul v-if="items['vcc-cards'].length" class="divide-y divide-gray-100 dark:divide-dark-700">
              <li v-for="card in items['vcc-cards']" :key="card.id" class="flex flex-wrap items-center justify-between gap-2 py-2.5 text-sm" :data-card="card.id">
                <router-link class="min-w-0 hover:text-primary-600" :to="recordRoute(organizationId, 'vcc-cards', card.id)">
                  <span class="font-medium">{{ recordLabel(card.details, card.id) }}</span>
                  <span class="text-gray-500"> •••• {{ card.details.last_four }} · {{ card.currency }}</span>
                </router-link>
                <span class="flex items-center gap-3">
                  <span class="tabular-nums" :class="{ 'text-amber-700 dark:text-amber-300': Number(card.details.available_balance) < 0 }">
                    {{ t('accounting.fields.available_balance') }} {{ formatMoney(String(card.details.available_balance || '0'), card.currency) }}
                  </span>
                  <router-link v-if="card.status === 'ACTIVE'" class="btn btn-ghost btn-sm" :to="topUpRoute(card)">{{ t('accounting.actions.topUp') }}</router-link>
                </span>
              </li>
            </ul>
            <p v-else class="text-sm text-gray-500">{{ t('accounting.orders.noCards') }}</p>
            <p v-if="negativeCards" class="accounting-field-hint">{{ t('accounting.orders.negativeCardHint') }}</p>
          </div>
          <div>
            <h3 class="mb-1 text-sm font-medium text-gray-500">{{ t('accounting.kinds.vcc-transactions') }}</h3>
            <OrderRecordRows kind="vcc-transactions" :records="items['vcc-transactions']" :organization-id="organizationId" />
          </div>
        </section>

        <!-- AI vendor invoices -->
        <section class="card space-y-3 p-5" data-testid="order-section-ai-invoice">
          <header class="accounting-toolbar">
            <h2 class="accounting-section-title">{{ t('accounting.orders.steps.ai_invoice') }}</h2>
            <div v-if="open" class="flex flex-wrap gap-2">
              <router-link class="btn btn-ghost btn-sm" :to="linkRoute('vendor-invoices')">{{ t('accounting.orders.linkExisting') }}</router-link>
              <router-link class="btn btn-secondary btn-sm" :to="addRoute('vendor-invoices')"><Icon name="plus" size="sm" aria-hidden="true" />{{ t('accounting.orders.add.vendor-invoices') }}</router-link>
            </div>
          </header>
          <OrderRecordRows kind="vendor-invoices" :records="items['vendor-invoices']" :organization-id="organizationId" />
          <p v-if="order.progress.purchase && !order.progress.ai_invoice" class="accounting-field-hint">{{ t('accounting.orders.aiInvoiceHint') }}</p>
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
const totalMetrics: (keyof Omit<OrderTotal, 'currency'>)[] = ['received', 'invoiced', 'spent', 'ai_cost']
const itemKinds: OrderItemKind[] = ['transfers', 'invoices', 'vcc-cards', 'vcc-transactions', 'vendor-invoices']

const open = computed(() => order.value?.status === 'OPEN')
const items = computed(
  () => Object.fromEntries(itemKinds.map((kind) => [kind, order.value?.items?.[kind] || []])) as Record<OrderItemKind, AccountingRecord[]>,
)
const activeCards = computed(() => items.value['vcc-cards'].filter((card) => card.status === 'ACTIVE'))
const negativeCards = computed(() => items.value['vcc-cards'].some((card) => Number(card.details.available_balance) < 0))
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
