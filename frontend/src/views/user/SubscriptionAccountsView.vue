<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <header>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('subscriptionAccounts.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.description') }}</p>
      </header>

      <div v-if="loading" class="flex justify-center py-16" role="status">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else-if="accounts.length === 0" class="border-y border-gray-200 py-12 text-center dark:border-dark-700">
        <Icon name="creditCard" size="xl" class="mx-auto text-gray-400" />
        <h2 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">{{ t('subscriptionAccounts.empty') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.emptyDescription') }}</p>
      </div>

      <div v-else class="divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700">
        <article
          v-for="account in accounts"
          :key="account.id"
          class="grid min-w-0 gap-5 py-6 md:grid-cols-[minmax(12rem,1.1fr)_minmax(14rem,1fr)_minmax(16rem,1.25fr)] md:items-start"
        >
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="truncate font-semibold text-gray-900 dark:text-white" :title="account.name">{{ account.name }}</h2>
              <span :class="['rounded-md px-2 py-0.5 text-xs font-medium', statusClass(account.status)]">
                {{ t(`subscriptionAccounts.status.${account.status}`) }}
              </span>
            </div>
            <div class="mt-2">
              <PlatformTypeBadge :platform="account.platform" :type="account.type" />
            </div>
          </div>

          <dl class="grid grid-cols-2 gap-x-5 gap-y-3 text-sm md:grid-cols-1">
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.lastUsed') }}</dt>
              <dd class="mt-1 text-gray-800 dark:text-gray-200">{{ formatOptionalDate(account.last_used_at) }}</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.createdAt') }}</dt>
              <dd class="mt-1 text-gray-800 dark:text-gray-200">{{ formatOptionalDate(account.created_at) }}</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.expiresAt') }}</dt>
              <dd class="mt-1 text-gray-800 dark:text-gray-200">{{ formatExpiration(account.expires_at) }}</dd>
            </div>
          </dl>

          <div class="min-w-0">
            <h3 class="mb-2 text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.usage') }}</h3>
            <SubscriptionAccountUsage :usage="account.usage" />
          </div>
        </article>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import subscriptionAccountsAPI from '@/api/subscriptionAccounts'
import { useSubscriptionAccountAccess } from '@/composables/useSubscriptionAccountAccess'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import SubscriptionAccountUsage from '@/components/account/SubscriptionAccountUsage.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import type { SubscriptionAccount } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const { setSubscriptionAccountAccess } = useSubscriptionAccountAccess()
const accounts = ref<SubscriptionAccount[]>([])
const loading = ref(true)

function formatOptionalDate(value: string | null): string {
  return value ? formatDateTimeToMinute(value) : t('common.dateTime.never')
}

function formatExpiration(value: number | null): string {
  return value ? formatDateTimeToMinute(new Date(value * 1000)) : t('subscriptionAccounts.noExpiration')
}

function statusClass(status: SubscriptionAccount['status']): string {
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
  if (status === 'error') return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

async function loadAccounts(): Promise<void> {
  loading.value = true
  try {
    accounts.value = await subscriptionAccountsAPI.list(true)
    setSubscriptionAccountAccess(accounts.value.length > 0)
  } catch {
    appStore.showError(t('subscriptionAccounts.failedToLoad'))
  } finally {
    loading.value = false
  }
}

onMounted(loadAccounts)
</script>
