<template>
  <li class="accounting-entry" :data-testid="`entry-${kind}`" :data-id="record.id">
    <div class="accounting-entry-main">
      <div class="min-w-0">
        <p class="accounting-entry-title">
          {{ title }}
          <span v-if="record.status === 'DRAFT'" class="badge badge-warning ms-1.5">{{ t('accounting.entries.draft') }}</span>
          <span v-if="credit" class="badge badge-gray ms-1.5">{{ t('accounting.entries.refund') }}</span>
          <span v-if="fromStatement" class="badge badge-gray ms-1.5" data-testid="entry-from-statement">{{ t('accounting.entries.fromStatement') }}</span>
          <span v-if="details.channel_match" class="badge badge-warning ms-1.5" data-testid="entry-match-review">{{ t('accounting.entries.matchReview') }}</span>
          <span v-if="vendorMissing" class="badge badge-warning ms-1.5" data-testid="entry-vendor-missing">{{ t('accounting.entries.vendorMissing') }}</span>
          <span v-if="record.status === 'CLOSED'" class="badge badge-gray ms-1.5">{{ t('accounting.entries.cardClosed') }}</span>
          <span v-if="missingFile" class="badge badge-gray ms-1.5" data-testid="entry-missing-file">{{ t('accounting.entries.missingFile') }}</span>
        </p>
        <p v-if="subtitle" class="accounting-entry-subtitle">{{ subtitle }}</p>
      </div>
      <div class="accounting-entry-amount">
        <p>{{ amountText }}</p>
        <p v-if="note" class="text-xs font-normal text-amber-700 dark:text-amber-300">{{ note }}</p>
      </div>
    </div>
    <div class="accounting-entry-tools">
      <button v-if="record.status === 'DRAFT' && editable" type="button" class="accounting-link" data-testid="entry-confirm" @click="emit('confirm')">{{ t('accounting.entries.makeEffective') }}</button>
      <button v-if="kind !== 'vcc-cards' && kind !== 'vcc-transactions'" type="button" class="accounting-link" :aria-expanded="panel === 'files'" data-testid="entry-files" @click="toggle('files')">
        {{ t('accounting.entries.files', { count: files }) }}
      </button>
      <button v-if="editable && (kind !== 'vcc-cards' || record.status === 'ACTIVE')" type="button" class="accounting-link" data-testid="entry-edit" @click="emit('edit')">
        {{ t(kind === 'vcc-transactions' ? 'accounting.entries.pickVendor' : 'common.edit') }}
      </button>
      <button v-if="movable && editable" type="button" class="accounting-link" :aria-expanded="panel === 'move'" data-testid="entry-move" @click="toggle('move')">
        {{ t('accounting.entries.move') }}
      </button>
      <button v-if="editable && removable" type="button" class="accounting-link text-red-600 dark:text-red-400" data-testid="entry-remove" @click="emit('remove')">
        {{ t(kind === 'vcc-cards' ? 'accounting.entries.closeCard' : 'common.delete') }}
      </button>
    </div>
    <div v-if="panel === 'files'" class="accounting-entry-panel">
      <RecordFiles :kind="kind" :record="record" @changed="emit('changed')" />
    </div>
    <form v-if="panel === 'move'" class="accounting-entry-panel flex flex-wrap items-center gap-2" data-testid="entry-move-form" @submit.prevent="move">
      <select v-model="target" class="input w-auto max-w-full" required :aria-label="t('accounting.entries.move')">
        <option value="">{{ t(targets.length ? 'accounting.form.select' : 'accounting.entries.noOtherOrder') }}</option>
        <option v-for="option in targets" :key="option.id" :value="option.id">{{ option.name }}</option>
      </select>
      <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !target">{{ t('accounting.entries.moveConfirm') }}</button>
      <button type="button" class="btn btn-ghost btn-sm" @click="panel = ''">{{ t('common.cancel') }}</button>
      <p v-if="moveError" role="alert" class="accounting-error w-full">{{ moveError }}</p>
    </form>
  </li>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RecordFiles from '../RecordFiles.vue'
import * as api from '../../api'
import { formatMoney } from '../../format'
import { compareMoney } from '../../money'
import { cardLabel, percent } from '../../orderEntries'
import { errorMessage, formatDate } from '../../shared'
import type { AccountingOrder, AccountingRecord, OrderItemKind } from '../../types'

