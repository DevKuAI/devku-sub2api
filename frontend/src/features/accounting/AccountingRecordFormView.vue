<template>
  <FormPage
    :title="creatingOrder ? t('accounting.orders.createTitle') : t(isEditing ? 'accounting.form.editTitle' : 'accounting.form.createTitle', { kind: t(`accounting.kinds.${kind}`) })"
    :subtitle="[organization?.name, orderName].filter(Boolean).join(' · ')"
    :back-label="t(orderId || creatingOrder ? 'accounting.orders.back' : isEditing ? 'accounting.form.backToRecord' : 'accounting.form.backToList')"
    :loading="loading"
    :error="error"
    :busy="busy"
    :submit-disabled="blocked"
    @submit="save"
    @cancel="leave(cancelLocation())"
  >
    <FormSection v-if="creatingOrder" :title="t('accounting.orders.section')" data-testid="form-section-order">
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.orders.name') }}</span>
        <input v-model="order.name" class="input w-full" maxlength="200" required data-testid="order-name" />
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.fields.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <textarea v-model="order.remark" class="input min-h-16 w-full resize-y" maxlength="4000" />
      </label>
      <label class="flex items-center gap-3 sm:col-span-2">
        <input v-model="order.withReceipt" type="checkbox" class="h-4 w-4" data-testid="order-with-receipt" />
        <span class="text-sm">{{ t('accounting.orders.withReceipt') }}</span>
      </label>
    </FormSection>
    <p v-if="blocked" role="alert" class="accounting-warning mt-6" data-testid="order-required">{{ t('accounting.form.orderRequired') }}</p>
    <FormSection
      v-for="section in sections"
      :key="section.key"
      :title="t(`accounting.form.sections.${section.key}`)"
      :data-testid="`form-section-${section.key}`"
    >
      <template v-for="field in section.fields" :key="field.key">
        <div class="min-w-0" :class="{ 'sm:col-span-2': field.wide }" :data-field="field.key">
          <!-- Receiving account: chosen from our own accounts by currency, stored as a snapshot. -->
          <template v-if="field.type === 'receiving-account'">
            <span class="input-label">{{ t('accounting.fields.receiving_account') }}</span>
            <div v-if="isEditing" class="accounting-snapshot">
              <p class="font-medium">{{ snapshotAccount }}</p>
              <p class="mt-1 text-xs text-gray-500">{{ t('accounting.form.receivingSnapshot') }}</p>
            </div>
            <template v-else-if="matchingAccounts.length">
              <select v-model="state.details.receiving_account_id" class="input w-full" required data-testid="receiving-account-select">
                <option value="">{{ t('accounting.form.select') }}</option>
                <option v-for="account in matchingAccounts" :key="account.id" :value="account.id">
                  {{ account.account_name }} · {{ account.bank_name }} · {{ account.bank_account }}
                </option>
              </select>
            </template>
            <p v-else-if="!relationsLoading" class="accounting-field-warning">
              {{ t('accounting.form.noReceivingAccount', { currency: state.currency }) }}
              <router-link class="accounting-link" :to="{ name: 'AdminAccountingReceivingAccountNew', query: { currency: state.currency, return_to: route.fullPath } }">
                {{ t('accounting.form.configureReceivingAccount') }}
              </router-link>
            </p>
          </template>

          <!-- Tax basis and rate share one control; the preview mirrors the server calculation. -->
          <template v-else-if="field.type === 'tax'">
            <span class="input-label">{{ t('accounting.fields.tax_rate') }}</span>
            <div class="flex flex-wrap items-center gap-3">
              <div class="accounting-segmented" role="group" :aria-label="t('accounting.form.taxBasis')">
                <button
                  v-for="inclusive in [true, false]"
                  :key="String(inclusive)"
                  type="button"
                  :aria-pressed="state.details.tax_inclusive === inclusive"
                  :data-testid="`tax-basis-${inclusive ? 'inclusive' : 'exclusive'}`"
                  @click="state.details.tax_inclusive = inclusive"
                >
                  {{ t(inclusive ? 'accounting.form.taxInclusive' : 'accounting.form.taxExclusive') }}
                </button>
              </div>
              <div class="relative w-28">
                <input
                  v-model="taxPercent"
                  type="number"
                  class="input w-full pr-8"
                  inputmode="decimal"
                  min="0"
                  max="100"
                  step="0.01"
                  :required="isRequired(field)"
                  :aria-label="t('accounting.fields.tax_rate')"
                  data-testid="tax-rate-input"
                />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-gray-500">%</span>
              </div>
              <div class="flex flex-wrap gap-1.5">
                <button
                  v-for="rate in taxRates"
                  :key="rate"
                  type="button"
                  class="accounting-chip"
                  :aria-pressed="taxPercent === String(rate)"
                  @click="taxPercent = String(rate)"
                >
                  {{ rate }}%
                </button>
              </div>
            </div>
            <dl v-if="kind === 'transfers' && preview" class="accounting-preview" data-testid="transfer-calculation">
              <div><dt>{{ t('accounting.fields.tax_amount') }}</dt><dd>{{ formatMoney(preview.tax, state.currency) }}</dd></div>
              <div><dt>{{ t('accounting.fields.receivable_amount') }}</dt><dd>{{ formatMoney(preview.receivable, state.currency) }}</dd></div>
              <div>
                <dt>{{ t('accounting.form.receiptDifference') }}</dt>
                <dd :class="hasDifference ? 'text-amber-700 dark:text-amber-300' : 'text-emerald-700 dark:text-emerald-300'">
                  {{ formatMoney(preview.difference || '0', state.currency) }}
                </dd>
              </div>
            </dl>
          </template>

          <label v-else-if="field.type === 'checkbox'" class="flex min-h-11 items-center gap-3">
            <input v-model="state.details[field.key]" type="checkbox" class="h-4 w-4" />
            <span class="text-sm text-gray-800 dark:text-dark-100">{{ label(field) }}</span>
          </label>

          <template v-else-if="field.type === 'segmented'">
            <span class="input-label">{{ label(field) }}</span>
            <div class="accounting-segmented" role="group" :aria-label="label(field)">
              <button
                v-for="option in field.options"
                :key="option"
                type="button"
                :aria-pressed="state.details[field.key] === option"
                :disabled="locked(field)"
                :data-testid="`${field.key}-${option}`"
                @click="state.details[field.key] = option"
              >
                {{ t(`accounting.options.${field.key}.${option}`) }}
              </button>
            </div>
          </template>

          <label v-else class="block">
            <span class="input-label">
              {{ label(field) }}<span v-if="!isRequired(field) && !locked(field)" class="accounting-optional">{{ t('accounting.form.optional') }}</span>
            </span>
            <select v-if="field.type === 'select'" v-model="state.details[field.key]" class="input w-full" :required="isRequired(field)">
              <option value="">{{ t('accounting.form.select') }}</option>
              <option v-for="option in selectOptions(field)" :key="option" :value="option">
                {{ te(`accounting.options.${field.key}.${option}`) ? t(`accounting.options.${field.key}.${option}`) : option }}
              </option>
            </select>
            <select
              v-else-if="field.type === 'parent'"
              v-model="state.parent_id"
              class="input w-full"
              :required="isRequired(field)"
              :disabled="locked(field)"
              @change="syncParentCurrency"
            >
              <option value="">{{ t('accounting.form.select') }}</option>
              <option v-for="record in referenceOptions(field)" :key="record.id" :value="record.id">{{ optionLabel(record, field.reference!) }}</option>
            </select>
            <select v-else-if="field.type === 'reference'" v-model="state.details[field.key]" class="input w-full" :required="isRequired(field)">
              <option value="">{{ t(isRequired(field) ? 'accounting.form.select' : 'accounting.form.none') }}</option>
              <option v-for="record in referenceOptions(field)" :key="record.id" :value="record.id">{{ optionLabel(record, field.reference!) }}</option>
            </select>
            <select v-else-if="field.type === 'contact'" v-model="state.details[field.key]" class="input w-full">
              <option value="">{{ t('accounting.form.none') }}</option>
              <option v-for="contact in contacts" :key="contact.id" :value="contact.id">
                {{ contact.name }} · {{ t(`accounting.contactRoles.${contact.role}`) }}
              </option>
            </select>
            <textarea
              v-else-if="field.type === 'textarea'"
              :value="String(state.details[field.key] ?? '')"
              @input="state.details[field.key] = ($event.target as HTMLTextAreaElement).value"
              class="input min-h-20 w-full resize-y"
              maxlength="4000"
              :required="isRequired(field)"
            />
            <input
              v-else-if="field.type === 'currency'"
              :value="currencyValue(field)"
              class="input w-full uppercase"
              list="accounting-currencies"
              maxlength="3"
              pattern="[A-Za-z]{3}"
              autocomplete="off"
              :required="isRequired(field)"
              :disabled="locked(field)"
              @input="setCurrency(field, $event)"
            />
            <input
              v-else-if="field.key === 'amount'"
              v-model="state.amount"
              type="number"
              class="input w-full"
              inputmode="decimal"
              :min="moneyStep(state.currency)"
              :step="moneyStep(state.currency)"
              required
              data-testid="amount-input"
            />
            <input
              v-else-if="field.key === 'occurred_at'"
              v-model="state.occurred_at"
              type="datetime-local"
              class="input w-full"
              required
            />
            <input
              v-else
              v-model="state.details[field.key]"
              class="input w-full"
              :type="inputType(field)"
              :inputmode="field.type === 'money' || field.type === 'decimal' ? 'decimal' : undefined"
              :step="field.type === 'money' ? moneyStep(state.currency) : field.type === 'decimal' ? 'any' : undefined"
              :min="field.type === 'money' || field.type === 'decimal' ? '0' : undefined"
              :autocomplete="field.type === 'secret' ? 'new-password' : 'off'"
              :required="isRequired(field)"
              :data-testid="`detail-${field.key}`"
              @input="field.key === 'exchange_rate' && (rateTouched = true)"
            />
          </label>

          <p v-if="field.type === 'contact' && !relationsLoading && !contacts.length" class="accounting-field-hint">
            {{ t('accounting.form.noContacts') }}
            <router-link class="accounting-link" :to="{ name: 'AdminAccountingOrganizationContactNew', params: { organizationId }, query: { return_to: route.fullPath } }">
              {{ t('accounting.form.createContact') }}
            </router-link>
          </p>
          <p v-else-if="field.hint" class="accounting-field-hint">{{ t(`accounting.form.${field.hint}`) }}</p>
          <p v-if="field.type === 'secret' && isEditing" class="accounting-field-hint">{{ t('accounting.form.secretKept') }}</p>
        </div>
      </template>

      <div v-if="section.key === 'validity' && kind === 'contracts'" class="sm:col-span-2" data-testid="contract-period-shortcuts">
        <span class="input-label">{{ t('accounting.form.contractTerm') }}</span>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="months in contractTerms"
            :key="months"
            type="button"
            class="accounting-chip"
            :aria-pressed="contractTerm === months"
            @click="applyContractTerm(months)"
          >
            {{ t('accounting.form.months', { count: months }) }}
          </button>
        </div>
      </div>
      <div v-if="section.key === 'invoiceAmounts'" class="sm:col-span-2">
        <span class="input-label">{{ t('accounting.fields.total_amount') }}</span>
        <p class="text-lg font-semibold tabular-nums" data-testid="invoice-total">{{ formatMoney(invoiceTotal || '0', state.currency) }}</p>
      </div>
    </FormSection>

    <FormSection v-if="!isEditing && sections.length" :title="t('accounting.form.sections.proof')" :hint="t('accounting.files.hint')">
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.files.choose') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input type="file" accept=".jpg,.jpeg,.png,.webp,.pdf" class="block w-full text-sm" @change="chooseFile" />
      </label>
    </FormSection>
    <datalist id="accounting-currencies">
      <option v-for="code in commonCurrencies" :key="code" :value="code" />
    </datalist>
  </FormPage>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import FormPage from './components/FormPage.vue'
