<template>
  <div class="accounting-page min-w-0 space-y-5" data-testid="accounting-workspace" :aria-busy="loading">
    <router-link v-if="!customer" class="accounting-back" to="/admin/accounting">
      <Icon name="arrowLeft" size="sm" aria-hidden="true" />{{ t('accounting.directory.title') }}
    </router-link>
    <header class="flex flex-wrap items-end justify-between gap-3">
      <div class="min-w-0">
        <h1 class="min-h-16 break-words text-2xl font-semibold text-gray-950 dark:text-white sm:min-h-8">
          <template v-if="customer || organizationName || !loading">{{ customer ? t('accounting.customerTitle') : organizationName || t('accounting.title') }}</template>
          <span v-else class="accounting-skeleton block h-8 w-64 max-w-full"><span class="sr-only">{{ t('accounting.loading') }}</span></span>
        </h1>
        <p v-if="customer" class="mt-1 min-h-5 text-sm text-gray-500 dark:text-dark-300">
          <span v-if="organizationName">{{ organizationName }}</span>
          <span v-else-if="loading" class="accounting-skeleton block h-5 w-48 max-w-full" aria-hidden="true" />
        </p>
      </div>
    </header>

    <div v-if="error" role="alert" class="accounting-error flex flex-wrap items-center justify-between gap-3">
      <p>{{ error }}</p>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
    </div>

    <nav v-if="ready || loading" class="accounting-tabs" role="tablist" :aria-label="t('accounting.title')">
      <button
        v-for="section in sections"
        :id="`${uid}-${section.key}`"
        :key="section.key"
        type="button"
        role="tab"
        :aria-selected="activeSection.key === section.key"
        :tabindex="activeSection.key === section.key ? 0 : -1"
        :disabled="!ready"
        @click="select(section.views[0]!)"
        @keydown="tabKey($event, section.key)"
      >
        {{ t(`accounting.sections.${section.key}`) }}
      </button>
    </nav>

    <div v-if="loading && !ready" :id="`${uid}-panel`" role="tabpanel" :aria-labelledby="`${uid}-${activeSection.key}`">
      <AccountingWorkspaceSkeleton :customer="customer" :overview="view === 'overview'" />
    </div>
    <div v-else-if="ready" :id="`${uid}-panel`" role="tabpanel" :aria-labelledby="`${uid}-${activeSection.key}`" class="min-w-0 space-y-4">
      <div v-if="activeSection.views.length > 1" class="accounting-segmented" role="group" :aria-label="t(`accounting.sections.${activeSection.key}`)">
        <button v-for="item in activeSection.views" :key="item" type="button" :aria-pressed="view === item" @click="select(item)">
          {{ t(`accounting.kinds.${item}`) }}
        </button>
      </div>

      <OverviewPanel v-if="view === 'overview'" :key="panelKey" :summaries="summaries" :organization-id="organizationId" :customer="customer" />
      <RecordList
        v-else-if="isRecordKind(view)"
        :key="`${panelKey}-list`"
        :kind="view"
        :organization-id="organizationId"
        :customer="customer"
        :currencies="currencies"
        @open="openCustomerRecord"
      />
      <OrdersPanel v-else-if="view === 'orders'" :key="`${panelKey}-orders`" :organization-id="organizationId" />
      <CostsPanel v-else-if="view === 'costs'" :key="`${panelKey}-costs`" :organization-id="organizationId" />
      <ExternalPanel v-else-if="view === 'external'" :key="`${panelKey}-external`" :organization-id="organizationId" :administrator="administrator" />
      <PeriodsPanel
        v-else-if="view === 'periods'"
        :key="`${panelKey}-periods`"
        :organization-id="organizationId"
        :organization-name="organizationName"
        :customer="customer"
        :administrator="administrator"
      />
      <ReconciliationPanel v-else-if="view === 'reconciliation'" :key="`${panelKey}-reconciliation`" :organization-id="organizationId" />
      <AuditPanel v-else-if="view === 'audit'" :key="`${panelKey}-audit`" :organization-id="organizationId" />
    </div>

    <!-- Customers get a read-only record view; admins open the full record page instead. -->
    <BaseDialog v-if="customer" :show="Boolean(customerRecord)" :title="customerRecord ? recordLabel(customerRecord.record.details, customerRecord.record.id) : ''" width="wide" @close="customerRecord = null">
      <div v-if="customerRecord" class="space-y-5">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="badge" :class="statusTone(customerRecord.record.status)">{{ t(`accounting.statuses.${customerRecord.record.status}`) }}</span>
          <span class="text-xl font-semibold tabular-nums">{{ formatMoney(displayAmount(customerRecord.kind, customerRecord.record), customerRecord.record.currency) }}</span>
        </div>
        <RecordFields :kind="customerRecord.kind" :record="customerRecord.record" customer />
        <RecordFiles v-if="customerRecord.kind !== 'vcc-cards'" :kind="customerRecord.kind" :record="customerRecord.record" customer />
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { Icon } from '@/components/icons'
import { useDesktopOrganizationAccess } from '@/composables/useDesktopOrganizationAccess'
import AuditPanel from './components/AuditPanel.vue'
import CostsPanel from './components/CostsPanel.vue'
import ExternalPanel from './components/ExternalPanel.vue'
import OrdersPanel from './components/OrdersPanel.vue'
import OverviewPanel from './components/OverviewPanel.vue'
import AccountingWorkspaceSkeleton from './components/AccountingWorkspaceSkeleton.vue'
import PeriodsPanel from './components/PeriodsPanel.vue'
import ReconciliationPanel from './components/ReconciliationPanel.vue'
import RecordFields from './components/RecordFields.vue'
import RecordFiles from './components/RecordFiles.vue'
import RecordList from './components/RecordList.vue'
import * as api from './api'
import { statusTone } from './actions'
import { formatMoney } from './format'
import { displayAmount, isRecordKind, recordLabel } from './schema'
import { errorMessage, isCanceled } from './shared'
import { useOrganization } from './useOrganization'
import type { AccountingKind, AccountingRecord, CurrencySummary } from './types'
import '@/styles/accounting.css'

