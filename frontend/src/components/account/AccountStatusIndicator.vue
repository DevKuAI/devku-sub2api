<template>
  <div :class="layout === 'inline' ? 'flex min-w-0 max-w-full flex-wrap items-center gap-x-2 gap-y-2' : 'flex items-center gap-2'">
    <div v-if="isRateLimited" :class="layout === 'inline' ? 'contents' : 'flex flex-col items-center gap-1'">
      <span class="badge text-xs badge-warning">{{ t('admin.accounts.status.rateLimited') }}</span>
      <span :class="layout === 'inline' ? 'order-1 text-xs leading-5 tabular-nums text-gray-600 dark:text-dark-300' : 'text-[11px] text-gray-400 dark:text-gray-500'">{{ rateLimitResumeText }}</span>
    </div>

    <div v-else-if="isOverloaded" :class="layout === 'inline' ? 'contents' : 'flex flex-col items-center gap-1'">
      <span class="badge text-xs badge-danger">{{ t('admin.accounts.status.overloaded') }}</span>
      <span :class="layout === 'inline' ? 'order-1 text-xs leading-5 tabular-nums text-gray-600 dark:text-dark-300' : 'text-[11px] text-gray-400 dark:text-gray-500'">{{ overloadCountdown }}</span>
    </div>

    <template v-else>
      <div v-if="isTempUnschedulable" :class="layout === 'inline' ? 'contents' : 'flex flex-col items-center gap-1'">
        <button
          v-if="!readonly"
          type="button"
          :class="['badge text-xs', statusClass, 'cursor-pointer']"
          :title="t('admin.accounts.status.viewTempUnschedDetails')"
          @click="handleTempUnschedClick"
        >
          {{ statusText }}
        </button>
        <span v-else :class="['badge text-xs', statusClass]">{{ statusText }}</span>
        <span :class="layout === 'inline' ? 'order-1 text-xs leading-5 tabular-nums text-gray-600 dark:text-dark-300' : 'max-w-[180px] text-center text-[11px] leading-4 text-gray-500 dark:text-gray-400'">
          {{ tempUnschedRecoveryText }}
        </span>
      </div>
      <span v-else :class="['badge text-xs', statusClass]">{{ statusText }}</span>
    </template>

    <HelpTooltip v-if="hasError && account.error_message" class="!ml-0" width-class="w-72">
      <template #trigger="{ tooltipId }">
        <button type="button" class="inline-flex min-h-6 min-w-6 items-center justify-center rounded text-red-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600 dark:text-red-400 dark:focus-visible:outline-primary-400" :aria-label="t('admin.accounts.status.error')" :aria-describedby="tooltipId">
          <Icon name="exclamationTriangle" size="sm" aria-hidden="true" />
        </button>
      </template>
      <div class="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{{ account.error_message }}</div>
    </HelpTooltip>

    <HelpTooltip v-if="isRateLimited" class="!ml-0" width-class="w-56" :content="t('admin.accounts.status.rateLimitedUntil', { time: formatDateTime(account.rate_limit_reset_at) })">
      <template #trigger="{ tooltipId }">
        <button type="button" class="inline-flex min-h-6 items-center gap-1 rounded bg-amber-100 px-2 py-0.5 text-xs font-medium tabular-nums text-amber-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600 dark:bg-amber-900/30 dark:text-amber-300 dark:focus-visible:outline-primary-400" :aria-label="`${t('admin.accounts.status.rateLimited')} (429)`" :aria-describedby="tooltipId">
          <Icon name="exclamationTriangle" size="xs" :stroke-width="2" class="shrink-0" aria-hidden="true" />
          429
        </button>
      </template>
    </HelpTooltip>

    <div
      v-if="activeModelStatuses.length > 0"
      :class="layout === 'inline' ? 'order-last flex min-w-0 max-w-full flex-wrap gap-2' : activeModelStatuses.length <= 4 ? 'flex flex-col gap-1' : activeModelStatuses.length <= 8 ? 'columns-2 gap-x-2' : 'columns-3 gap-x-2'"
    >
      <HelpTooltip v-for="item in activeModelStatuses" :key="`${item.kind}-${item.model}`" class="!ml-0 max-w-full break-inside-avoid" :class="layout === 'inline' ? '' : 'mb-1'" width-class="w-72" :content="modelStatusTooltip(item)">
        <template #trigger="{ tooltipId }">
          <button
            type="button"
            :aria-describedby="tooltipId"
            :class="[
              'inline-flex min-h-6 min-w-0 max-w-full items-center gap-1 rounded px-2 py-0.5 text-xs font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600 dark:focus-visible:outline-primary-400',
              item.kind === 'credits_exhausted' ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300' : item.kind === 'credits_active' ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300' : 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300'
            ]"
          >
            <span v-if="item.kind === 'credits_active'" aria-hidden="true">⚡</span>
            <Icon v-else name="exclamationTriangle" size="xs" :stroke-width="2" class="shrink-0" aria-hidden="true" />
            <span class="min-w-0 max-w-40 truncate">{{ item.kind === 'credits_exhausted' ? t('admin.accounts.status.creditsExhausted') : formatScopeName(item.model) }}</span>
            <span class="shrink-0 text-xs tabular-nums">{{ formatCountdown(item.reset_at) }}</span>
          </button>
        </template>
      </HelpTooltip>
    </div>

    <HelpTooltip v-if="isOverloaded" class="!ml-0" width-class="w-56" :content="t('admin.accounts.status.overloadedUntil', { time: formatTime(account.overload_until) })">
      <template #trigger="{ tooltipId }">
        <button type="button" class="inline-flex min-h-6 items-center gap-1 rounded bg-red-100 px-2 py-0.5 text-xs font-medium tabular-nums text-red-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-600 dark:bg-red-900/30 dark:text-red-300 dark:focus-visible:outline-primary-400" :aria-label="`${t('admin.accounts.status.overloaded')} (529)`" :aria-describedby="tooltipId">
          <Icon name="exclamationTriangle" size="xs" :stroke-width="2" class="shrink-0" aria-hidden="true" />
          529
        </button>
      </template>
    </HelpTooltip>
  </div>