import FormSection from './components/FormSection.vue'
import * as api from './api'
import { formatMoney } from './format'
import { addMoney, taxPreview } from './money'
import { baseKeys, creatableKinds, orderOnlyKinds, recordLabel, schemas, type FieldDef, type FormState } from './schema'
import {
  commonCurrencies,
  errorMessage,
  localDate,
  localDateTime,
  moneyStep,
  orderRoute,
  recordRoute,
  safeReturnPath,
  toISO,
  workspaceRoute,
} from './shared'
import { useLeaveGuard } from './useLeaveGuard'
import { useOrganization } from './useOrganization'
import type { AccountingKind, AccountingOrder, AccountingRecord, OrganizationContact, ReceivingAccount } from './types'

const { t, te } = useI18n()
const route = useRoute()
const organizationId = computed(() => String(route.params.organizationId || ''))
// Creating an order records its first receipt in the same form.
const creatingOrder = computed(() => route.name === 'AdminAccountingOrderNew')
const kind = computed<AccountingKind>(() => {
  const requested = String(route.params.kind) as AccountingKind
  return !creatingOrder.value && creatableKinds.includes(requested) ? requested : 'transfers'
})
const recordId = computed(() => String(route.params.id || ''))
const isEditing = computed(() => Boolean(recordId.value))
const orderId = computed(() => (isEditing.value ? '' : String(route.query.order_id || '')))
const orderName = ref('')
const order = reactive({ name: '', remark: '', withReceipt: true })
// Cards and card spend are only ever recorded inside an order.
const blocked = computed(() => !isEditing.value && !creatingOrder.value && orderOnlyKinds.includes(kind.value) && !orderId.value)
const schema = computed(() => schemas[kind.value])

