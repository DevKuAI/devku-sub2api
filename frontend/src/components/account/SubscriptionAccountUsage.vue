<template>
  <div class="min-w-0 space-y-2">
    <div v-if="bars.length" class="space-y-2">
      <UsageProgressBar
        v-for="bar in bars"
        :key="bar.key"
        :label="bar.label"
        :utilization="bar.utilization"
        :resets-at="bar.resetsAt"
        :window-stats="bar.windowStats"
        :show-window-stats-when-empty="true"
        :color="bar.color"
        label-width="auto"
      />
      <p v-if="usage?.updated_at" class="text-xs text-gray-400 dark:text-dark-500">
        {{ t('subscriptionAccounts.usageUpdatedAt', { time: formatDateTimeToMinute(usage.updated_at) }) }}
      </p>
    </div>
    <span v-else class="text-sm text-gray-400 dark:text-dark-500">
      {{ t('subscriptionAccounts.noUsage') }}
    </span>
    <OpenAIQuotaResetCell
      v-if="account?.platform === 'openai' && account.type === 'oauth'"
      :account="account"
      :api="quotaAPI"
      :busy="querying"
    >
      <template #pre-actions="{ busy }">
        <button
          type="button"
          class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-blue-600 transition-colors hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-50 dark:text-blue-400 dark:hover:bg-blue-900/30"
          :disabled="querying || busy"
          @click="loadUsage(true)"
        >
          <Icon name="refresh" size="xs" :class="{ 'animate-spin': querying }" aria-hidden="true" />
          {{ t('admin.accounts.usageWindow.activeQuery') }}
        </button>
      </template>
    </OpenAIQuotaResetCell>
    <p v-if="queryError" role="alert" class="text-xs text-red-600 dark:text-red-400">
      {{ t('subscriptionAccounts.failedToRefreshUsage') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'
import OpenAIQuotaResetCell from '@/components/account/OpenAIQuotaResetCell.vue'
import Icon from '@/components/icons/Icon.vue'
import subscriptionAccountsAPI from '@/api/subscriptionAccounts'
import { formatDateTimeToMinute } from '@/utils/format'
import type { SubscriptionAccount, SubscriptionAccountUsage, UsageProgress, WindowStats } from '@/types'

const props = defineProps<{
  usage?: SubscriptionAccountUsage | null
  account?: SubscriptionAccount
}>()

const emit = defineEmits<{
  'update:usage': [usage: SubscriptionAccountUsage | null]
}>()

const { t } = useI18n()
const querying = ref(false)
const queryError = ref(false)

async function loadUsage(refresh: boolean): Promise<void> {
  if (!props.account || querying.value) return
  querying.value = true
  queryError.value = false
  try {
    const usage = refresh
      ? await subscriptionAccountsAPI.refreshUsage(props.account.id)
      : await subscriptionAccountsAPI.getUsage(props.account.id)
    emit('update:usage', usage)
  } catch {
    queryError.value = true
  } finally {
    querying.value = false
  }
}

const quotaAPI = {
  refresh: subscriptionAccountsAPI.refreshQuota,
  reset: async (id: number) => {
    const result = await subscriptionAccountsAPI.resetQuota(id)
    // A failed usage read must not turn an already consumed credit into a reset error.
    void loadUsage(false)
    return result
  },
}

interface UsageBar {
  key: string
  label: string
  utilization: number
  resetsAt: string | null
  windowStats?: WindowStats | null
  color: 'indigo' | 'emerald' | 'purple' | 'amber'
}

function progressBar(key: string, label: string, progress: UsageProgress | null | undefined, color: UsageBar['color']): UsageBar | null {
  if (!progress) return null
  return {
    key,
    label,
    utilization: progress.utilization,
    resetsAt: progress.resets_at,
    windowStats: progress.window_stats,
    color
  }
}

function grokBar(key: string, label: string, quota: SubscriptionAccountUsage['grok_request_quota'], color: UsageBar['color']): UsageBar | null {
  if (!quota || !quota.limit || quota.limit <= 0 || quota.remaining == null) return null
  const resetsAt = quota.reset_at || (quota.reset_unix ? new Date(quota.reset_unix * 1000).toISOString() : null)
  return {
    key,
    label,
    utilization: ((quota.limit - quota.remaining) / quota.limit) * 100,
    resetsAt,
    color
  }
}

const bars = computed<UsageBar[]>(() => {
  const usage = props.usage
  if (!usage) return []
  const result = [
    progressBar('five_hour', '5h', usage.five_hour, 'indigo'),
    progressBar('seven_day', '7d', usage.seven_day, 'emerald'),
    progressBar('seven_day_sonnet', '7d S', usage.seven_day_sonnet, 'purple'),
    progressBar('seven_day_fable', '7d F', usage.seven_day_fable, 'amber'),
    progressBar('thirty_day', '30d', usage.thirty_day, 'purple'),
    progressBar('gemini_shared_daily', '1d', usage.gemini_shared_daily, 'indigo'),
    progressBar('gemini_pro_daily', 'Pro 1d', usage.gemini_pro_daily, 'indigo'),
    progressBar('gemini_flash_daily', 'Flash 1d', usage.gemini_flash_daily, 'emerald'),
    progressBar('gemini_shared_minute', '1m', usage.gemini_shared_minute, 'amber'),
    progressBar('gemini_pro_minute', 'Pro 1m', usage.gemini_pro_minute, 'amber'),
    progressBar('gemini_flash_minute', 'Flash 1m', usage.gemini_flash_minute, 'amber'),
    grokBar('grok_requests', 'Req', usage.grok_request_quota, 'indigo'),
    grokBar('grok_tokens', 'Token', usage.grok_token_quota, 'emerald')
  ].filter((bar): bar is UsageBar => bar !== null)

  for (const [model, quota] of Object.entries(usage.antigravity_quota || {})) {
    result.push({
      key: `antigravity:${model}`,
      label: model,
      utilization: quota.utilization,
      resetsAt: quota.reset_time || null,
      color: 'purple'
    })
  }
  return result
})
</script>