</template>

<script setup lang="ts" generic="T extends AccountStatusInfo & Partial<Pick<Account, 'extra' | 'error_message'>>">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import type { Account, AccountStatusInfo } from '@/types'
import { formatCountdown, formatDateTime, formatDateTimeToMinute, formatCountdownWithSuffix, formatTime } from '@/utils/format'

const { t } = useI18n()

const props = defineProps<{
  account: T
  readonly?: boolean
  layout?: 'stacked' | 'inline'
}>()

const emit = defineEmits<{
  (e: 'show-temp-unsched', account: T): void
}>()

// Computed: is rate limited (429)
const isRateLimited = computed(() => {
  if (!props.account.rate_limit_reset_at) return false
  return new Date(props.account.rate_limit_reset_at) > new Date()
})

type AccountModelStatusItem = {
  kind: 'rate_limit' | 'credits_exhausted' | 'credits_active'
  model: string
  reset_at: string
}

// Computed: active model statuses (普通模型限流 + 积分耗尽 + 走积分中)
const activeModelStatuses = computed<AccountModelStatusItem[]>(() => {
  const extra = props.account.extra as Record<string, unknown> | undefined
  const modelLimits = (props.account.model_rate_limits ?? extra?.model_rate_limits) as
    | Record<string, { rate_limit_reset_at: string }>
    | undefined
  const now = new Date()
  const items: AccountModelStatusItem[] = []

  if (!modelLimits) return items

  // 检查 AICredits key 是否生效（积分是否耗尽）
  const aiCreditsEntry = modelLimits['AICredits']
  const hasActiveAICredits = aiCreditsEntry && new Date(aiCreditsEntry.rate_limit_reset_at) > now
  const allowOverages = props.account.allow_overages ?? !!extra?.allow_overages

  for (const [model, info] of Object.entries(modelLimits)) {
    if (new Date(info.rate_limit_reset_at) <= now) continue

    if (model === 'AICredits') {
      // AICredits key → 积分已用尽
      items.push({ kind: 'credits_exhausted', model, reset_at: info.rate_limit_reset_at })
    } else if (allowOverages && !hasActiveAICredits) {
      // 普通模型限流 + overages 启用 + 积分可用 → 正在走积分
      items.push({ kind: 'credits_active', model, reset_at: info.rate_limit_reset_at })
    } else {
      // 普通模型限流
      items.push({ kind: 'rate_limit', model, reset_at: info.rate_limit_reset_at })
    }
  }

  return items
})

const modelStatusTooltip = (item: AccountModelStatusItem): string => {
  const time = formatDateTimeToMinute(item.reset_at)
  if (item.kind === 'credits_exhausted') return t('admin.accounts.status.creditsExhaustedUntil', { time })
  const key = item.kind === 'credits_active' ? 'modelCreditOveragesUntil' : 'modelRateLimitedUntil'
  return t(`admin.accounts.status.${key}`, { model: formatScopeName(item.model), time })
}

