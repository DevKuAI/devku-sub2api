<template>
  <section class="space-y-3" :aria-label="t('admin.desktop.conversations.statistics.title')" :aria-busy="loading" data-testid="conversation-statistics">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.desktop.conversations.statistics.title') }}</h3>
      <button class="btn btn-ghost btn-sm gap-1.5" type="button" :disabled="loading" @click="load">
        <Icon name="refresh" size="sm" aria-hidden="true" />{{ t('common.refresh') }}
      </button>
    </div>
    <div v-if="error" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300" role="alert">
      {{ t('admin.desktop.conversations.statistics.loadFailed') }}
      <button class="ml-2 underline" type="button" @click="load">{{ t('admin.desktop.conversations.retry') }}</button>
    </div>
    <div v-else class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <article v-for="period in periods" :key="period.key" class="min-w-0 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800" :data-testid="`statistics-${period.key}`">
        <div class="flex items-center justify-between gap-2 text-sm text-gray-500 dark:text-dark-400">
          <h4>{{ t(`admin.desktop.conversations.statistics.${period.key}`) }}</h4>
          <Icon :name="period.icon" size="md" class="shrink-0 text-primary-500 dark:text-primary-400" aria-hidden="true" />
        </div>
        <div class="mt-3 flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <span class="break-all text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100">{{ statistics ? statistics[period.key].record_count.toLocaleString() : '—' }}</span>
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.records') }}</span>
        </div>
        <p class="mt-2 break-words text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.statistics.prompts', { count: statistics ? statistics[period.key].prompt_count.toLocaleString() : '—' }) }}</p>
      </article>
    </div>
    <p v-if="loading" class="text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="statistics" class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">
      {{ t('admin.desktop.conversations.statistics.hint', { timezone: statistics.timezone }) }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDesktopConversationStatistics } from '@/api/desktopConversations'
import type { DesktopConversationFilters, DesktopConversationStatistics } from '@/api/desktopConversations'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ organizationId: string; selfManaged: boolean; filters: DesktopConversationFilters }>()
const { t } = useI18n()
const periods = [
  { key: 'today', icon: 'clock' },
  { key: 'week', icon: 'calendar' },
  { key: 'month', icon: 'chartBar' },
  { key: 'total', icon: 'chat' },
] as const
const statistics = ref<DesktopConversationStatistics | null>(null)
const loading = ref(false)
const error = ref(false)
let controller: AbortController | undefined

async function load() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  statistics.value = null
  error.value = false
  loading.value = false
  if (!props.organizationId) return
  loading.value = true
  try {
    const result = await getDesktopConversationStatistics(props.organizationId, props.selfManaged, props.filters, request.signal)
    if (!request.signal.aborted) statistics.value = result
  } catch {
    if (!request.signal.aborted) error.value = true
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
}
watch(() => [props.organizationId, props.selfManaged, props.filters], load, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>
