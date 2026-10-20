<template>
  <AppLayout>
    <main class="accounting-page mx-auto w-full max-w-5xl space-y-5 pb-10" data-testid="receiving-accounts-page">
      <PageHeader
        :title="t('accounting.receivingAccounts.title')"
        :subtitle="t('accounting.receivingAccounts.description')"
        :back="{ to: { name: 'AdminAccounting' }, label: t('accounting.directory.title') }"
      >
        <template #actions>
          <router-link class="btn btn-primary" :to="{ name: 'AdminAccountingReceivingAccountNew' }">
            <Icon name="plus" size="sm" aria-hidden="true" />
            {{ t('accounting.receivingAccounts.create') }}
          </router-link>
        </template>
      </PageHeader>

      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="loading" role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>

      <div v-else-if="accounts.length" class="card overflow-x-auto">
        <table class="accounting-table">
          <thead>
            <tr>
              <th>{{ t('accounting.currency') }}</th>
              <th>{{ t('accounting.receivingAccountForm.accountName') }}</th>
              <th>{{ t('accounting.receivingAccountForm.bankName') }}</th>
              <th>{{ t('accounting.receivingAccountForm.bankAccount') }}</th>
              <th class="text-right">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="account in accounts" :key="account.id" class="accounting-clickable-row" @click="edit(account)">
              <td><span class="badge badge-gray font-semibold">{{ account.currency }}</span></td>
              <td class="font-medium text-gray-900 dark:text-white">{{ account.account_name }}</td>
              <td>
                <p>{{ account.bank_name }}</p>
                <p v-if="account.bank_branch" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ account.bank_branch }}</p>
              </td>
              <td class="font-mono tabular-nums">{{ account.bank_account }}</td>
              <td class="text-right">
                <router-link
                  class="accounting-link text-sm"
                  :to="editRoute(account)"
                  :aria-label="`${t('common.edit')} ${account.currency} ${account.account_name}`"
                  @click.stop
                >
                  {{ t('common.edit') }}
                </router-link>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else class="card p-10 text-center">
        <p class="font-medium text-gray-900 dark:text-white">{{ t('accounting.receivingAccounts.empty') }}</p>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">{{ t('accounting.receivingAccounts.emptyHint') }}</p>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'
import PageHeader from './components/PageHeader.vue'
import * as api from './api'
import type { ReceivingAccount } from './types'
import '@/styles/accounting.css'

const { t } = useI18n()
const router = useRouter()
const accounts = ref<ReceivingAccount[]>([])
const loading = ref(true)
const error = ref('')

function editRoute(account: ReceivingAccount) {
  return { name: 'AdminAccountingReceivingAccountEdit', params: { accountId: account.id } }
}
function edit(account: ReceivingAccount) {
  router.push(editRoute(account))
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    accounts.value = await api.receivingAccounts()
  } catch (value) {
    error.value = (value as { message?: string }).message || t('accounting.errors.generic')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