const { organization, loadOrganization } = useOrganization(() => organizationId.value)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const relationsLoading = ref(false)
const file = ref<File | null>(null)
const original = ref<AccountingRecord | null>(null)
const contacts = ref<OrganizationContact[]>([])
const accounts = ref<ReceivingAccount[]>([])
const references = reactive<Partial<Record<AccountingKind, AccountingRecord[]>>>({})
const rateTouched = ref(false)
const state = reactive<FormState>({
  currency: String(route.query.currency || 'CNY').toUpperCase(),
  amount: '',
  occurred_at: localDateTime(),
  parent_id: String(route.query.parent_id || ''),
  details: {},
})
const requestKey = api.stableRequest()
const { leave, markClean } = useLeaveGuard(() => [state, order, file.value?.name])
const taxRates = [0, 1, 3, 6, 9, 13]
const contractTerms = [6, 12, 24, 36]

const allFields = computed(() => schema.value.sections.flatMap((section) => section.fields))
const sections = computed(() =>
  (blocked.value || (creatingOrder.value && !order.withReceipt) ? [] : schema.value.sections)
    .map((section) => ({ key: section.key, fields: section.fields.filter(visible) }))
    .filter((section) => section.fields.length),
)
const matchingAccounts = computed(() => accounts.value.filter((account) => account.currency === state.currency))
const snapshotAccount = computed(() =>
  [original.value?.details.receiving_account_name, original.value?.details.receiving_bank_name, original.value?.details.receiving_account]
    .filter(Boolean)
    .join(' · '),
)
const preview = computed(() => (kind.value === 'transfers' ? taxPreview(state) : null))
const hasDifference = computed(() => Boolean(preview.value && Number(preview.value.difference) !== 0))
const invoiceTotal = computed(() => addMoney(String(state.details.net_amount || '0'), String(state.details.tax_amount || '0')))
const taxPercent = computed({
  get: () => (state.details.tax_rate === '' || state.details.tax_rate === undefined ? '' : String(Number((Number(state.details.tax_rate) * 100).toFixed(4)))),
  set: (value: string) => {
    state.details.tax_rate = value === '' ? '' : String(Number((Number(value) / 100).toFixed(6)))
  },
})
const contractTerm = computed(() =>
  contractTerms.find((months) => state.details.effective_at && contractEnd(String(state.details.effective_at), months) === state.details.expires_at),
)

