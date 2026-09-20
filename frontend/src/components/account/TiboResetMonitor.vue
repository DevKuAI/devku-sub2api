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

    <div v-else-if="monitor" class="grid gap-5 px-5 py-5 sm:grid-cols-2">
      <section
        class="min-w-0 rounded-lg border border-gray-200 px-4 py-4"
        :class="{ 'sm:col-span-2': !latestCredit }"
      >
        <div class="flex flex-wrap items-center gap-2">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('subscriptionAccounts.tiboReset.latest') }}</p>
          <span v-if="latest" class="rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
            {{ t('subscriptionAccounts.tiboReset.directReset') }}
          </span>
        </div>
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
        <p v-if="!latest" class="mt-2 text-sm text-gray-400 dark:text-dark-500">{{ t('subscriptionAccounts.tiboReset.noData') }}</p>
      </section>

      <section v-if="latestCredit" class="min-w-0 rounded-lg border border-gray-200 px-4 py-4">
        <div class="flex flex-wrap items-center gap-2">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t(creditAnnounced ? 'subscriptionAccounts.tiboReset.cardForecast' : 'subscriptionAccounts.tiboReset.latestCard') }}</p>
          <span class="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
            {{ t(creditAnnounced ? 'subscriptionAccounts.tiboReset.cardAnnouncement' : 'subscriptionAccounts.tiboReset.resetCard') }}
          </span>
        </div>
        <p class="mt-2 text-sm text-amber-700 dark:text-amber-300">{{ latestCredit.title }}</p>
        <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ eventDate(latestCredit) }}</p>
        <p v-if="creditAnnounced && latestCredit.schedule?.label" class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ latestCredit.schedule.label }}</p>
        <p v-if="latestCredit.scope" class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ latestCredit.scope }}</p>
        <a
          v-if="latestCreditPost?.url"
          :href="latestCreditPost.url"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-3 inline-flex text-xs font-medium text-teal-700 hover:underline dark:text-teal-300"
        >
          {{ t('subscriptionAccounts.tiboReset.viewPost') }}
        </a>
      </section>

    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import subscriptionAccountsAPI from '@/api/subscriptionAccounts'
import { formatDate as formatDateWithOptions } from '@/utils/format'
import type { TiboResetMonitor, TiboResetMonitorEvent } from '@/types'

const { t } = useI18n()
const loading = ref(true)
const monitor = ref<TiboResetMonitor | null>(null)
const dateOnlyPattern = /^\d{4}-\d{2}-\d{2}$/
const sourceDateFormatter = computed(() => {
  const options: Intl.DateTimeFormatOptions = {
    timeZone: monitor.value?.timezone || 'Asia/Shanghai',
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
  }
  try {
    return new Intl.DateTimeFormat('en-US', options)
  } catch {
    return new Intl.DateTimeFormat('en-US', { ...options, timeZone: 'Asia/Shanghai' })
  }
})

function validDate(value?: string | null): value is string {
  if (!value || !Number.isFinite(Date.parse(value))) return false
  return !dateOnlyPattern.test(value) || new Date(value).toISOString().slice(0, 10) === value
}

function timestamp(value?: string | null): number {
  const parsed = value ? Date.parse(value) : NaN
  return Number.isFinite(parsed) ? parsed : 0
}

function eventDateValue(event: TiboResetMonitorEvent): string | null {
  const candidates = event.status === 'announced'
    ? [event.schedule?.from]
    : [event.confirmedAt, event.occurredOn]
  return candidates.find(validDate) || null
}

function eventTimeKey(event: TiboResetMonitorEvent): string {
  // Publication time only orders undated events; it is not an occurrence time.
  const value = eventDateValue(event) || event.createdAt
  if (!validDate(value)) return ''
  // Compare calendar days and exact times in the same source timezone.
  if (dateOnlyPattern.test(value)) return `${value.replace(/-/g, '')}000000000`
  const date = new Date(value)
  const parts = Object.fromEntries(sourceDateFormatter.value.formatToParts(date).map((part) => [part.type, part.value]))
  return ['year', 'month', 'day', 'hour', 'minute', 'second'].map((field) => parts[field]).join('')
    + String(date.getUTCMilliseconds()).padStart(3, '0')
}

function findLatestEvent(type: string, statuses: string[]): TiboResetMonitorEvent | null {
  return (monitor.value?.events || [])
    .filter((event) => event.type === type && statuses.includes(event.status))
    .sort((a, b) => eventTimeKey(b).localeCompare(eventTimeKey(a))
      || Number(b.status === 'confirmed') - Number(a.status === 'confirmed')
      || timestamp(b.createdAt) - timestamp(a.createdAt))[0] || null
}

function sourcePost(event: TiboResetMonitorEvent | null) {
  const posts = [...(event?.posts || [])].sort((a, b) => timestamp(b.publishedAt) - timestamp(a.publishedAt))
  const stage = event?.status === 'confirmed'
    ? (event.type === 'direct_reset' ? '确认完成' : '确认发卡')
    : (event?.type === 'direct_reset' ? '预告' : '发卡预告')
  return posts.find((post) => post.stage === stage && post.url) || posts.find((post) => post.url) || null
}

const latest = computed(() => findLatestEvent('direct_reset', ['confirmed']))
const latestCredit = computed(() => findLatestEvent('reset_credit', ['confirmed', 'announced']))
const creditAnnounced = computed(() => latestCredit.value?.status === 'announced')
const latestPost = computed(() => sourcePost(latest.value))
const latestCreditPost = computed(() => sourcePost(latestCredit.value))

function formatDate(value: string): string {
  return formatDateWithOptions(value, {
    timeZone: sourceDateFormatter.value.resolvedOptions().timeZone,
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
  })
}

function eventDate(event: TiboResetMonitorEvent): string {
  const value = eventDateValue(event)
  if (!value) return t('subscriptionAccounts.tiboReset.unknown')
  return dateOnlyPattern.test(value) ? value.replace(/-/g, '/') : formatDate(value)
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
