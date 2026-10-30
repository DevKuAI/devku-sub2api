<template>
  <form class="accounting-entry-form" novalidate :aria-busy="busy || undefined" :data-testid="`entry-form-${kind}`" @submit.prevent="submit">
    <p v-if="error" ref="errorRef" role="alert" tabindex="-1" class="accounting-error sm:col-span-2 lg:col-span-3">{{ error }}</p>
    <template v-for="field in fields" :key="field.key">
      <label class="block min-w-0" :class="{ 'sm:col-span-2 lg:col-span-3': field.wide }" :data-field="field.key">
        <span class="input-label">
          {{ t(`accounting.entries.fields.${field.key}`) }}<span v-if="!field.required" class="accounting-optional">{{ t('accounting.form.optional') }}</span>
        </span>
        <select v-if="field.type === 'currency'" v-model="values[field.key]" class="input w-full" :required="field.required" @change="currencyChanged">
          <option v-for="code in currencies" :key="code" :value="code">{{ code }}</option>
        </select>
        <select v-else-if="field.type === 'rate'" v-model="values[field.key]" class="input w-full" :required="field.required">
          <option v-for="rate in rates" :key="rate" :value="rate">{{ rate }}%</option>
        </select>
        <template v-else-if="field.type === 'vendor'">
          <select v-model="values[field.key]" class="input w-full" :required="field.required" :data-testid="`entry-vendor-${field.key}`">
            <option value="">{{ t('accounting.form.select') }}</option>
            <option v-for="vendor in vendorOptions(field.vendorKind!, values[field.key] || '')" :key="vendor.id" :value="vendor.id">
              {{ vendor.name }}{{ vendor.status === 'ACTIVE' ? '' : ` (${t('accounting.vendors.disabled')})` }}
            </option>
          </select>
          <p v-if="unclassified" class="accounting-field-warning">{{ t('accounting.vendors.unclassified', { name: unclassified }) }}</p>
          <p v-if="!vendorOptions(field.vendorKind!, '').length" class="accounting-field-warning">
            {{ t('accounting.vendors.emptyList') }}
            <router-link class="accounting-link" :to="{ name: 'AdminAccountingVendorNew', query: { kind: field.vendorKind, return_to: route.fullPath } }">
              {{ t(`accounting.vendors.add.${field.vendorKind}`) }}
            </router-link>
          </p>
        </template>
        <select v-else-if="field.type === 'card'" v-model="values[field.key]" class="input w-full" :required="field.required">
          <option value="">{{ t('accounting.form.select') }}</option>
          <option v-for="card in cards" :key="card.id" :value="card.id">{{ cardLabel(card) }} · {{ card.currency }}</option>
        </select>
        <template v-else-if="field.type === 'receiving-account'">
          <select v-if="accounts.length" v-model="values[field.key]" class="input w-full" :required="field.required">
            <option value="">{{ t('accounting.form.select') }}</option>
            <option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.account_name }} · {{ account.bank_name }} · {{ account.bank_account }}</option>
          </select>
          <p v-else class="accounting-field-warning">
            {{ t('accounting.form.noReceivingAccount', { currency: values.currency }) }}
            <router-link class="accounting-link" :to="{ name: 'AdminAccountingReceivingAccountNew', query: { currency: values.currency, return_to: route.fullPath } }">
              {{ t('accounting.form.configureReceivingAccount') }}
            </router-link>
          </p>
        </template>
        <span v-else-if="field.type === 'money'" class="accounting-money-input">
          <input v-model="values[field.key]" type="text" class="input w-full tabular-nums" inputmode="decimal" :pattern="moneyPattern" :required="field.required" autocomplete="off" />
          <span aria-hidden="true">{{ moneyCurrency }}</span>
        </span>
        <textarea v-else-if="field.type === 'textarea'" v-model="values[field.key]" class="input min-h-16 w-full resize-y" maxlength="4000" />
        <input
          v-else-if="field.type === 'card-id'"
          v-model="values[field.key]"
          class="input w-full font-mono"
          maxlength="128"
          pattern="\\S+"
          autocomplete="off"
          :required="field.required"
        />
        <input
          v-else-if="field.type === 'last-four'"
          v-model="values[field.key]"
          class="input w-full font-mono"
          inputmode="numeric"
          maxlength="4"
          pattern="[0-9]{4}"
          autocomplete="off"
          :required="field.required"
        />
        <input v-else v-model="values[field.key]" :type="field.type === 'text' ? 'text' : field.type" class="input w-full" :required="field.required" autocomplete="off" />
      </label>
    </template>
    <div v-if="definition.file && !record" class="sm:col-span-2 lg:col-span-3">
      <span class="input-label">{{ t('accounting.entries.file') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
      <AccountingFileUpload v-model="file" />
    </div>
    <div class="accounting-entry-actions sm:col-span-2 lg:col-span-3">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="emit('cancel')">{{ t('common.cancel') }}</button>
      <button type="submit" class="btn btn-primary btn-sm" :disabled="busy" data-testid="entry-save">
        <span v-if="busy" class="accounting-spinner" aria-hidden="true" />
        {{ busy ? t('accounting.form.submitting') : t('common.save') }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AccountingFileUpload from '../AccountingFileUpload.vue'
import * as api from '../../api'
import { activeCards, cardLabel, entryDefinitions, taxRates, type EntryContext, type EntryValues } from '../../orderEntries'
import { commonCurrencies, currencyDigits, errorMessage } from '../../shared'
import type { AccountingRecord, OrderItemKind, VendorKind } from '../../types'

// One inline form for every kind of order entry. Saving makes the entry effective; a chosen file is attached
// right after, and a failed upload leaves the entry saved with the file still to add.
const props = defineProps<{ kind: OrderItemKind; context: EntryContext; record?: AccountingRecord; preset?: EntryValues }>()
const emit = defineEmits<{ saved: [record: AccountingRecord, uploadFailed: boolean]; cancel: []; dirty: [boolean] }>()
const { t, te } = useI18n()
const route = useRoute()
const definition = computed(() => entryDefinitions[props.kind])
const fields = computed(() => definition.value.fields.filter((field) => !(props.record && field.createOnly)))
const values = reactive<EntryValues>({ ...(props.record ? definition.value.values(props.record) : definition.value.defaults(props.context)), ...props.preset })
const baseline = JSON.stringify(values)
const file = ref<File | null>(null)
const busy = ref(false)
const error = ref('')
const errorRef = ref<HTMLElement>()
const requestKey = api.stableRequest()

const rates = computed(() => (taxRates.includes(values.tax_rate) || !values.tax_rate ? taxRates : [...taxRates, values.tax_rate]))
const currencies = computed(() => (commonCurrencies.includes(values.currency) || !values.currency ? commonCurrencies : [...commonCurrencies, values.currency]))
const cards = computed(() => {
  const active = activeCards(props.context)
  const current = props.context.items['vcc-cards'].find((card) => card.id === values.parent_id)
  return current && !active.includes(current) ? [...active, current] : active
})
const accounts = computed(() => props.context.receivingAccounts.filter((account) => account.currency === values.currency))
// Disabled vendors only appear for records that already chose them.
function vendorOptions(kind: VendorKind, current: string) {
  return props.context.vendors.filter((vendor) => vendor.kind === kind && (vendor.status === 'ACTIVE' || vendor.id === current))
}
// Older records may name a vendor in free text that matched no list entry; show it so the right one gets picked.
const legacyNameKey: Partial<Record<OrderItemKind, string>> = { 'vcc-cards': 'provider', 'vcc-transactions': 'merchant', 'vendor-invoices': 'vendor' }
const unclassified = computed(() => {
  const key = legacyNameKey[props.kind]
  const field = definition.value.fields.find((item) => item.type === 'vendor')
  if (!props.record || !key || !field || values[field.key]) return ''
  return String(props.record.details[key] || '')
})
const moneyCurrency = computed(() => (props.kind === 'vcc-transactions' ? cards.value.find((card) => card.id === values.parent_id)?.currency || '' : values.currency))
const moneyPattern = computed(() => {
  const digits = currencyDigits(moneyCurrency.value || 'CNY')
  return digits ? `\\d+(\\.\\d{1,${digits}})?` : '\\d+'
})

watch(values, () => emit('dirty', JSON.stringify(values) !== baseline || Boolean(file.value)), { deep: true })
watch(file, () => emit('dirty', true))

function currencyChanged() {
  if (props.kind === 'transfers' && !props.record) {
    values.receiving_account_id = accounts.value.length === 1 ? accounts.value[0]!.id : ''
  }
}

async function submit(event: Event) {
  const form = event.target as HTMLFormElement
  form.classList.add('was-validated')
  if (!form.checkValidity()) {
    form.reportValidity()
    return
  }
  busy.value = true
  error.value = ''
  try {
    const input = definition.value.payload(values, props.context, props.record)
    const path = props.record ? `${props.kind}/${props.record.id}` : props.kind
    const saved = props.record
      ? await api.updateEntry(props.context.order.id, props.kind, props.record.id, { ...input, version: props.record.version }, requestKey(path, input))
      : await api.addEntry(props.context.order.id, props.kind, input, requestKey(path, input))
    let uploadFailed = false
    if (file.value) {
      try {
        await api.upload(file.value, props.kind, saved)
      } catch {
        uploadFailed = true
      }
    }
    requestKey.reset()
    emit('saved', saved, uploadFailed)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
    await nextTick()
    errorRef.value?.focus()
  } finally {
    busy.value = false
  }
}
</script>