const props = defineProps<{ kind: OrderItemKind; record: AccountingRecord; cards?: AccountingRecord[]; editable: boolean; order?: AccountingOrder }>()
const emit = defineEmits<{ edit: []; remove: []; confirm: []; changed: [] }>()
const { t, te } = useI18n()
const panel = ref<'' | 'files' | 'move'>('')
const details = computed(() => props.record.details)
const credit = computed(() => details.value.transaction_type === 'REFUND' || details.value.direction === 'CREDIT')
const files = computed(() => Number(details.value.attachment_count || 0))
const missingFile = computed(() => ['transfers', 'invoices', 'vendor-invoices'].includes(props.kind) && props.record.status !== 'DRAFT' && files.value === 0)
// Card spend comes from statements: it is never added or removed here, and moves between orders with its refunds.
const fromStatement = computed(() => props.kind === 'vcc-transactions' && details.value.source === 'CHANNEL')
const vendorMissing = computed(() => props.kind === 'vcc-transactions' && details.value.transaction_type === 'CONSUME' && !details.value.vendor_id)
const movable = computed(() => props.kind === 'vcc-transactions' && props.record.status === 'CONFIRMED' && !credit.value)
const removable = computed(() => (props.kind === 'vcc-cards' ? props.record.status === 'ACTIVE' : !fromStatement.value))
const targets = ref<{ id: string; name: string }[]>([])
const target = ref('')
const busy = ref(false)
const moveError = ref('')
const requestKey = api.stableRequest()

const title = computed(() => {
  const d = details.value
  switch (props.kind) {
    case 'transfers':
      return String(d.bank_reference || d.payer_name || '')
    case 'invoices':
      return t('accounting.entries.invoiceNumber', { number: d.invoice_number })
    case 'vcc-cards':
      return cardLabel(props.record)
    case 'vcc-transactions':
      return String(d.merchant || '')
    default:
      return [d.vendor, d.invoice_number].filter(Boolean).join(' · ')
  }
})
const subtitle = computed(() => {
  const d = details.value
  const date = formatDate(props.record.occurred_at)
  switch (props.kind) {
    case 'transfers':
      return [date, d.payer_name, t('accounting.entries.taxRate', { rate: percent(d.tax_rate) })].filter(Boolean).join(' · ')
    case 'invoices':
      return [d.issue_date || date, d.invoice_code ? t('accounting.entries.invoiceCode', { code: d.invoice_code }) : ''].filter(Boolean).join(' · ')
    case 'vcc-cards':
      return [d.provider, props.record.currency].filter(Boolean).join(' · ')
    case 'vcc-transactions': {
      const card = props.cards?.find((item) => item.id === props.record.parent_id)
      return [date, card ? cardLabel(card) : '', d.provider_transaction_id].filter(Boolean).join(' · ')
    }
    default:
      return [d.period_start, d.period_end].filter(Boolean).join(' ~ ')
  }
})
const amountText = computed(() => {
  if (props.kind === 'vcc-cards') return t('accounting.entries.spent', { amount: formatMoney(String(details.value.spent_amount || '0'), props.record.currency) })
  const value = String(details.value.effective_amount || props.record.amount)
  return `${credit.value ? '−' : ''}${formatMoney(value, props.record.currency)}`
})
// AI invoices show what they have not matched to card spend yet.
const note = computed(() => {
  const left = String(details.value.unallocated_cost || '0')
  return props.kind === 'vendor-invoices' && props.record.status === 'CONFIRMED' && compareMoney(left, '0') > 0
    ? t('accounting.entries.uncovered', { amount: formatMoney(left, props.record.currency) })
    : ''
})

function toggle(next: 'files' | 'move') {
  panel.value = panel.value === next ? '' : next
  if (panel.value === 'move') void loadTargets()
}
// Other open orders of the same organization.
async function loadTargets() {
  target.value = ''
  moveError.value = ''
  try {
    const result = await api.orders({ organization_id: props.order?.organization_id, state: 'open', page_size: 100 })
    targets.value = result.items.filter((item) => item.id !== props.order?.id).map((item) => ({ id: item.id, name: item.name }))
  } catch (cause) {
    moveError.value = errorMessage(cause, t, te)
  }
}
async function move() {
  busy.value = true
  moveError.value = ''
  const path = `orders/${target.value}/links`
  const payload = { object_kind: 'vcc-transactions', object_id: props.record.id }
  try {
    await api.mutate(path, payload, requestKey(path, payload))
    requestKey.reset()
    panel.value = ''
    emit('changed')
  } catch (cause) {
    moveError.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
</script>
