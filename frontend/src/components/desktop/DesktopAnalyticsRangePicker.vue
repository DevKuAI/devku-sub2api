<template>
  <div class="space-y-3" data-testid="desktop-analytics-range-picker">
    <div class="flex flex-wrap items-center gap-3">
      <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.desktop.usageStatistics.range') }}</span>
      <div class="inline-flex max-w-full overflow-x-auto rounded border border-gray-200 p-0.5 dark:border-dark-700" role="group" :aria-label="t('admin.desktop.usageStatistics.range')">
        <button v-for="option in options" :key="option.value" type="button" class="shrink-0 rounded px-3 py-1.5 text-xs font-medium" :class="mode === option.value ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900' : 'text-gray-600 dark:text-gray-300'" :aria-pressed="mode === option.value" @click="select(option.value)">
          {{ t(`admin.desktop.usageStatistics.${option.value}`) }}
        </button>
      </div>
    </div>
    <form v-if="mode === 'custom'" class="flex flex-wrap items-end gap-3" @submit.prevent="applyCustom">
      <label class="space-y-1 text-xs text-gray-600 dark:text-gray-300"><span>{{ t('admin.desktop.usageStatistics.fromDate') }}</span><input v-model="from" class="input block min-w-36 text-base sm:text-sm" type="date" required /></label>
      <label class="space-y-1 text-xs text-gray-600 dark:text-gray-300"><span>{{ t('admin.desktop.usageStatistics.toDate') }}</span><input v-model="to" class="input block min-w-36 text-base sm:text-sm" type="date" required /></label>
      <button class="btn btn-secondary" type="submit">{{ t('admin.desktop.usageStatistics.applyRange') }}</button>
    </form>
    <p v-if="error" class="text-sm text-red-700 dark:text-red-300" role="alert">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DesktopAnalyticsRange } from '@/api/desktopOrganizationUsage'

const props = defineProps<{ modelValue: DesktopAnalyticsRange }>()
const emit = defineEmits<{ 'update:modelValue': [value: DesktopAnalyticsRange] }>()
const { t } = useI18n()
const options = [{ value: 'days7' }, { value: 'days30' }, { value: 'days90' }, { value: 'custom' }] as const
type Mode = (typeof options)[number]['value']
const mode = ref<Mode>(props.modelValue.from ? 'custom' : `days${props.modelValue.days ?? 30}` as Mode)
const from = ref(props.modelValue.from || '')
const to = ref(props.modelValue.to || '')
const error = ref('')

function select(value: Mode) {
  mode.value = value
  error.value = ''
  if (value !== 'custom') emit('update:modelValue', { days: Number(value.slice(4)) as 7 | 30 | 90 })
}

function applyCustom() {
  const start = Date.parse(`${from.value}T00:00:00Z`)
  const end = Date.parse(`${to.value}T00:00:00Z`)
  const now = new Date()
  const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start || (end - start) / 86400000 >= 90 || to.value > today) {
    error.value = t('admin.desktop.usageStatistics.invalidRange')
    return
  }
  error.value = ''
  emit('update:modelValue', { from: from.value, to: to.value })
}
</script>
