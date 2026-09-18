<template>
  <AppLayout>
    <div class="mx-auto min-w-0 max-w-6xl space-y-6">
      <header class="space-y-2">
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('subscriptionAccounts.title') }}</h1>
        <p class="max-w-2xl text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('subscriptionAccounts.description') }}</p>
      </header>

      <div v-if="loading" class="flex items-center justify-center gap-3 py-16 text-sm text-gray-600 dark:text-dark-300" role="status">
        <div class="h-6 w-6 rounded-full border-2 border-primary-500 border-t-transparent motion-safe:animate-spin" aria-hidden="true"></div>
        <span>{{ t('common.loading') }}</span>
      </div>

      <div v-else-if="loadError" class="rounded-xl border border-red-200 bg-red-50 p-6 dark:border-red-900/60 dark:bg-red-900/10" role="alert">
        <p class="text-sm text-red-800 dark:text-red-300">{{ t('subscriptionAccounts.failedToLoad') }}</p>
        <button type="button" class="btn btn-secondary mt-4" @click="loadAccounts">{{ t('subscriptionAccounts.retry') }}</button>
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
          class="min-w-0 rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700"
        >
          <header class="grid min-w-0 gap-4 rounded-t-xl border-b border-primary-100 bg-primary-50/50 px-5 py-5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start sm:px-6 dark:border-dark-700 dark:bg-primary-950/20">
            <div class="min-w-0 space-y-3">
              <h2
                :id="`subscription-account-${account.id}`"
                class="break-words text-base font-semibold leading-6 text-gray-900 dark:text-white [overflow-wrap:anywhere]"
              >
                {{ account.name }}
              </h2>
              <AccountStatusIndicator :account="account" readonly layout="inline" />
            </div>
            <PlatformTypeBadge
              class="min-w-0 max-w-full sm:max-w-xs sm:items-end"
              layout="inline"
              :platform="account.platform"
              :type="account.type"
              :auth-mode="account.auth_mode"
              :plan-type="account.plan_type"
              :privacy-mode="account.privacy_mode"
              :subscription-expires-at="account.subscription_expires_at"
            >
              <template v-if="account.openai_compact_state" #details>
                <span :class="['inline-flex items-center gap-1.5 text-xs font-medium leading-5', compactMeta[account.openai_compact_state].className]">
                  <span :class="['h-1.5 w-1.5 shrink-0 rounded-full', compactMeta[account.openai_compact_state].dotClass]" aria-hidden="true" />
                  <span>{{ t(compactMeta[account.openai_compact_state].label) }}</span>
                </span>
              </template>
            </PlatformTypeBadge>
          </header>

          <div class="min-w-0">
            <section class="min-w-0 px-5 py-5 sm:px-6">
              <h3 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('subscriptionAccounts.usage') }}</h3>
              <SubscriptionAccountUsage :account="account" v-model:usage="account.usage" @status-changed="refreshAccountStatus" />
            </section>

            <dl class="grid min-w-0 grid-cols-2 gap-x-6 gap-y-4 rounded-b-xl border-t border-gray-100 bg-gray-50/70 px-5 py-4 text-sm sm:px-6 xl:grid-cols-4 dark:border-dark-700 dark:bg-dark-900/30">
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
const loadError = ref(false)

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
  loadError.value = false
  try {
    accounts.value = await subscriptionAccountsAPI.list(true)
    setSubscriptionAccountAccess(accounts.value.length > 0)
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(loadAccounts)
</script>
