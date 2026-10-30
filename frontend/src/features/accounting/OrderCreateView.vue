<template>
  <FormPage
    :title="t('accounting.orders.createTitle')"
    :subtitle="t('accounting.home.createHint')"
    :back-label="t('accounting.home.title')"
    :loading="loading"
    :error="error"
    :busy="busy"
    :submit-label="t('accounting.orders.create')"
    @submit="save"
    @cancel="leave(ordersRoute(form.organization_id ? { organization: form.organization_id } : {}))"
  >
    <FormSection :title="t('accounting.orders.section')">
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.home.organization') }}</span>
        <select v-model="form.organization_id" class="input w-full" required data-testid="order-organization" @change="organizationChanged">
          <option value="">{{ t('accounting.form.select') }}</option>
          <option v-for="organization in organizations" :key="organization.id" :value="organization.id">{{ organization.name }}</option>
        </select>
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.orders.contractField') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <select v-model="form.contract_id" class="input w-full" :disabled="!form.organization_id" data-testid="order-contract">
          <option value="">{{ t('accounting.orders.noContract') }}</option>
          <option v-for="contract in contracts" :key="contract.id" :value="contract.id">{{ [contract.details.name, contract.details.contract_number].filter(Boolean).join(' · ') }}</option>
        </select>
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.orders.name') }}</span>
        <input v-model="form.name" class="input w-full" maxlength="200" required data-testid="order-name" @input="named = true" />
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.entries.fields.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <textarea v-model="form.remark" class="input min-h-16 w-full resize-y" maxlength="4000" />
      </label>
    </FormSection>
  </FormPage>
</template>

<script setup lang="ts">
import { nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import FormPage from './components/FormPage.vue'
import FormSection from './components/FormSection.vue'
import * as api from './api'
import { errorMessage, localDate, orderRoute, ordersRoute } from './shared'
import { useLeaveGuard } from './useLeaveGuard'
import type { AccountingOrder, AccountingOrganization, AccountingRecord } from './types'

// An order starts with just the organization and a name; everything else is filled in on the order page.
const { t, te } = useI18n()
const route = useRoute()
const organizations = ref<AccountingOrganization[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const named = ref(false)
const form = reactive({ organization_id: typeof route.query.organization === 'string' ? route.query.organization : '', name: '', remark: '', contract_id: '' })
const contracts = ref<AccountingRecord[]>([])
// The organization's contracts are offered once it is chosen; a framework contract may cover many orders.
async function loadContracts() {
  form.contract_id = ''
  contracts.value = form.organization_id ? await api.organizationContracts(form.organization_id) : []
}
function organizationChanged() {
  suggestName()
  loadContracts().catch((cause) => (error.value = errorMessage(cause, t, te)))
}
const { leave, markClean } = useLeaveGuard(() => form)
const requestKey = api.stableRequest()

function suggestName() {
  if (named.value) return
  const organization = organizations.value.find((item) => item.id === form.organization_id)
  form.name = organization ? `${organization.name} ${localDate()}` : ''
}
async function load() {
  try {
    organizations.value = await api.organizations({})
    suggestName()
    await loadContracts()
    await nextTick()
    markClean()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    const payload = { organization_id: form.organization_id, name: form.name.trim(), remark: form.remark.trim(), ...(form.contract_id ? { contract_id: form.contract_id } : {}) }
    const order = (await api.mutate('orders', payload, requestKey('orders', payload))) as AccountingOrder
    await leave(orderRoute(order.id), true)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>
