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
        <select v-model="form.organization_id" class="input w-full" required data-testid="order-organization" @change="suggestName">
          <option value="">{{ t('accounting.form.select') }}</option>
          <option v-for="organization in organizations" :key="organization.id" :value="organization.id">{{ organization.name }}</option>
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
import type { AccountingOrder, AccountingOrganization } from './types'

// An order starts with just the organization and a name; everything else is filled in on the order page.
const { t, te } = useI18n()
const route = useRoute()
const organizations = ref<AccountingOrganization[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const named = ref(false)
const form = reactive({ organization_id: typeof route.query.organization === 'string' ? route.query.organization : '', name: '', remark: '' })
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
    const payload = { organization_id: form.organization_id, name: form.name.trim(), remark: form.remark.trim() }
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
