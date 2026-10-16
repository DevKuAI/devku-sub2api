<template>
  <FormPage
    :title="t(isEditing ? 'accounting.receivingAccountForm.editTitle' : 'accounting.receivingAccountForm.createTitle')"
    :subtitle="t('accounting.receivingAccounts.description')"
    :back-label="t('common.back')"
    :loading="loading"
    :error="error"
    :busy="busy"
    @submit="save"
    @cancel="leave(returnLocation())"
  >
    <FormSection :title="t('accounting.receivingAccountForm.account')">
      <label class="block">
        <span class="input-label">{{ t('accounting.currency') }}</span>
        <input
          :value="form.currency"
          class="input w-full uppercase"
          maxlength="3"
          pattern="[A-Za-z]{3}"
          required
          :disabled="isEditing"
          @input="form.currency = ($event.target as HTMLInputElement).value.toUpperCase()"
        />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.receivingAccountForm.accountName') }}</span>
        <input v-model="form.account_name" class="input w-full" maxlength="200" required autocomplete="organization" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.receivingAccountForm.bankName') }}</span>
        <input v-model="form.bank_name" class="input w-full" maxlength="200" required autocomplete="off" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.receivingAccountForm.bankBranch') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input v-model="form.bank_branch" class="input w-full" maxlength="200" autocomplete="off" />
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.receivingAccountForm.bankAccount') }}</span>
        <input v-model="form.bank_account" class="input w-full font-mono" maxlength="100" required inputmode="numeric" autocomplete="off" />
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('admin.desktop.business.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <textarea v-model="form.remark" class="input min-h-20 w-full resize-y" maxlength="1000" />
      </label>
    </FormSection>
  </FormPage>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import FormPage from './components/FormPage.vue'
import FormSection from './components/FormSection.vue'
import * as api from './api'
import { errorMessage, safeReturnPath } from './shared'
import { useLeaveGuard } from './useLeaveGuard'
import type { ReceivingAccountInput } from './types'

const { t, te } = useI18n()
const route = useRoute()
const accountId = computed(() => String(route.params.accountId || ''))
const isEditing = computed(() => Boolean(accountId.value))
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const form = reactive<ReceivingAccountInput>({
  currency: String(route.query.currency || 'CNY').toUpperCase(),
  account_name: '',
  bank_name: '',
  bank_account: '',
  bank_branch: '',
  remark: '',
})
const { leave, markClean } = useLeaveGuard(() => form)

function returnLocation() {
  return safeReturnPath(route.query.return_to) || { name: 'AdminAccountingReceivingAccounts' }
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    if (isEditing.value) {
      const account = (await api.receivingAccounts()).find((item) => item.id === accountId.value)
      if (!account) throw new Error(t('accounting.receivingAccountForm.notFound'))
      const { currency, account_name, bank_name, bank_account, bank_branch, remark } = account
      Object.assign(form, { currency, account_name, bank_name, bank_account, bank_branch, remark })
    }
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
    const payload: ReceivingAccountInput = {
      currency: form.currency.trim().toUpperCase(),
      account_name: form.account_name.trim(),
      bank_name: form.bank_name.trim(),
      bank_account: form.bank_account.trim(),
      bank_branch: form.bank_branch.trim(),
      remark: form.remark.trim(),
    }
    if (isEditing.value) await api.updateReceivingAccount(accountId.value, payload)
    else await api.createReceivingAccount(payload)
    await leave(returnLocation(), true)
  } catch (cause) {
    error.value =
      (cause as { reason?: string }).reason === 'ACCOUNTING_STATE_CONFLICT'
        ? t('accounting.receivingAccountForm.accountExists')
        : errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>
