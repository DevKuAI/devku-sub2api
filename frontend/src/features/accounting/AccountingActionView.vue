<template>
  <FormPage
    :title="t(`accounting.actions.${titleKey}`)"
    :subtitle="organization?.name"
    :back-label="t('common.back')"
    :loading="loading"
    :error="error"
    :busy="busy"
    :danger="danger"
    :submit-label="t(`accounting.actions.${titleKey}`)"
    :submit-disabled="unavailable"
    @submit="save"
    @cancel="leave(returnLocation())"
  >
    <FormSection v-if="record" :title="t('accounting.actionForm.source')">
      <dl class="accounting-fields sm:col-span-2">
        <div><dt>{{ t(`accounting.kinds.${kind}`) }}</dt><dd>{{ recordLabel(record.details, record.id) }}</dd></div>
        <div><dt>{{ t('accounting.amount') }}</dt><dd>{{ formatMoney(effectiveAmount(record), record.currency) }}</dd></div>
        <div v-if="limit"><dt>{{ t(`accounting.fields.${limit.field}`) }}</dt><dd>{{ formatMoney(limit.amount, limit.currency) }}</dd></div>
      </dl>
    </FormSection>
    <FormSection v-else-if="parentOrder" :title="t('accounting.actionForm.source')">
      <dl class="accounting-fields sm:col-span-2">
        <div><dt>{{ t('accounting.orders.name') }}</dt><dd>{{ parentOrder.name }}</dd></div>
        <div><dt>{{ t('accounting.status') }}</dt><dd>{{ t(`accounting.orders.states.${parentOrder.state}`) }}</dd></div>
      </dl>
    </FormSection>
    <FormSection v-else-if="target" :title="t('accounting.actionForm.source')">
      <dl class="accounting-fields sm:col-span-2">
        <div><dt>{{ t('accounting.kinds.vcc-cards') }}</dt><dd>{{ target.card_name }} •••• {{ target.last_four }}</dd></div>
        <div><dt>{{ t('accounting.actionForm.externalAction') }}</dt><dd>{{ t(`accounting.options.external_action.${target.action}`) }}</dd></div>
        <div><dt>{{ t('accounting.amount') }}</dt><dd>{{ formatMoney(String(target.amount), String(target.currency)) }}</dd></div>
      </dl>
    </FormSection>

    <FormSection :title="t('accounting.actionForm.details')" :hint="te(`accounting.actionForm.hints.${action}`) ? t(`accounting.actionForm.hints.${action}`) : undefined">
      <!-- Choose where the money or cost goes. -->
      <label v-if="action === 'allocate' || action === 'fx-allocate'" class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.kinds.vcc-cards') }}</span>
        <select v-model="form.target_id" class="input w-full" required>
          <option value="">{{ t('accounting.form.select') }}</option>
          <option v-for="card in options" :key="card.id" :value="card.id">{{ optionLabel(card, 'available_balance') }}</option>
        </select>
        <span v-if="!options.length" class="accounting-field-hint">{{ t('accounting.actionForm.noCards') }}</span>
      </label>
      <label v-if="action === 'settle'" class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.kinds.transfers') }}</span>
        <select v-model="form.target_id" class="input w-full" required>
          <option value="">{{ t('accounting.form.select') }}</option>
          <option v-for="transfer in options" :key="transfer.id" :value="transfer.id">{{ optionLabel(transfer) }}</option>
        </select>
        <span v-if="!options.length" class="accounting-field-hint">{{ t('accounting.actionForm.noTransfers') }}</span>
      </label>
      <template v-if="action === 'cost'">
        <label v-if="!record" class="block sm:col-span-2">
          <span class="input-label">{{ t('accounting.kinds.vendor-invoices') }}</span>
          <select v-model="form.invoice_id" class="input w-full" required>
            <option value="">{{ t('accounting.form.select') }}</option>
            <option v-for="invoice in invoices" :key="invoice.id" :value="invoice.id">{{ optionLabel(invoice, 'unallocated_cost') }}</option>
          </select>
        </label>
        <div class="sm:col-span-2">
          <span class="input-label">{{ t('accounting.actionForm.costSource') }}</span>
          <div class="accounting-segmented" role="group">
            <button
              v-for="sourceKind in ['vcc-transactions', 'expenses']"
              :key="sourceKind"
              type="button"
              :aria-pressed="form.source_kind === sourceKind"
              @click="form.source_kind = sourceKind; form.target_id = ''"
            >
              {{ t(`accounting.kinds.${sourceKind}`) }}
            </button>
          </div>
        </div>
        <label class="block sm:col-span-2">
          <span class="input-label">{{ t(`accounting.kinds.${form.source_kind}`) }}</span>
          <select v-model="form.target_id" class="input w-full" required>
            <option value="">{{ t('accounting.form.select') }}</option>
            <option v-for="source in costSources" :key="source.id" :value="source.id">{{ optionLabel(source) }}</option>
          </select>
        </label>
      </template>

      <label v-if="hasAmount" class="block">
        <span class="input-label">{{ t(action === 'adjust' ? 'accounting.actionForm.adjustment' : action === 'match' || action === 'exclude' ? 'accounting.actionForm.referenceAmount' : 'accounting.amount') }}</span>
        <input
          v-model="form.amount"
          type="number"
          class="input w-full"
          inputmode="decimal"
          :min="action === 'adjust' ? undefined : action === 'match' || action === 'exclude' ? '0' : moneyStep(amountCurrency)"
          :step="moneyStep(amountCurrency)"
          required
          data-testid="action-amount"
        />
        <span v-if="action === 'adjust' && adjustedTotal" class="accounting-field-hint">{{ t('accounting.actionForm.adjustedTotal', { amount: formatMoney(adjustedTotal, amountCurrency) }) }}</span>
      </label>
      <label v-if="['pay', 'settle', 'cost', 'top-up'].includes(action)" class="block">
        <span class="input-label">{{ t('accounting.occurredAt') }}</span>
        <input v-model="form.occurred_at" type="datetime-local" class="input w-full" required />
      </label>
      <template v-if="action === 'cost' && crossCurrency">
        <label class="block">
          <span class="input-label">{{ t('accounting.actionForm.sourceAmount', { currency: selectedSource?.currency }) }}</span>
          <input v-model="form.source_amount" type="number" class="input w-full" inputmode="decimal" min="0" step="any" required />
        </label>
        <label class="block">
          <span class="input-label">{{ t('accounting.fields.exchange_rate') }}</span>
          <input v-model="form.exchange_rate" type="number" class="input w-full" inputmode="decimal" min="0" step="any" required />
        </label>
      </template>

      <template v-if="action === 'pay'">
        <label class="block">
          <span class="input-label">{{ t('accounting.fields.payment_account') }}</span>
          <input v-model="form.payment_account" class="input w-full" required />
        </label>
        <label class="block">
          <span class="input-label">{{ t('accounting.fields.transaction_number') }}</span>
          <input v-model="form.reference" class="input w-full" required />
        </label>
      </template>

      <template v-if="action === 'match' || action === 'exclude' || action === 'external'">
        <label class="block" :class="{ 'sm:col-span-2': action === 'external' }">
          <span class="input-label">{{ t(action === 'external' ? 'accounting.actionForm.providerReference' : 'accounting.actionForm.reference') }}</span>
          <input v-model="form.reference" class="input w-full" :required="action === 'external'" />
        </label>
      </template>

      <!-- Proof: pick an uploaded file or upload a new one here. -->
      <div v-if="proofMode" class="sm:col-span-2" data-testid="action-proof">
        <span class="input-label">
          {{ t(action === 'pay' ? 'accounting.fields.proof_attachment_id' : 'accounting.actionForm.proof') }}<span v-if="proofMode === 'optional'" class="accounting-optional">{{ t('accounting.form.optional') }}</span>
        </span>
        <select v-if="files.length" v-model="form.attachment_id" class="input w-full" :required="proofMode === 'required' && !proofFile">
          <option value="">{{ t(proofFile ? 'accounting.actionForm.useNewFile' : 'accounting.form.select') }}</option>
          <option v-for="file in files" :key="file.id" :value="file.id">{{ file.filename }}</option>
        </select>
        <p v-if="files.length" class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('accounting.actionForm.orUpload') }}</p>
        <AccountingFileUpload class="mt-2" :model-value="proofFile" :disabled="busy" @update:model-value="chooseProof" />
      </div>

      <template v-if="action === 'order-edit'">
        <label class="block sm:col-span-2">
          <span class="input-label">{{ t('accounting.orders.name') }}</span>
          <input v-model="form.name" class="input w-full" maxlength="200" required />
        </label>
        <label class="block sm:col-span-2">
          <span class="input-label">{{ t('accounting.fields.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
          <textarea v-model="form.remark" class="input min-h-20 w-full resize-y" maxlength="4000" />
        </label>
      </template>
      <label v-if="action === 'order-link'" class="block sm:col-span-2">
        <span class="input-label">{{ t(`accounting.kinds.${linkKind}`) }}</span>
        <select v-model="form.target_id" class="input w-full" required>
          <option value="">{{ t('accounting.form.select') }}</option>
          <option v-for="item in options" :key="item.id" :value="item.id">{{ optionLabel(item) }} · {{ t(`accounting.statuses.${item.status}`) }}</option>
        </select>
        <span v-if="!options.length" class="accounting-field-hint">{{ t('accounting.actionForm.nothingToLink') }}</span>
      </label>
      <template v-if="action === 'top-up'">
        <label class="block">
          <span class="input-label">{{ t('accounting.actionForm.reference') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
          <input v-model="form.reference" class="input w-full" maxlength="200" />
        </label>
        <label class="block sm:col-span-2">
          <span class="input-label">{{ t('accounting.fields.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
          <textarea v-model="form.remark" class="input min-h-20 w-full resize-y" maxlength="4000" />
        </label>
      </template>

      <template v-if="action === 'close'">
        <label class="block">
          <span class="input-label">{{ t('accounting.actionForm.month') }}</span>
          <input v-model="form.month" type="month" class="input w-full" :max="lastClosableMonth" required />
        </label>
        <label class="flex items-start gap-3 sm:col-span-2">
          <input v-model="form.accept_partial" type="checkbox" class="mt-1 h-4 w-4" />
          <span class="text-sm text-gray-700 dark:text-dark-200">{{ t('accounting.actionForm.acceptPartial') }}</span>
        </label>
      </template>

      <p v-if="overpay" class="accounting-warning sm:col-span-2">
        {{ t(auth.isAdmin ? 'accounting.actionForm.overpay' : 'accounting.actionForm.overpayAdminOnly') }}
      </p>
      <label v-if="reasonShown" class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.reason') }}<span v-if="!reasonRequired" class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <textarea v-model="form.reason" class="input min-h-20 w-full resize-y" maxlength="4000" :required="reasonRequired" />
      </label>
      <label v-if="action === 'pay'" class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.fields.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <textarea v-model="form.remark" class="input min-h-20 w-full resize-y" maxlength="4000" />
      </label>
    </FormSection>
  </FormPage>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import FormPage from './components/FormPage.vue'
import FormSection from './components/FormSection.vue'
import AccountingFileUpload from './components/AccountingFileUpload.vue'
import * as api from './api'
import { formatMoney } from './format'
import { addMoney, compareMoney, subtractMoney } from './money'
import { effectiveAmount, recordLabel } from './schema'
import { errorMessage, localDateTime, moneyStep, orderRoute, recordRoute, safeReturnPath, toISO, workspaceRoute } from './shared'
import { useLeaveGuard } from './useLeaveGuard'
import { useOrganization } from './useOrganization'
import type { AccountingAttachment, AccountingKind, AccountingOrder, AccountingRecord, WorkflowRow } from './types'

const recordActions = ['void', 'allocate', 'pay', 'adjust', 'match', 'exclude', 'settle', 'cost', 'fx-allocate', 'top-up']
const workflowActions = ['cost', 'external', 'external-exempt', 'close', 'reopen', 'void-cost', 'void-settlement', 'order-close', 'order-edit', 'order-link']
const orderActions = ['order-close', 'order-edit', 'order-link']
const returnTabs: Record<string, string> = {
  cost: 'costs',
  'void-cost': 'costs',
  external: 'external',
  'external-exempt': 'external',
  close: 'periods',
  reopen: 'periods',
}

const { t, te } = useI18n()
const route = useRoute()
const auth = useAuthStore()
const organizationId = computed(() => String(route.params.organizationId))
const kind = computed(() => String(route.params.kind || '') as AccountingKind)
const recordId = computed(() => String(route.params.id || ''))
const targetId = computed(() => String(route.params.targetId || ''))
const action = computed(() => {
  const requested = String(route.params.action)
  return (recordId.value ? recordActions : workflowActions).includes(requested) ? requested : ''
})
const titleKey = computed(
  () =>
    ({
      'external-exempt': 'externalExempt',
      'void-cost': 'voidCost',
      'void-settlement': 'voidSettlement',
      'fx-allocate': 'allocate',
      'top-up': 'topUp',
      'order-close': 'orderClose',
      'order-edit': 'orderEdit',
      'order-link': 'orderLink',
      close: recordId.value ? 'close' : 'closePeriod',
    })[action.value] ||
    action.value ||
    'void',
)
const danger = computed(() => ['void', 'void-cost', 'void-settlement', 'external-exempt', 'reopen'].includes(action.value))
const linkKind = computed(() => String(route.query.kind || 'transfers') as AccountingKind)

const { organization, loadOrganization } = useOrganization(() => organizationId.value)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const record = ref<AccountingRecord | null>(null)
const target = ref<WorkflowRow | null>(null)
const parentOrder = ref<AccountingOrder | null>(null)
const options = ref<AccountingRecord[]>([])
const invoices = ref<AccountingRecord[]>([])
const sources = reactive<Record<string, AccountingRecord[]>>({ 'vcc-transactions': [], expenses: [] })
const files = ref<AccountingAttachment[]>([])
const proofFile = ref<File | null>(null)
const form = reactive({
  target_id: '',
  invoice_id: '',
  source_kind: 'vcc-transactions',
  amount: '',
  occurred_at: localDateTime(),
  source_amount: '',
  exchange_rate: '',
  payment_account: '',
  reference: '',
  attachment_id: '',
  reason: '',
  remark: '',
  month: '',
  name: '',
  accept_partial: false,
})
const requestKey = api.stableRequest()
const { leave, markClean } = useLeaveGuard(() => [form, proofFile.value?.name])

const now = new Date()
const lastClosableMonth = localDateTime(new Date(now.getFullYear(), now.getMonth() - 1, 15)).slice(0, 7)
const unavailable = computed(() => !action.value || (!record.value && Boolean(recordId.value)))
const hasAmount = computed(() => ['allocate', 'fx-allocate', 'pay', 'adjust', 'match', 'exclude', 'settle', 'cost', 'top-up'].includes(action.value))
// The remaining amount that bounds this action, shown next to the source record.
const limit = computed(() => {
  const current = record.value
  if (!current) return null
  const field = { allocate: 'unallocated_amount', pay: 'unpaid_amount', settle: 'outstanding_amount', cost: 'unallocated_cost', 'fx-allocate': 'target_available', 'top-up': 'available_balance' }[action.value]
  if (!field || current.details[field] === undefined) return null
  return { field, amount: String(current.details[field]), currency: action.value === 'fx-allocate' ? String(current.details.target_currency) : current.currency }
})
const amountCurrency = computed(() => {
  if (action.value === 'fx-allocate') return String(record.value?.details.target_currency || 'CNY')
  if (action.value === 'cost') return selectedInvoice.value?.currency || 'CNY'
  return record.value?.currency || 'CNY'
})
const adjustedTotal = computed(() => (record.value && form.amount ? addMoney(effectiveAmount(record.value), String(form.amount)) : ''))
const overpay = computed(() => action.value === 'pay' && Boolean(limit.value) && compareMoney(String(form.amount), limit.value!.amount) > 0)
const proofMode = computed(() => ({ pay: 'required', external: 'required', match: 'optional', exclude: 'optional' })[action.value] as 'required' | 'optional' | undefined)
const reasonRequired = computed(() => {
  if (['void', 'adjust', 'exclude', 'external-exempt', 'reopen', 'void-cost', 'void-settlement', 'order-close'].includes(action.value)) return true
  if (action.value === 'match') return Boolean(record.value) && compareMoney(String(form.amount), effectiveAmount(record.value!)) !== 0
  if (action.value === 'close') return form.accept_partial
  return overpay.value
})
const reasonShown = computed(() => reasonRequired.value || ['match', 'close'].includes(action.value))
const selectedInvoice = computed(() => (record.value && action.value === 'cost' ? record.value : invoices.value.find((invoice) => invoice.id === form.invoice_id)))
const costSources = computed(() => sources[form.source_kind] || [])
const selectedSource = computed(() => costSources.value.find((source) => source.id === form.target_id))
const crossCurrency = computed(() => Boolean(selectedSource.value && selectedInvoice.value && selectedSource.value.currency !== selectedInvoice.value.currency))

function optionLabel(item: AccountingRecord, balanceField?: string) {
  const balance = balanceField ? String(item.details[balanceField] ?? item.amount) : effectiveAmount(item)
  return `${recordLabel(item.details, item.id)} · ${formatMoney(balance, item.currency)} · ${item.occurred_at.slice(0, 10)}`
}
function chooseProof(file: File | null) {
  proofFile.value = file
  if (proofFile.value) form.attachment_id = ''
}
function returnLocation() {
  const back = safeReturnPath(route.query.return_to)
  if (back) return back
  if (orderActions.includes(action.value)) return orderRoute(organizationId.value, targetId.value)
  if (record.value) return recordRoute(organizationId.value, kind.value, record.value.id)
  return workspaceRoute(organizationId.value, returnTabs[action.value] || 'overview')
}
async function all(listKind: AccountingKind) {
  const rows: AccountingRecord[] = []
  for (let page = 1; ; page++) {
    const result = await api.listRecords(listKind, false, { organization_id: organizationId.value, page, page_size: 100 })
    rows.push(...result.items)
    if (page * 100 >= result.total || !result.items.length) return rows
  }
}

watch(
  () => [form.target_id, form.invoice_id],
  () => {
    if (action.value !== 'cost' || !selectedSource.value || !selectedInvoice.value) return
    const remaining = String(selectedInvoice.value.details.unallocated_cost ?? selectedInvoice.value.amount)
    const source = effectiveAmount(selectedSource.value)
    form.amount = crossCurrency.value || compareMoney(source, remaining) > 0 ? remaining : source
  },
)

async function load() {
  loading.value = true
  error.value = ''
  try {
    if (!action.value) throw new Error(t('accounting.errors.referenceUnavailable'))
    const [current] = await Promise.all([recordId.value ? api.getRecord(kind.value, recordId.value) : null, loadOrganization()])
    if (current && current.organization_id !== organizationId.value) throw new Error(t('accounting.errors.organizationUnavailable'))
    record.value = current
    form.amount = action.value === 'top-up' ? '' : limit.value?.amount || (['match', 'exclude'].includes(action.value) && current ? effectiveAmount(current) : '')
    // A top-up usually just clears the card's shortfall.
    if (action.value === 'top-up' && current && compareMoney(String(current.details.available_balance || '0'), '0') < 0)
      form.amount = subtractMoney('0', String(current.details.available_balance))
    if (current && action.value === 'pay') form.payment_account = String(current.details.payment_account || '')
    if (action.value === 'close') form.month = lastClosableMonth

    if (action.value === 'allocate' || action.value === 'fx-allocate') {
      const currency = action.value === 'allocate' ? current!.currency : String(current!.details.target_currency)
      options.value = (await all('vcc-cards')).filter((card) => card.status === 'ACTIVE' && card.currency === currency)
      if (options.value.length === 1) form.target_id = options.value[0]!.id
    }
    if (action.value === 'settle')
      options.value = (await all('transfers')).filter(
        (transfer) => transfer.status === 'RECEIVED' && transfer.currency === current!.currency && transfer.details.funding_type !== 'FX',
      )
    if (action.value === 'cost') {
      const [invoiceRows, transactions, expenses] = await Promise.all([current ? [] : all('vendor-invoices'), all('vcc-transactions'), all('expenses')])
      invoices.value = invoiceRows.filter((invoice) => invoice.status === 'CONFIRMED' && Number(invoice.details.unallocated_cost ?? invoice.amount) > 0)
      sources['vcc-transactions'] = transactions.filter(
        (row) => row.status === 'CONFIRMED' && row.details.transaction_type !== 'REFUND' && !(row.details.transaction_type === 'ADJUSTMENT' && row.details.direction === 'CREDIT'),
      )
      sources.expenses = expenses.filter((row) => !['DRAFT', 'VOID'].includes(row.status))
      if (route.query.invoice_id) form.invoice_id = String(route.query.invoice_id)
    }
    if (['pay', 'match', 'exclude'].includes(action.value))
      files.value = (await api.attachments(kind.value, current!.id, false)).filter((file) => file.status === 'COMPLETE')
    if (orderActions.includes(action.value)) {
      parentOrder.value = await api.order(targetId.value)
      if (parentOrder.value.organization_id !== organizationId.value) throw new Error(t('accounting.errors.organizationUnavailable'))
      form.name = parentOrder.value.name
      form.remark = parentOrder.value.remark
    }
    // Only records outside any order can be linked; sales invoices only, and never internal funding.
    if (action.value === 'order-link')
      options.value = (await all(linkKind.value)).filter(
        (item) =>
          !item.details.order_id &&
          item.status !== 'VOID' &&
          !item.details.funding_type &&
          (linkKind.value !== 'invoices' || item.details.invoice_type === 'SALES'),
      )
    if (['external', 'external-exempt'].includes(action.value)) {
      const rows = await api.workflow<WorkflowRow[]>('external-actions', organizationId.value)
      target.value = rows.find((row) => row.id === targetId.value) || null
      if (!target.value) throw new Error(t('accounting.errors.referenceUnavailable'))
      if (action.value === 'external')
        files.value = (await api.attachments('vcc-cards', String(target.value.card_id), false)).filter((file) => file.status === 'COMPLETE')
    }
    await nextTick()
    markClean()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}

// Uploads a newly chosen proof to the object it documents and returns its attachment id.
async function proofId() {
  if (!proofFile.value) return form.attachment_id
  const owner =
    action.value === 'external'
      ? { kind: 'vcc-cards' as AccountingKind, id: String(target.value!.card_id) }
      : { kind: kind.value, id: record.value!.id }
  const attachment = await api.upload(proofFile.value, owner.kind, { id: owner.id, organization_id: organizationId.value })
  files.value = [...files.value, attachment]
  form.attachment_id = attachment.id
  proofFile.value = null
  return attachment.id
}

function request(attachmentId: string): [string, Record<string, unknown>, ('post' | 'patch')?] {
  const id = record.value?.id
  const scope = { organization_id: organizationId.value }
  const amount = String(form.amount)
  switch (action.value) {
    case 'void':
      return [`${kind.value}/${id}/void`, { reason: form.reason }]
    case 'allocate':
      return [`transfers/${id}/allocations`, { card_id: form.target_id, amount }]
    case 'fx-allocate':
      return [`fx-exchanges/${id}/allocations`, { ...scope, source_id: form.target_id, amount }]
    case 'pay':
      return [
        `expenses/${id}/payments`,
        {
          organization_id: organizationId.value,
          parent_id: id,
          currency: record.value!.currency,
          amount,
          occurred_at: toISO(form.occurred_at),
          details: {
            payment_account: form.payment_account,
            transaction_number: form.reference,
            proof_attachment_id: attachmentId,
            override: overpay.value,
            reason: overpay.value ? form.reason : '',
            remark: form.remark,
          },
        },
      ]
    case 'adjust':
    case 'match':
    case 'exclude':
      return [
        `reconciliation/${action.value === 'adjust' ? 'adjustments' : action.value}`,
        { object_kind: kind.value, object_id: id, currency: record.value!.currency, amount, reference: form.reference, reason: form.reason, attachment_id: attachmentId },
      ]
    case 'settle':
      return [`receivables/${id}/settlements`, { ...scope, transfer_id: form.target_id, amount, occurred_at: toISO(form.occurred_at) }]
    case 'cost':
      return [
        'cost-allocations',
        {
          ...scope,
          invoice_id: selectedInvoice.value!.id,
          source_kind: form.source_kind,
          source_id: form.target_id,
          amount,
          source_amount: crossCurrency.value ? String(form.source_amount) : '',
          exchange_rate: crossCurrency.value ? String(form.exchange_rate) : '',
          occurred_at: toISO(form.occurred_at),
        },
      ]
    case 'external':
      return [`external-actions/${targetId.value}/confirm`, { ...scope, reference: form.reference, attachment_id: attachmentId }]
    case 'external-exempt':
      return [`external-actions/${targetId.value}/void`, { ...scope, reason: form.reason }]
    case 'close':
      return ['periods/close', { ...scope, month: form.month, accept_partial: form.accept_partial, reason: form.reason }]
    case 'reopen':
      return [`periods/${targetId.value}/reopen`, { ...scope, reason: form.reason }]
    case 'void-settlement':
      return [`settlements/${targetId.value}/void`, { ...scope, reason: form.reason }]
    case 'top-up':
      return [`vcc-cards/${id}/top-ups`, { amount, occurred_at: toISO(form.occurred_at), reference: form.reference, remark: form.remark }]
    case 'order-close':
      return [`orders/${targetId.value}/close`, { reason: form.reason }]
    case 'order-edit':
      return [`orders/${targetId.value}`, { name: form.name, remark: form.remark, version: parentOrder.value?.version }, 'patch']
    case 'order-link':
      return [`orders/${targetId.value}/links`, { object_kind: linkKind.value, object_id: form.target_id }]
    default:
      return [`cost-allocations/${targetId.value}/void`, { ...scope, reason: form.reason }]
  }
}

async function save() {
  if (action.value === 'adjust' && compareMoney(String(form.amount), '0') === 0) {
    error.value = t('accounting.actionForm.nonZeroAdjustment')
    return
  }
  if (proofMode.value === 'required' && !form.attachment_id && !proofFile.value) {
    error.value = t('accounting.actionForm.proofRequired')
    return
  }
  busy.value = true
  error.value = ''
  try {
    const [path, payload, method] = request(await proofId())
    await api.mutate(path, payload, requestKey(path, payload), method ?? 'post')
    requestKey.reset()
    await leave(returnLocation(), true)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>
