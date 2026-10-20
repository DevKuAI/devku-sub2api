<template>
  <li class="accounting-entry" :data-testid="`entry-${kind}`" :data-id="record.id">
    <div class="accounting-entry-main">
      <div class="min-w-0">
        <p class="accounting-entry-title">
          {{ title }}
          <span v-if="record.status === 'DRAFT'" class="badge badge-warning ms-1.5">{{ t('accounting.entries.draft') }}</span>
          <span v-if="credit" class="badge badge-gray ms-1.5">{{ t('accounting.entries.refund') }}</span>
          <span v-if="record.status === 'CLOSED'" class="badge badge-gray ms-1.5">{{ t('accounting.entries.cardClosed') }}</span>
          <span v-if="missingFile" class="badge badge-gray ms-1.5" data-testid="entry-missing-file">{{ t('accounting.entries.missingFile') }}</span>
        </p>
        <p v-if="subtitle" class="accounting-entry-subtitle">{{ subtitle }}</p>
      </div>
      <div class="accounting-entry-amount">
        <p :class="{ 'text-amber-700 dark:text-amber-300': negative }">{{ amountText }}</p>
        <p v-if="note" class="text-xs font-normal text-amber-700 dark:text-amber-300">{{ note }}</p>
      </div>
    </div>
    <div class="accounting-entry-tools">
      <button v-if="record.status === 'DRAFT' && editable" type="button" class="accounting-link" data-testid="entry-confirm" @click="emit('confirm')">{{ t('accounting.entries.makeEffective') }}</button>
      <button v-if="kind === 'vcc-cards' && record.status === 'ACTIVE' && editable" type="button" class="accounting-link" :aria-expanded="panel === 'top-up'" @click="toggle('top-up')">{{ t('accounting.entries.topUp') }}</button>
      <button v-if="kind !== 'vcc-cards' && kind !== 'vcc-transactions'" type="button" class="accounting-link" :aria-expanded="panel === 'files'" data-testid="entry-files" @click="toggle('files')">
        {{ t('accounting.entries.files', { count: files }) }}
      </button>
      <button v-if="refundable && editable" type="button" class="accounting-link" @click="emit('refund')">{{ t('accounting.entries.addRefund') }}</button>
      <button v-if="editable && (kind !== 'vcc-cards' || record.status === 'ACTIVE')" type="button" class="accounting-link" data-testid="entry-edit" @click="emit('edit')">{{ t('common.edit') }}</button>
      <button v-if="editable && (kind !== 'vcc-cards' || record.status === 'ACTIVE')" type="button" class="accounting-link text-red-600 dark:text-red-400" data-testid="entry-remove" @click="emit('remove')">
        {{ t(kind === 'vcc-cards' ? 'accounting.entries.closeCard' : 'common.delete') }}
      </button>
    </div>
    <div v-if="panel === 'files'" class="accounting-entry-panel">
      <RecordFiles :kind="kind" :record="record" @changed="emit('changed')" />
    </div>
    <div v-if="panel === 'top-up'" class="accounting-entry-panel">
      <CardTopUpForm :card="record" @saved="toppedUp" @cancel="panel = ''" />
    </div>
  </li>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CardTopUpForm from './CardTopUpForm.vue'
import RecordFiles from '../RecordFiles.vue'
import { formatMoney } from '../../format'
import { compareMoney } from '../../money'
import { cardLabel, percent } from '../../orderEntries'
import { formatDate } from '../../shared'
import type { AccountingRecord, OrderItemKind } from '../../types'

const props = defineProps<{ kind: OrderItemKind; record: AccountingRecord; cards?: AccountingRecord[]; editable: boolean }>()
const emit = defineEmits<{ edit: []; remove: []; confirm: []; refund: []; changed: [] }>()
const { t } = useI18n()
const panel = ref<'' | 'files' | 'top-up'>('')
const details = computed(() => props.record.details)
const credit = computed(() => details.value.transaction_type === 'REFUND' || details.value.direction === 'CREDIT')
const files = computed(() => Number(details.value.attachment_count || 0))
const missingFile = computed(() => ['transfers', 'invoices', 'vendor-invoices'].includes(props.kind) && props.record.status !== 'DRAFT' && files.value === 0)
const refundable = computed(() => props.kind === 'vcc-transactions' && props.record.status === 'CONFIRMED' && details.value.transaction_type === 'CONSUME')
const balance = computed(() => String(details.value.available_balance || '0'))
const negative = computed(() => props.kind === 'vcc-cards' && compareMoney(balance.value, '0') < 0)

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
  if (props.kind === 'vcc-cards') return t('accounting.entries.balance', { amount: formatMoney(balance.value, props.record.currency) })
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

function toggle(next: 'files' | 'top-up') {
  panel.value = panel.value === next ? '' : next
}
function toppedUp() {
  panel.value = ''
  emit('changed')
}
</script>