function visible(field: FieldDef) {
  if (field.key === 'difference_reason') return hasDifference.value || Boolean(state.details.difference_reason)
  return !field.show || field.show(state)
}
function isRequired(field: FieldDef) {
  if (field.key === 'difference_reason') return hasDifference.value
  if (field.requiredOnCreate && !isEditing.value) return true
  return typeof field.required === 'function' ? field.required(state) : Boolean(field.required)
}
function locked(field: FieldDef) {
  // Invoices raised from an order are always sales invoices to this organization.
  if (field.key === 'invoice_type' && orderId.value) return true
  return isEditing.value && Boolean(field.lockedOnEdit)
}
function label(field: FieldDef) {
  return t(`accounting.${field.label || `fields.${field.key}`}`)
}
function inputType(field: FieldDef) {
  return { money: 'number', decimal: 'number', date: 'date', month: 'month', secret: 'password' }[field.type as string] || 'text'
}
function selectOptions(field: FieldDef) {
  const current = String(state.details[field.key] || '')
  return current && !field.options?.includes(current) ? [...(field.options || []), current] : field.options || []
}
function referenceOptions(field: FieldDef) {
  return (references[field.reference!] || []).filter(
    (record) => record.id === state.details[field.key] || record.id === state.parent_id || !field.filter || field.filter(record, state),
  )
}
function optionLabel(record: AccountingRecord, referenceKind: AccountingKind) {
  const balance =
    referenceKind === 'transfers'
      ? String(record.details.unallocated_amount ?? record.amount)
      : referenceKind === 'vcc-cards'
        ? String(record.details.available_balance || '0')
        : String(record.details.effective_amount || record.amount)
  return `${recordLabel(record.details, record.id)} · ${formatMoney(balance, record.currency)} · ${record.occurred_at.slice(0, 10)}`
}
function currencyValue(field: FieldDef) {
  return field.key === 'currency' ? state.currency : String(state.details[field.key] || '')
}
function setCurrency(field: FieldDef, event: Event) {
  const value = (event.target as HTMLInputElement).value.toUpperCase()
  if (field.key === 'currency') state.currency = value
  else state.details[field.key] = value
}
function syncParentCurrency() {
  const parent = (references[kind.value === 'fx-exchanges' ? 'transfers' : 'vcc-cards'] || []).find((record) => record.id === state.parent_id)
  if (parent) state.currency = parent.currency
}
function chooseFile(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0] || null
}
function contractEnd(start: string, months: number) {
  const [year, month, day] = start.split('-').map(Number)
  if (!year || !month || !day) return ''
  const end = new Date(Date.UTC(year, month - 1 + months, day))
  if (end.getUTCDate() !== day) end.setUTCDate(0)
  else end.setUTCDate(end.getUTCDate() - 1)
  return end.toISOString().slice(0, 10)
}
function applyContractTerm(months: number) {
  const start = String(state.details.effective_at || state.details.signed_at || localDate())
  state.details.effective_at = start
  state.details.expires_at = contractEnd(start, months)
}

