<template>
  <div class="rounded-xl border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-800/60" :aria-busy="busy">
    <input ref="input" type="file" class="hidden" accept=".jpg,.jpeg,.png,.webp,.pdf" :disabled="disabled || busy" :aria-label="t('accounting.files.choose')" @change="select" />
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center">
      <div class="flex min-w-0 flex-1 items-center gap-3">
        <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-white text-primary-600 dark:border-dark-600 dark:bg-dark-700 dark:text-primary-400">
          <Icon name="document" size="lg" aria-hidden="true" />
        </span>
        <div :id="`${id}-file`" class="min-w-0 flex-1" aria-live="polite" aria-atomic="true">
          <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ modelValue?.name || t('common.noFileSelected') }}</p>
          <p v-if="modelValue" class="mt-1 text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ formatBytes(modelValue.size) }}</p>
        </div>
      </div>
      <div class="flex shrink-0 flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary min-h-11 flex-1 sm:flex-none" :disabled="disabled || busy" :aria-describedby="`${id}-file ${id}-hint`" @click="input?.click()">
          <Icon name="upload" size="sm" :stroke-width="2" aria-hidden="true" />
          {{ t(busy ? 'accounting.files.uploading' : modelValue ? 'accounting.files.replace' : 'accounting.files.choose') }}
        </button>
        <button v-if="modelValue" type="button" class="btn btn-secondary h-11 w-11 shrink-0 p-0" :disabled="disabled || busy" :aria-label="`${t('common.remove')} ${modelValue.name}`" :title="t('common.remove')" @click="emit('update:modelValue', null)">
          <Icon name="x" size="sm" :stroke-width="2" aria-hidden="true" />
        </button>
      </div>
    </div>
    <p :id="`${id}-hint`" class="mt-4 border-t border-gray-200 pt-3 text-xs leading-5 text-gray-500 dark:border-dark-700 dark:text-dark-400">{{ t('accounting.files.hint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import { formatBytes } from '@/utils/format'

const props = withDefaults(defineProps<{ modelValue?: File | null; disabled?: boolean; busy?: boolean }>(), { modelValue: null, disabled: false, busy: false })
const emit = defineEmits<{ 'update:modelValue': [file: File | null] }>()
const { t } = useI18n()
const id = useId()
const input = ref<HTMLInputElement>()

function select(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (file && !props.disabled && !props.busy) emit('update:modelValue', file)
}
</script>
