<template>
  <div class="space-y-5" data-testid="accounting-overview">
    <div v-if="summaries.length" class="grid gap-3 lg:grid-cols-2">
      <section v-for="entry in summaries" :key="entry.currency" class="card p-5" :aria-label="entry.currency">
        <h2 class="text-sm font-semibold text-gray-500">{{ entry.currency }}</h2>
        <dl class="mt-3 grid grid-cols-2 gap-4 sm:grid-cols-3">
          <div v-for="metric in metrics" :key="metric">
            <dt class="text-xs text-gray-500 dark:text-dark-300">{{ t(`accounting.metrics.${metric}`) }}</dt>
            <dd class="mt-1 break-all text-lg font-semibold tabular-nums">
              {{ formatMoney(entry[metric], entry.currency) }}
            </dd>
          </div>
        </dl>
      </section>
    </div>
    <p v-else class="card flex min-h-48 items-center justify-center p-8 text-center text-sm text-gray-500">{{ t('accounting.overview.emptyCustomer') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatMoney } from '../format'
import type { CurrencySummary } from '../types'

// The organization's figures per currency: what it paid, what its cards spent and what came back as refunds.
defineProps<{ summaries: CurrencySummary[] }>()
const { t } = useI18n()
const metrics = ['received', 'consumed', 'refunded'] as const
</script>
