<template>
  <AppLayout>
    <div class="accounting-page min-w-0 space-y-6" data-testid="accounting-channels">
      <AccountingNav current="AdminAccountingChannels" />

      <div class="accounting-toolbar">
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('accounting.channels.description') }}</p>
        <router-link class="btn btn-primary ml-auto" :to="{ name: 'AdminAccountingChannelImport' }" data-testid="channel-import-open">
          <Icon name="upload" size="md" class="mr-1" aria-hidden="true" />{{ t('accounting.channels.importAction') }}
        </router-link>
      </div>

      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="loading" role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>

      <template v-else-if="accounts.length">
        <section v-for="summary in accounts" :key="summary.account.id" class="card channel-account" :data-testid="`channel-account-${summary.account.account_ref}`">
          <header class="channel-account-header">
            <div class="min-w-0">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ summary.account.name || summary.account.account_ref }}</h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                {{ summary.account.platform }} · {{ summary.account.account_ref }}
                <template v-if="summary.last_import_at"> · {{ t('accounting.channels.lastImport', { date: formatDateTime(summary.last_import_at) }) }}</template>
              </p>
            </div>
            <router-link class="accounting-link text-sm" :to="{ name: 'AdminAccountingChannel', params: { accountId: summary.account.id } }">
              {{ t('accounting.channels.view') }}
            </router-link>
          </header>

          <WalletFlow :wallets="summary.wallets" />

          <dl class="channel-figures">
            <div>
              <dt>{{ t('accounting.channels.positions') }}</dt>
              <dd v-for="position in summary.positions" :key="position.currency" class="tabular-nums">{{ formatMoney(position.amount, position.currency) }}</dd>
            </div>
            <div v-if="summary.fee_costs.length">
              <dt>{{ t('accounting.channels.feeCosts') }}</dt>
              <dd v-for="fee in summary.fee_costs" :key="fee.currency" class="tabular-nums">{{ formatMoney(fee.amount, fee.currency) }}</dd>
            </div>
            <div>
              <dt>{{ t('accounting.channels.detail.cardSpend') }}</dt>
              <dd>{{ t('accounting.channels.cards', { count: summary.cards }) }}</dd>
            </div>
          </dl>

          <ul v-if="warnings(summary).length" class="channel-warnings">
            <li v-for="warning in warnings(summary)" :key="warning" class="badge badge-warning">{{ warning }}</li>
          </ul>
          <p v-else class="text-sm text-emerald-700 dark:text-emerald-400">{{ t('accounting.channels.allClear') }}</p>
        </section>
      </template>

      <div v-else class="card p-10 text-center">
        <p class="font-medium text-gray-900 dark:text-white">{{ t('accounting.channels.empty') }}</p>
        <p class="mt-2 text-sm text-gray-500">{{ t('accounting.channels.emptyHint') }}</p>
        <router-link class="btn btn-primary mt-4" :to="{ name: 'AdminAccountingChannelImport' }">{{ t('accounting.channels.importAction') }}</router-link>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'
import AccountingNav from './components/AccountingNav.vue'
import WalletFlow from './components/channel/WalletFlow.vue'
import * as api from './api'
import { formatMoney } from './format'
import { errorMessage, formatDateTime } from './shared'
import type { ChannelAccountSummary } from './types'
import '@/styles/accounting.css'

// Every channel account with its balances against the platform, so a glance shows whether the books agree.
const { t, te } = useI18n()
const accounts = ref<ChannelAccountSummary[]>([])
const loading = ref(true)
const error = ref('')

function warnings(summary: ChannelAccountSummary) {
  const out: string[] = []
  const gaps = summary.wallets.reduce((count, wallet) => count + wallet.gaps.length, 0)
  if (gaps) out.push(t('accounting.channels.gaps', { count: gaps }))
  if (summary.unpaired_transfers) out.push(t('accounting.channels.unpaired', { count: summary.unpaired_transfers }))
  if (summary.missing_proofs) out.push(t('accounting.channels.proofs.missingCount', { count: summary.missing_proofs }))
  if (summary.wallets.some((wallet) => wallet.reported && !wallet.reported.matched)) out.push(t('accounting.channels.mismatched'))
  if (summary.wallets.some((wallet) => wallet.negative_spans.length)) out.push(t('accounting.channels.negative'))
  return out
}

onMounted(async () => {
  try {
    accounts.value = await api.channelAccounts()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
})
</script>
