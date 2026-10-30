<template>
  <ol class="channel-flow" :aria-label="t('accounting.channels.detail.flow')">
    <li v-for="(wallet, index) in wallets" :key="`${wallet.wallet}-${wallet.currency}`" class="channel-flow-step">
      <span v-if="index > 0" class="channel-flow-arrow" aria-hidden="true">→</span>
      <div class="channel-wallet" :class="statusClass(wallet)" :data-testid="`channel-wallet-${wallet.wallet}-${wallet.currency}`">
        <p class="channel-wallet-label">{{ walletLabel(wallet.wallet, t, te) }} · {{ wallet.currency }}</p>
        <p class="channel-wallet-balance tabular-nums">{{ formatMoney(wallet.balance, wallet.currency) }}</p>
        <p v-if="wallet.reported" class="channel-wallet-meta">
          <span class="badge" :class="wallet.reported.matched ? 'badge-success' : 'badge-danger'">
            {{ wallet.reported.matched ? t('accounting.channels.matched') : t('accounting.channels.difference', { amount: formatMoney(wallet.reported.difference, wallet.currency) }) }}
          </span>
          <span>{{ t('accounting.channels.platform') }} {{ formatMoney(wallet.reported.amount, wallet.currency) }} · {{ t('accounting.channels.asOf', { date: formatDate(wallet.reported.as_of) }) }}</span>
        </p>
        <p v-else class="channel-wallet-meta"><span class="badge badge-gray">{{ t('accounting.channels.noReport') }}</span></p>
      </div>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { walletLabel } from '../../channelShared'
import { formatMoney } from '../../format'
import { formatDate } from '../../shared'
import type { ChannelWallet } from '../../types'

// Balances in the order money moves through them, each checked against the platform's own figure.
defineProps<{ wallets: ChannelWallet[] }>()
const { t, te } = useI18n()

function statusClass(wallet: ChannelWallet) {
  if (!wallet.reported) return 'is-unknown'
  return wallet.reported.matched && !wallet.gaps.length ? 'is-matched' : 'is-mismatched'
}
</script>
