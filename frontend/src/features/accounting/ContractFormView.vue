<template>
  <FormPage
    :title="t(isEditing ? 'accounting.contractForm.editTitle' : 'accounting.contractForm.createTitle')"
    :subtitle="t('accounting.contractForm.description')"
    :back-label="t('common.back')"
    :loading="loading"
    :error="error"
    :busy="busy"
    @submit="save"
    @cancel="leave(returnLocation())"
  >
    <FormSection :title="t('accounting.contractForm.contract')">
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.fields.name') }}</span>
        <input v-model="form.name" class="input w-full" maxlength="200" required data-testid="contract-name" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.fields.contract_number') }}</span>
        <input v-model="form.contract_number" class="input w-full font-mono" maxlength="100" required data-testid="contract-number" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.fields.signed_at') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input v-model="form.signed_at" type="date" class="input w-full" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.fields.effective_at') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input v-model="form.effective_at" type="date" class="input w-full" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.fields.expires_at') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input v-model="form.expires_at" type="date" class="input w-full" :min="form.effective_at || undefined" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.currency') }}</span>
        <select v-model="form.currency" class="input w-full">
          <option v-for="code in currencies" :key="code" :value="code">{{ code }}</option>
        </select>
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.contractForm.amount') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input v-model="form.amount" class="input w-full tabular-nums" inputmode="decimal" :pattern="moneyPattern" autocomplete="off" data-testid="contract-amount" />
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.fields.contact_name') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <select v-model="form.contact_id" class="input w-full">
          <option value="">{{ t('accounting.form.select') }}</option>
          <option v-for="contact in contacts" :key="contact.id" :value="contact.id">{{ contact.name }}{{ contact.phone ? ` · ${contact.phone}` : '' }}</option>
        </select>
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('admin.desktop.business.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <textarea v-model="form.remark" class="input min-h-20 w-full resize-y" maxlength="4000" />
      </label>
      <label class="flex items-center gap-2 text-sm sm:col-span-2">
        <input v-model="form.customer_visible" type="checkbox" class="h-4 w-4" data-testid="contract-shared" />
        {{ t('accounting.contractForm.share') }}
      </label>
    </FormSection>

    <FormSection :title="t('accounting.contractForm.files')">
      <div class="sm:col-span-2">
        <RecordFiles v-if="record" kind="contracts" :record="record" />
        <template v-else>
          <AccountingFileUpload v-model="file" :disabled="busy" />
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('accounting.contractForm.filesHint') }}</p>
        </template>
      </div>
    </FormSection>
  </FormPage>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import FormPage from './components/FormPage.vue'
import FormSection from './components/FormSection.vue'
import RecordFiles from './components/RecordFiles.vue'
import AccountingFileUpload from './components/AccountingFileUpload.vue'
import * as api from './api'
import { commonCurrencies, currencyDigits, errorMessage, safeReturnPath } from './shared'
import { useLeaveGuard } from './useLeaveGuard'
import type { AccountingRecord, OrganizationContact } from './types'

// A sales contract is a document record of the organization: who signed what and when, with its scan. Orders
// pick the contract they are done under; the organization sees it only when shared.
const { t, te } = useI18n()
const route = useRoute()
const organizationId = computed(() => String(route.params.organizationId || ''))
const contractId = computed(() => String(route.params.contractId || ''))
const isEditing = computed(() => Boolean(contractId.value))
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const record = ref<AccountingRecord | null>(null)
const contacts = ref<OrganizationContact[]>([])
const file = ref<File | null>(null)
const form = reactive({
  name: '',
  contract_number: '',
  signed_at: '',
  effective_at: '',
  expires_at: '',
  currency: 'CNY',
  amount: '',
  contact_id: '',
  remark: '',
  customer_visible: false,
})
const requestKey = api.stableRequest()
const { leave, markClean } = useLeaveGuard(() => ({ ...form, file: file.value?.name || '' }))
const currencies = computed(() => (commonCurrencies.includes(form.currency) ? commonCurrencies : [...commonCurrencies, form.currency]))
const moneyPattern = computed(() => {
  const digits = currencyDigits(form.currency)
  return digits ? `\\d+(\\.\\d{1,${digits}})?` : '\\d+'
})

function returnLocation() {
  return (
    safeReturnPath(route.query.return_to) || {
      name: 'AdminDesktopOrganizationDetail',
      params: { organizationId: organizationId.value },
      query: { tab: 'business' },
    }
  )
}
function details() {
  const out: Record<string, string> = { name: form.name.trim(), contract_number: form.contract_number.trim(), remark: form.remark.trim() }
  for (const key of ['signed_at', 'effective_at', 'expires_at', 'contact_id'] as const) out[key] = form[key]
  return out
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    contacts.value = await api.organizationContacts(organizationId.value)
    if (route.query.upload_failed) error.value = t('accounting.contractForm.uploadFailed')
    if (isEditing.value) {
      const current = await api.getRecord('contracts', contractId.value)
      record.value = current
      const text = (key: string) => String(current.details[key] || '')
      Object.assign(form, {
        name: text('name'),
        contract_number: text('contract_number'),
        signed_at: text('signed_at'),
        effective_at: text('effective_at'),
        expires_at: text('expires_at'),
        currency: current.currency,
        amount: Number(current.amount) ? current.amount.replace(/\.?0+$/, '') : '',
        contact_id: text('contact_id'),
        remark: text('remark'),
        customer_visible: current.customer_visible,
      })
    }
    await nextTick()
    markClean()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}
async function share(id: string) {
  const path = `contracts/${id}/sharing`
  const payload = { customer_visible: form.customer_visible }
  await api.mutate(path, payload, requestKey(path, payload), 'patch')
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    const amount = form.amount.trim() || '0'
    if (record.value) {
      const path = `contracts/${record.value.id}`
      const payload = { currency: form.currency, amount, version: record.value.version, details: details() }
      await api.mutate(path, payload, requestKey(path, payload), 'patch')
      if (form.customer_visible !== record.value.customer_visible) await share(record.value.id)
    } else {
      const payload = { organization_id: organizationId.value, currency: form.currency, amount, details: details() }
      const created = (await api.mutate('contracts', payload, requestKey('contracts', payload))) as AccountingRecord
      // A contract entered here has been signed; drafts are not part of this record keeping.
      await api.mutate(`contracts/${created.id}/confirm`, {}, requestKey(`contracts/${created.id}/confirm`, {}))
      if (form.customer_visible) await share(created.id)
      requestKey.reset()
      if (file.value) {
        try {
          await api.upload(file.value, 'contracts', created)
        } catch {
          // The contract is saved; its edit page lets the scan be added again.
          await leave({ name: 'AdminAccountingContractEdit', params: { organizationId: organizationId.value, contractId: created.id }, query: { ...route.query, upload_failed: '1' } }, true)
          return
        }
      }
    }
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
