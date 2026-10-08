<template>
  <div class="desktop-analytics analytics-range space-y-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800" data-testid="desktop-analytics-range-picker">
    <div class="flex flex-wrap items-center gap-3">
      <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.desktop.usageStatistics.range') }}</span>
      <div class="range-options" role="group" :aria-label="t('admin.desktop.usageStatistics.range')">
        <button v-for="option in options" :key="option.value" type="button" class="analytics-button" :aria-pressed="mode === option.value" @click="select(option.value)">
          <Icon v-if="mode === option.value" name="check" size="xs" aria-hidden="true" />{{ t(`admin.desktop.usageStatistics.${option.value}`) }}
        </button>
      </div>
    </div>
    <form v-if="mode === 'custom'" class="range-form" novalidate @submit.prevent="applyCustom">
      <label class="min-w-0 space-y-2 text-sm text-gray-700 dark:text-dark-200"><span>{{ t('admin.desktop.usageStatistics.fromDate') }}</span><input ref="fromInput" v-model="from" class="analytics-input" name="usage-from" type="date" required :max="to || today()" :aria-invalid="error ? true : undefined" :aria-describedby="error ? errorID : undefined" /></label>
      <label class="min-w-0 space-y-2 text-sm text-gray-700 dark:text-dark-200"><span>{{ t('admin.desktop.usageStatistics.toDate') }}</span><input ref="toInput" v-model="to" class="analytics-input" name="usage-to" type="date" required :min="from || undefined" :max="today()" :aria-invalid="error ? true : undefined" :aria-describedby="error ? errorID : undefined" /></label>
      <button class="analytics-button" type="submit">{{ t('admin.desktop.usageStatistics.applyRange') }}</button>
    </form>
    <p v-if="error" :id="errorID" class="text-sm text-red-700 dark:text-red-300" role="alert">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import '@/styles/desktopAnalytics.css'
import Icon from '@/components/icons/Icon.vue'
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
const errorID = useId()
const fromInput = ref<HTMLInputElement | null>(null)
const toInput = ref<HTMLInputElement | null>(null)
watch(() => props.modelValue, value => {
  mode.value = value.from ? 'custom' : `days${value.days ?? 30}` as Mode
  from.value = value.from || ''
  to.value = value.to || ''
  error.value = ''
}, { deep: true })
function today() {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
}

function select(value: Mode) {
  mode.value = value
  error.value = ''
  if (value !== 'custom') emit('update:modelValue', { days: Number(value.slice(4)) as 7 | 30 | 90 })
}

function applyCustom() {
  const start = Date.parse(`${from.value}T00:00:00Z`)
  const end = Date.parse(`${to.value}T00:00:00Z`)
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start || (end - start) / 86400000 >= 90 || to.value > today()) {
    error.value = t('admin.desktop.usageStatistics.invalidRange')
    ;(!Number.isFinite(start) || (Number.isFinite(end) && end < start) ? fromInput.value : toInput.value)?.focus()
    return
  }
  error.value = ''
  emit('update:modelValue', { from: from.value, to: to.value })
}
</script>

<style scoped>
.range-options { display: grid; width: 100%; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.range-form { display: grid; grid-template-columns: minmax(0, 1fr); gap: 16px; align-items: end; }
@container (min-width: 32rem) {
  .range-options { display: flex; flex-wrap: wrap; width: auto; }
  .range-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@container (min-width: 48rem) { .range-form { grid-template-columns: repeat(2, minmax(0, 1fr)) minmax(0, auto); } }
</style>