// Keep the receiving account valid for the chosen currency; a single match is selected automatically.
watch(
  () => [state.currency, accounts.value],
  () => {
    if (kind.value !== 'transfers' || isEditing.value) return
    if (!matchingAccounts.value.some((account) => account.id === state.details.receiving_account_id))
      state.details.receiving_account_id = matchingAccounts.value.length === 1 ? matchingAccounts.value[0]!.id : ''
  },
)
// Suggest the exchange rate from both amounts until the user types one.
watch(
  () => [state.amount, state.details.target_amount],
  ([source, target]) => {
    if (kind.value !== 'fx-exchanges' || rateTouched.value) return
    const rate = Number(target) / Number(source)
    state.details.exchange_rate = Number.isFinite(rate) && rate > 0 ? String(Number(rate.toFixed(10))) : ''
  },
)
watch(
  () => state.details.invoice_type,
  (type) => {
    if (kind.value === 'invoices' && type === 'SALES' && !state.details.recipient_name && organization.value)
      state.details.recipient_name = organization.value.name
  },
)

function initialize(record: AccountingRecord | null) {
  const details: FormState['details'] = {}
  for (const field of allFields.value) {
    if (baseKeys.has(field.key)) continue
    const previous = record?.details[field.key]
    if (previous !== undefined) details[field.key] = previous
    else if (field.type === 'checkbox') details[field.key] = false
    else if (field.type === 'segmented' && field.required) details[field.key] = field.options![0]!
    else details[field.key] = ''
  }
  if (allFields.value.some((field) => field.type === 'tax')) details.tax_inclusive = record ? record.details.tax_inclusive === true : true
  if (kind.value === 'fx-exchanges' && !record) details.fee_amount = '0'
  if (kind.value === 'invoices' && !record) details.recipient_name = organization.value?.name || ''
  if (kind.value === 'invoices' && orderId.value) details.invoice_type = 'SALES'
  state.details = details
  if (!record) return
  state.currency = record.currency
  state.amount = record.amount
  state.occurred_at = localDateTime(record.occurred_at)
  state.parent_id = record.parent_id || ''
  rateTouched.value = true
}

