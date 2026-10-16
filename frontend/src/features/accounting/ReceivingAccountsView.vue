<template>
  <AppLayout>
    <main class="mx-auto w-full max-w-6xl pb-10" data-testid="receiving-accounts-page">
      <button
        type="button"
        class="accounting-back"
        @click="router.push({ name: 'AdminAccounting' })"
      >
        <Icon name="arrowLeft" size="sm" aria-hidden="true" />
        {{ t('accounting.directory.title') }}
      </button>

      <header class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-200 pb-6 dark:border-dark-700">
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">
            {{ t('accounting.receivingAccounts.title') }}
          </h1>
          <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-dark-300">
            {{ t('accounting.receivingAccounts.description') }}
          </p>
        </div>
        <router-link class="btn btn-primary" :to="{ name: 'AdminAccountingReceivingAccountNew' }">
          <Icon name="plus" size="sm" aria-hidden="true" />
          {{ t('accounting.receivingAccounts.create') }}
        </router-link>
      </header>

      <p v-if="error" role="alert" class="mt-6 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
        {{ error }}
      </p>
      <p v-if="loading" role="status" class="py-10 text-sm text-gray-500">
        {{ t('accounting.loading') }}
      </p>

      <div v-else-if="accounts.length" class="mt-6 overflow-x-auto border-y border-gray-200 dark:border-dark-700">
        <table class="w-full text-left text-sm">
          <thead class="text-xs text-gray-500 dark:text-dark-400">
            <tr class="border-b border-gray-200 dark:border-dark-700">
              <th class="px-3 py-3 font-medium">{{ t('accounting.currency') }}</th>
              <th class="px-3 py-3 font-medium">{{ t('accounting.receivingAccountForm.accountName') }}</th>
              <th class="px-3 py-3 font-medium">{{ t('accounting.receivingAccountForm.bankName') }}</th>
              <th class="px-3 py-3 font-medium">{{ t('accounting.receivingAccountForm.bankAccount') }}</th>
              <th class="px-3 py-3 text-right font-medium">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="account in accounts" :key="account.id" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
              <td class="px-3 py-4 align-top font-semibold text-gray-900 dark:text-white">{{ account.currency }}</td>
              <td class="px-3 py-4 align-top">{{ account.account_name }}</td>
              <td class="px-3 py-4 align-top">
                <div>{{ account.bank_name }}</div>
                <div v-if="account.bank_branch" class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ account.bank_branch }}</div>
              </td>
              <td class="px-3 py-4 align-top font-mono">{{ account.bank_account }}</td>
              <td class="px-3 py-4 text-right align-top">
                <router-link
                  class="detail-row-action"
                  :to="{ name: 'AdminAccountingReceivingAccountEdit', params: { accountId: account.id } }"
                  :title="t('common.edit')"
                  :aria-label="`${t('common.edit')} ${account.currency} ${account.account_name}`"
                >
                  <Icon name="edit" size="sm" />
                </router-link>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else class="py-16 text-center">
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
import * as api from './api'
import type { ReceivingAccount } from './types'
import '@/styles/accounting.css'

const { t } = useI18n()
const router = useRouter()
const accounts = ref<ReceivingAccount[]>([])
const loading = ref(true)
const error = ref('')

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
