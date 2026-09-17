<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-5">
      <header class="border-b border-gray-200 pb-5 dark:border-dark-700">
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

      <div v-else class="space-y-4">
        <article
          v-for="account in accounts"
          :key="account.id"
          :aria-labelledby="`subscription-account-${account.id}`"
          class="min-w-0 overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700"
        >
          <div class="flex flex-col items-start justify-between gap-3 border-b border-primary-100 bg-primary-50/80 px-5 py-4 sm:flex-row sm:items-center dark:border-primary-900/60 dark:bg-primary-950/30">
            <div class="flex min-w-0 w-full flex-wrap items-center gap-2 sm:w-auto sm:flex-1">
              <h2
                :id="`subscription-account-${account.id}`"
                class="min-w-0 break-words text-base font-semibold text-gray-900 dark:text-white"
              >
                {{ account.name }}
              </h2>
              <AccountStatusIndicator :account="account" readonly />
            </div>
            <PlatformTypeBadge
              class="w-full sm:w-auto sm:max-w-sm sm:shrink-0"
              layout="inline"
              :platform="account.platform"
              :type="account.type"
              :auth-mode="account.auth_mode"
              :plan-type="account.plan_type"
              :privacy-mode="account.privacy_mode"
              :subscription-expires-at="account.subscription_expires_at"
            >
              <template v-if="account.openai_compact_state" #details>
                <span :class="['inline-flex items-center gap-1.5 text-[11px] font-medium leading-4', compactMeta[account.openai_compact_state].className]">
                  <span :class="['h-1.5 w-1.5 shrink-0 rounded-full', compactMeta[account.openai_compact_state].dotClass]" aria-hidden="true" />
                  <span>{{ t(compactMeta[account.openai_compact_state].label) }}</span>
                </span>
              </template>
            </PlatformTypeBadge>
          </div>

          <div class="grid min-w-0 lg:grid-cols-[minmax(0,1fr)_18rem]">
            <section class="min-w-0 px-5 py-5">
              <h3 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('subscriptionAccounts.usage') }}</h3>
              <SubscriptionAccountUsage :account="account" v-model:usage="account.usage" @status-changed="refreshAccountStatus" />
            </section>

            <dl class="grid grid-cols-2 gap-4 border-t border-gray-100 bg-gray-50/70 px-5 py-5 text-sm lg:grid-cols-1 lg:border-l lg:border-t-0 dark:border-dark-700 dark:bg-dark-900/30">
              <div class="min-w-0">
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.capacity') }}</dt>
                <dd class="mt-1" :title="t('subscriptionAccounts.currentConcurrency')">
                  <span
                    :class="[
                      'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-medium',
                      (account.current_concurrency ?? 0) > 0
                        ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
                        : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                    ]"
                  >
                    <Icon name="grid" size="xs" aria-hidden="true" />
                    <span class="font-mono tabular-nums">{{ account.current_concurrency ?? '-' }}</span>
                  </span>
                </dd>
              </div>
              <div class="min-w-0">
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.lastUsed') }}</dt>
                <dd class="mt-1 break-words tabular-nums text-gray-800 dark:text-gray-200">{{ formatOptionalDate(account.last_used_at) }}</dd>
              </div>
              <div class="min-w-0">
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.createdAt') }}</dt>
                <dd class="mt-1 break-words tabular-nums text-gray-800 dark:text-gray-200">{{ formatOptionalDate(account.created_at) }}</dd>
              </div>
              <div class="min-w-0">
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.expiresAt') }}</dt>
                <dd class="mt-1 break-words tabular-nums text-gray-800 dark:text-gray-200">{{ formatExpiration(account.expires_at) }}</dd>
              </div>
            </dl>
          </div>
        </article>
      </div>
      <TiboResetMonitor v-if="hasOpenAISubscriptionAccount" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import subscriptionAccountsAPI from '@/api/subscriptionAccounts'
import { useSubscriptionAccountAccess } from '@/composables/useSubscriptionAccountAccess'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import SubscriptionAccountUsage from '@/components/account/SubscriptionAccountUsage.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import TiboResetMonitor from '@/components/account/TiboResetMonitor.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import type { SubscriptionAccount } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const { setSubscriptionAccountAccess } = useSubscriptionAccountAccess()
const accounts = ref<SubscriptionAccount[]>([])
const loading = ref(true)

const hasOpenAISubscriptionAccount = computed(() =>
  accounts.value.some((account) => account.platform === 'openai' && account.type === 'oauth'),
)

const compactMeta = {
  active: {
    label: 'admin.accounts.openai.compactSupported',
    className: 'text-emerald-600 dark:text-emerald-300',
    dotClass: 'bg-emerald-500',
  },
  blocked: {
    label: 'admin.accounts.openai.compactUnsupported',
    className: 'text-rose-600 dark:text-rose-300',
    dotClass: 'bg-rose-500',
  },
  auto: {
    label: 'admin.accounts.openai.compactAuto',
    className: 'text-slate-500 dark:text-slate-400',
    dotClass: 'bg-slate-300 dark:bg-slate-500',
  },
}

function formatOptionalDate(value: string | null): string {
  return value ? formatDateTimeToMinute(value) : t('common.time.never')
}

function formatExpiration(value: number | null): string {
  return value ? formatDateTimeToMinute(new Date(value * 1000)) : t('subscriptionAccounts.noExpiration')
}

async function refreshAccountStatus(): Promise<void> {
  try {
    const refreshed = await subscriptionAccountsAPI.list()
    accounts.value = refreshed.map((account) => ({
      ...account,
      usage: accounts.value.find((current) => current.id === account.id)?.usage,
    }))
    setSubscriptionAccountAccess(accounts.value.length > 0)
  } catch {
    appStore.showError(t('subscriptionAccounts.failedToLoad'))
  }
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