const props = withDefaults(defineProps<{ customer?: boolean; organizationId?: string; administrator?: boolean }>(), {
  customer: false,
  organizationId: '',
  administrator: false,
})
const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const uid = useId()

const adminSections = [
  { key: 'overview', views: ['overview'] },
  { key: 'orders', views: ['orders'] },
  { key: 'receipts', views: ['transfers', 'receivables', 'fx-exchanges'] },
  { key: 'cards', views: ['vcc-cards', 'vcc-transactions'] },
  { key: 'documents', views: ['contracts', 'invoices', 'vendor-invoices', 'costs'] },
  { key: 'expenses', views: ['expenses', 'expense-payments'] },
  { key: 'closing', views: ['reconciliation', 'external', 'periods', 'audit'] },
]
const customerSections = ['overview', 'transfers', 'vcc-cards', 'vcc-transactions', 'contracts', 'invoices', 'periods'].map((key) => ({ key, views: [key] }))
const legacyTabs: Record<string, string> = { workbench: 'overview' }

const sections = computed(() => (props.customer ? customerSections : adminSections))
const views = computed(() => sections.value.flatMap((section) => section.views))
const view = computed(() => {
  const requested = String(route.query.tab || '')
  const normalized = legacyTabs[requested] || requested
  return views.value.includes(normalized) ? normalized : 'overview'
})
const activeSection = computed(() => sections.value.find((section) => section.views.includes(view.value)) || sections.value[0]!)

const access = useDesktopOrganizationAccess()
const { organization, missing, loadOrganization } = useOrganization(() => props.organizationId)
const customerOrganization = ref<{ id: string; name: string } | null>(null)
const summaries = ref<CurrencySummary[]>([])
const loading = ref(true)
const initialized = ref(false)
const error = ref('')
const customerRecord = ref<{ kind: AccountingKind; record: AccountingRecord } | null>(null)
let abort: AbortController | null = null

const ready = computed(() => initialized.value && (props.customer ? Boolean(customerOrganization.value) : Boolean(organization.value)))
const organizationName = computed(() => (props.customer ? customerOrganization.value?.name : organization.value?.name) || '')
// Panels remount when the bound organization changes so no stale rows survive a switch.
const panelKey = computed(() => (props.customer ? customerOrganization.value?.id : props.organizationId) || '')
const currencies = computed(() => summaries.value.map((entry) => entry.currency))

function select(next: string) {
  if (!ready.value) return
  if (next !== view.value) void router.replace({ query: { ...route.query, tab: next } })
}
function tabKey(event: KeyboardEvent, key: string) {
  const index = sections.value.findIndex((section) => section.key === key)
  const step = { ArrowRight: 1, ArrowLeft: -1 }[event.key]
  const target = event.key === 'Home' ? 0 : event.key === 'End' ? sections.value.length - 1 : step ? (index + step + sections.value.length) % sections.value.length : -1
  if (target < 0) return
  event.preventDefault()
  const section = sections.value[target]!
  select(section.views[0]!)
  document.getElementById(`${uid}-${section.key}`)?.focus()
}
function openCustomerRecord(record: AccountingRecord) {
  if (isRecordKind(view.value)) customerRecord.value = { kind: view.value, record }
}

async function load() {
  abort?.abort()
  const controller = new AbortController()
  abort = controller
  loading.value = true
  error.value = ''
  try {
    if (props.customer) {
      const bound = await access.refreshDesktopOrganizationAccess(true)
      if (controller.signal.aborted) return
      if (!bound) {
        customerOrganization.value = null
        throw new Error(t('accounting.errors.generic'))
      }
      if (customerOrganization.value?.id !== bound.public_id) {
        customerRecord.value = null
        initialized.value = false
        summaries.value = []
      }
      customerOrganization.value = { id: bound.public_id, name: bound.name || bound.public_id }
    } else {
      await loadOrganization(controller.signal)
      if (controller.signal.aborted) return
      if (missing.value) throw new Error(t('accounting.errors.organizationUnavailable'))
    }
    const result = await api.summary(props.customer, props.customer ? {} : { organization_id: props.organizationId }, controller.signal)
    if (controller.signal.aborted) return
    summaries.value = result
    initialized.value = true
  } catch (cause) {
    if (!controller.signal.aborted && !isCanceled(cause)) error.value = errorMessage(cause, t, te)
  } finally {
    if (abort === controller) loading.value = false
  }
}
// The organization binding can change in another tab; customers re-check it when returning.
function onFocus() {
  if (props.customer) void load()
}

watch(view, () => (customerRecord.value = null))
onMounted(() => {
  void load()
  window.addEventListener('focus', onFocus)
})
onUnmounted(() => {
  abort?.abort()
  window.removeEventListener('focus', onFocus)
})
</script>
