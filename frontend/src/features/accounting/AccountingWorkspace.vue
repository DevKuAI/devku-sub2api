<template>
  <div class="accounting-page min-w-0 space-y-5" data-testid="accounting-workspace" :aria-busy="loading">
    <PageHeader :title="t('accounting.customerTitle')" :subtitle="organization?.name" />

    <div v-if="error" role="alert" class="accounting-error flex flex-wrap items-center justify-between gap-3">
      <p>{{ error }}</p>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
    </div>

    <nav v-if="ready || loading" class="accounting-tabs" role="tablist" :aria-label="t('accounting.customerTitle')">
      <button
        v-for="section in sections"
        :id="`${uid}-${section}`"
        :key="section"
        type="button"
        role="tab"
        :aria-selected="view === section"
        :tabindex="view === section ? 0 : -1"
        :disabled="!ready"
        @click="select(section)"
        @keydown="tabKey($event, section)"
      >
        {{ t(`accounting.sections.${section}`) }}
      </button>
    </nav>

    <div v-if="loading && !ready" :id="`${uid}-panel`" role="tabpanel" :aria-labelledby="`${uid}-${view}`">
      <AccountingWorkspaceSkeleton customer :overview="view === 'overview'" />
    </div>
    <div v-else-if="ready" :id="`${uid}-panel`" role="tabpanel" :aria-labelledby="`${uid}-${view}`" class="min-w-0 space-y-4">
      <OverviewPanel v-if="view === 'overview'" :key="panelKey" :summaries="summaries" />
      <PeriodsPanel v-else-if="view === 'periods'" :key="`${panelKey}-periods`" :organization-name="organization?.name" />
      <RecordList v-else-if="isRecordKind(view)" :key="`${panelKey}-${view}`" :kind="view" :currencies="currencies" @open="openRecord" />
    </div>

    <BaseDialog :show="Boolean(record)" :title="record ? customerLabel(record.kind, record.record) : ''" width="wide" @close="record = null">
      <div v-if="record" class="space-y-5">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="badge" :class="statusTone(record.record.status)">{{ t(`accounting.statuses.${record.record.status}`) }}</span>
          <span class="text-xl font-semibold tabular-nums">{{ formatMoney(displayAmount(record.kind, record.record), record.record.currency) }}</span>
        </div>
        <RecordFields :kind="record.kind" :record="record.record" customer />
        <RecordFiles v-if="record.kind !== 'vcc-cards'" :kind="record.kind" :record="record.record" customer />
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useDesktopOrganizationAccess } from '@/composables/useDesktopOrganizationAccess'
import AccountingWorkspaceSkeleton from './components/AccountingWorkspaceSkeleton.vue'
import OverviewPanel from './components/OverviewPanel.vue'
import PageHeader from './components/PageHeader.vue'
import PeriodsPanel from './components/PeriodsPanel.vue'
import RecordFields from './components/RecordFields.vue'
import RecordFiles from './components/RecordFiles.vue'
import RecordList from './components/RecordList.vue'
import * as api from './api'
import { statusTone } from './actions'
import { formatMoney } from './format'
import { customerLabel, displayAmount, isRecordKind } from './schema'
import { errorMessage, isCanceled } from './shared'
import type { AccountingKind, AccountingRecord, CurrencySummary } from './types'
import '@/styles/accounting.css'

// The organization's read-only view of its own books.
const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const uid = useId()
const sections = ['overview', 'transfers', 'vcc-cards', 'vcc-transactions', 'contracts', 'invoices', 'periods']
const view = computed(() => {
  const requested = String(route.query.tab || '')
  return sections.includes(requested) ? requested : 'overview'
})

const access = useDesktopOrganizationAccess()
const organization = ref<{ id: string; name: string } | null>(null)
const summaries = ref<CurrencySummary[]>([])
const loading = ref(true)
const initialized = ref(false)
const error = ref('')
const record = ref<{ kind: AccountingKind; record: AccountingRecord } | null>(null)
let abort: AbortController | null = null

const ready = computed(() => initialized.value && Boolean(organization.value))
// Panels remount when the bound organization changes so no stale rows survive a switch.
const panelKey = computed(() => organization.value?.id || '')
const currencies = computed(() => summaries.value.map((entry) => entry.currency))

function select(next: string) {
  if (ready.value && next !== view.value) void router.replace({ query: { ...route.query, tab: next } })
}
function tabKey(event: KeyboardEvent, key: string) {
  const index = sections.indexOf(key)
  const step = { ArrowRight: 1, ArrowLeft: -1 }[event.key]
  const target = event.key === 'Home' ? 0 : event.key === 'End' ? sections.length - 1 : step ? (index + step + sections.length) % sections.length : -1
  if (target < 0) return
  event.preventDefault()
  select(sections[target]!)
  document.getElementById(`${uid}-${sections[target]}`)?.focus()
}
function openRecord(row: AccountingRecord) {
  if (isRecordKind(view.value)) record.value = { kind: view.value, record: row }
}

async function load() {
  abort?.abort()
  const controller = new AbortController()
  abort = controller
  loading.value = true
  error.value = ''
  try {
    const bound = await access.refreshDesktopOrganizationAccess(true)
    if (controller.signal.aborted) return
    if (!bound) {
      organization.value = null
      throw new Error(t('accounting.errors.generic'))
    }
    if (organization.value?.id !== bound.public_id) {
      record.value = null
      initialized.value = false
      summaries.value = []
    }
    organization.value = { id: bound.public_id, name: bound.name || bound.public_id }
    const result = await api.summary(true, {}, controller.signal)
    if (controller.signal.aborted) return
    summaries.value = result
    initialized.value = true
  } catch (cause) {
    if (!controller.signal.aborted && !isCanceled(cause)) error.value = errorMessage(cause, t, te)
  } finally {
    if (abort === controller) loading.value = false
  }
}
// The organization binding can change in another tab, so it is checked again when the window regains focus.
function onFocus() {
  void load()
}

watch(view, () => (record.value = null))
onMounted(() => {
  void load()
  window.addEventListener('focus', onFocus)
})
onUnmounted(() => {
  abort?.abort()
  window.removeEventListener('focus', onFocus)
})
</script>
