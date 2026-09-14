<template>
  <section class="overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700" data-testid="tibo-reset-monitor">
    <header class="flex flex-wrap items-start justify-between gap-3 border-b border-teal-100 bg-teal-50/70 px-5 py-4 dark:border-teal-900/50 dark:bg-teal-950/20">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('subscriptionAccounts.tiboReset.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.tiboReset.description') }}</p>
      </div>
      <span v-if="monitor?.checkedAt" class="text-xs text-gray-400 dark:text-dark-500">
        {{ t('subscriptionAccounts.tiboReset.updatedAt', { time: formatDate(monitor.checkedAt) }) }}
      </span>
    </header>

    <div v-if="loading" class="px-5 py-5" role="status">
      <div class="h-24 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
    </div>

    <div v-else-if="monitor" class="px-5 py-5">
      <section class="min-w-0 rounded-lg border border-gray-200 px-4 py-4 dark:border-dark-700">
        <p class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.tiboReset.latest') }}</p>
        <p v-if="latest" class="mt-2 text-sm text-emerald-700 dark:text-emerald-300">{{ latest.title }}</p>
        <p v-if="latest" class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ eventDate(latest) }}</p>
        <p v-if="latest?.scope" class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ latest.scope }}</p>
        <a
          v-if="latestPost?.url"
          :href="latestPost.url"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-3 inline-flex text-xs font-medium text-teal-700 hover:underline dark:text-teal-300"
        >
          {{ t('subscriptionAccounts.tiboReset.viewPost') }}
        </a>
        <p v-else class="mt-2 text-sm text-gray-400 dark:text-dark-500">{{ t('subscriptionAccounts.tiboReset.noData') }}</p>
      </section>

    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import subscriptionAccountsAPI from '@/api/subscriptionAccounts'
import { formatDateTimeToMinute } from '@/utils/format'
import type { TiboResetMonitor, TiboResetMonitorEvent } from '@/types'

const { t } = useI18n()
const loading = ref(true)
const monitor = ref<TiboResetMonitor | null>(null)

function eventTime(event: TiboResetMonitorEvent): number {
  const value = event.confirmedAt || event.occurredOn || event.updatedAt || event.createdAt
  const timestamp = value ? new Date(value).getTime() : 0
  return Number.isFinite(timestamp) ? timestamp : 0
}

const latest = computed(() => {
  const events = monitor.value?.events || []
  const confirmed = events.filter((event) => event.status === 'confirmed')
  const directResets = confirmed.filter((event) => event.type === 'direct_reset')
  return [...(directResets.length > 0 ? directResets : confirmed)]
    .sort((a, b) => eventTime(b) - eventTime(a))[0] || null
})

const latestPost = computed(() => latest.value?.posts?.[0] || null)

function formatDate(value: string): string {
  return formatDateTimeToMinute(value)
}

function eventDate(event: TiboResetMonitorEvent): string {
  const value = event.confirmedAt || event.occurredOn || event.updatedAt || event.createdAt
  return value ? formatDate(value) : t('subscriptionAccounts.tiboReset.unknown')
}

onMounted(async () => {
  try {
    monitor.value = await subscriptionAccountsAPI.getTiboResetMonitor()
  } catch {
    monitor.value = null
  } finally {
    loading.value = false
  }
})
</script>