const formatScopeName = (scope: string): string => {
  const aliases: Record<string, string> = {
    // Claude 系列
    'claude-fable-5-1': 'CFable51',
    'claude-fable-5': 'CFable5',
    'claude-opus-4-6': 'COpus46',
    'claude-opus-4-6-thinking': 'COpus46T',
    'claude-opus-4-7': 'COpus47',
    'claude-opus-4-8': 'COpus48',
    'claude-opus-5': 'COpus5',
    'claude-sonnet-4-6': 'CSon46',
    'claude-sonnet-4-5': 'CSon45',
    'claude-sonnet-4-5-thinking': 'CSon45T',
    'claude-sonnet-5': 'CSon5',
    // Gemini 2.5 系列
    'gemini-2.5-flash': 'G25F',
    'gemini-2.5-flash-lite': 'G25FL',
    'gemini-2.5-flash-thinking': 'G25FT',
    'gemini-2.5-pro': 'G25P',
    'gemini-2.5-flash-image': 'G25I',
    // Gemini 3.5 系列
    'gemini-3.5-flash': 'G35F',
    // Gemini 3 系列
    'gemini-3-flash': 'G3F',
    'gemini-3.1-pro-high': 'G3PH',
    'gemini-3.1-pro-low': 'G3PL',
    'gemini-3-pro-image': 'G3PI',
    'gemini-3.1-flash-image': 'G31FI',
    // 其他
    'gpt-oss-120b-medium': 'GPT120',
    'tab_flash_lite_preview': 'TabFL',
    // 旧版 scope 别名（兼容）
    claude: 'Claude',
    claude_sonnet: 'CSon',
    claude_opus: 'COpus',
    claude_haiku: 'CHaiku',
    gemini_text: 'Gemini',
    gemini_image: 'GImg',
    gemini_flash: 'GFlash',
    gemini_pro: 'GPro',
  }
  return aliases[scope] || scope
}

// Computed: is overloaded (529)
const isOverloaded = computed(() => {
  if (!props.account.overload_until) return false
  return new Date(props.account.overload_until) > new Date()
})

// Computed: is temp unschedulable
const isTempUnschedulable = computed(() => {
  if (!props.account.temp_unschedulable_until) return false
  return new Date(props.account.temp_unschedulable_until) > new Date()
})

// Computed: has error status
const hasError = computed(() => {
  return props.account.status === 'error'
})

const isQuotaExceeded = computed(() => {
  const exceeded = (used?: number | null, limit?: number | null) =>
    typeof limit === 'number' && limit > 0 && typeof used === 'number' && used >= limit
  return (
    exceeded(props.account.quota_used, props.account.quota_limit) ||
    exceeded(props.account.quota_daily_used, props.account.quota_daily_limit) ||
    exceeded(props.account.quota_weekly_used, props.account.quota_weekly_limit)
  )
})

// Computed: countdown text for rate limit (429)
const rateLimitCountdown = computed(() => {
  return formatCountdown(props.account.rate_limit_reset_at)
})

const rateLimitResumeText = computed(() => {
  if (!rateLimitCountdown.value) return ''
  return t('admin.accounts.status.rateLimitedAutoResume', { time: rateLimitCountdown.value })
})

// Computed: countdown text for overload (529)
const overloadCountdown = computed(() => {
  return formatCountdownWithSuffix(props.account.overload_until)
})

const tempUnschedRecoveryText = computed(() => {
  if (!isTempUnschedulable.value || !props.account.temp_unschedulable_until) return ''
  return t('admin.accounts.status.tempUnschedulableUntil', {
    time: formatDateTime(props.account.temp_unschedulable_until)
  })
})

// Computed: status badge class
const statusClass = computed(() => {
  if (hasError.value) {
    return 'badge-danger'
  }
  if (isTempUnschedulable.value) {
    return 'badge-warning'
  }
  if (props.account.status !== 'active') {
    return props.account.status === 'error' ? 'badge-danger' : 'badge-gray'
  }
  if (isQuotaExceeded.value) {
    return 'badge-warning'
  }
  if (props.account.schedulable === false) {
    return 'badge-gray'
  }
  return 'badge-success'
})

// Computed: status text
const statusText = computed(() => {
  if (hasError.value) {
    return t('admin.accounts.status.error')
  }
  if (isTempUnschedulable.value) {
    return t('admin.accounts.status.tempUnschedulable')
  }
  if (props.account.status !== 'active') {
    return t(`admin.accounts.status.${props.account.status}`)
  }
  if (isQuotaExceeded.value) {
    return t('admin.accounts.status.quotaExceeded')
  }
  if (props.account.schedulable === false) {
    return t('admin.accounts.status.paused')
  }
  return t(`admin.accounts.status.${props.account.status}`)
})

const handleTempUnschedClick = () => {
  if (!isTempUnschedulable.value) return
  emit('show-temp-unsched', props.account)
}
</script>