async function loadRelations() {
  const kinds = new Set(allFields.value.flatMap((field) => (field.reference ? [field.reference] : [])))
  relationsLoading.value = true
  try {
    const [contactRows, accountRows, ...rows] = await Promise.all([
      allFields.value.some((field) => field.type === 'contact') ? api.organizationContacts(organizationId.value) : [],
      kind.value === 'transfers' && !isEditing.value ? api.receivingAccounts() : [],
      ...[...kinds].map((referenceKind) =>
        api.listRecords(referenceKind, false, { organization_id: organizationId.value, page_size: 100 }).then((result) => result.items),
      ),
    ])
    contacts.value = contactRows
    accounts.value = accountRows
    ;[...kinds].forEach((referenceKind, index) => {
      references[referenceKind] = rows[index]!
    })
  } finally {
    relationsLoading.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [, record, parentOrder] = await Promise.all([
      loadOrganization(),
      isEditing.value ? api.getRecord(kind.value, recordId.value) : Promise.resolve(null),
      orderId.value ? api.order(orderId.value) : Promise.resolve(null),
    ])
    orderName.value = parentOrder?.name || ''
    if (creatingOrder.value) order.name = `${organization.value?.name || ''} ${localDate()}`.trim()
    if (record && record.organization_id !== organizationId.value) throw new Error(t('accounting.errors.organizationUnavailable'))
    original.value = record
    initialize(record)
    await loadRelations()
    if (!isEditing.value) syncParentCurrency()
    await nextTick()
    markClean()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}

function payload() {
  const details: Record<string, string | boolean> = {}
  for (const field of allFields.value) {
    if (baseKeys.has(field.key)) continue
    const value = visible(field) ? state.details[field.key]! : field.type === 'checkbox' ? false : ''
    // Number inputs yield numbers; the API accepts detail values only as strings or booleans.
    details[field.key] = typeof value === 'number' ? String(value) : value
  }
  if ('tax_inclusive' in state.details) details.tax_inclusive = state.details.tax_inclusive!
  if (kind.value === 'vcc-cards') {
    const [year, month] = String(details.expiry || '').split('-')
    if (year && month) details.expiry = `${month}/${year.slice(-2)}`
    else delete details.expiry
    if (isEditing.value) for (const key of ['card_number', 'cvv', 'provider_token']) if (!details[key]) delete details[key]
  }
  if (!isEditing.value && kind.value === 'transfers') details.payer_name = organization.value?.name || ''
  const dateField = schema.value.occurredAtFrom
  const occurredAt = dateField
    ? toISO(String(details[dateField]))
    : allFields.value.some((field) => field.key === 'occurred_at')
      ? toISO(state.occurred_at)
      : original.value?.occurred_at || new Date().toISOString()
  return {
    organization_id: organizationId.value,
    parent_id: state.parent_id || undefined,
    currency: state.currency,
    amount:
      kind.value === 'invoices'
        ? invoiceTotal.value
        : allFields.value.some((field) => field.key === 'amount')
          ? String(state.amount)
          : original.value?.amount || '0',
    occurred_at: occurredAt,
    details,
    ...(original.value ? { version: original.value.version } : {}),
    ...(orderId.value ? { order_id: orderId.value } : {}),
  }
}

function cancelLocation() {
  if (isEditing.value) return recordRoute(organizationId.value, kind.value, recordId.value)
  if (creatingOrder.value) return workspaceRoute(organizationId.value, 'orders')
  return safeReturnPath(route.query.return_to) || (orderId.value ? orderRoute(organizationId.value, orderId.value) : workspaceRoute(organizationId.value, kind.value))
}

async function createOrder() {
  const body = {
    organization_id: organizationId.value,
    name: order.name.trim(),
    remark: order.remark.trim(),
    ...(order.withReceipt ? { transfer: payload() } : {}),
  }
  const created = (await api.mutate('orders', body, requestKey('orders', body))) as AccountingOrder
  requestKey.reset()
  let uploadFailed = false
  if (file.value && order.withReceipt) {
    try {
      const receipt = (await api.order(created.id)).items?.transfers?.[0]
      if (receipt) await api.upload(file.value, 'transfers', receipt)
    } catch {
      uploadFailed = true
    }
  }
  await leave({ ...(orderRoute(organizationId.value, created.id) as object), query: uploadFailed ? { upload_failed: '1' } : {} }, true)
}

async function save() {
  busy.value = true
  error.value = ''
  try {
    if (creatingOrder.value) return await createOrder()
    const body = payload()
    const path = isEditing.value ? `${kind.value}/${recordId.value}` : kind.value
    const saved = (await api.mutate(path, body, requestKey(path, body), isEditing.value ? 'patch' : 'post')) as AccountingRecord
    requestKey.reset()
    let uploadFailed = false
    if (file.value) {
      try {
        await api.upload(file.value, kind.value, saved)
      } catch {
        uploadFailed = true
      }
    }
    const back = safeReturnPath(route.query.return_to) || (orderId.value ? orderRoute(organizationId.value, orderId.value) : '')
    await leave(
      back && !isEditing.value && !uploadFailed
        ? back
        : { ...(recordRoute(organizationId.value, kind.value, saved.id) as object), query: uploadFailed ? { upload_failed: '1' } : {} },
      true,
    )
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>
