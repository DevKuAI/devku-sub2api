<template>
  <AppLayout>
    <main class="mx-auto w-full max-w-4xl pb-10" data-testid="accounting-form-page">
      <button type="button" class="accounting-back" @click="emit('cancel')">
        <Icon name="arrowLeft" size="sm" aria-hidden="true" />
        {{ backLabel }}
      </button>
      <header class="border-b border-gray-200 pb-5 dark:border-dark-700">
        <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">{{ title }}</h1>
        <p v-if="subtitle" class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ subtitle }}</p>
      </header>
      <p v-if="loading" role="status" class="py-8 text-sm text-gray-500">{{ t('accounting.loading') }}</p>
      <form v-else novalidate class="divide-y divide-gray-200 dark:divide-dark-700" @submit.prevent="submit">
        <p v-if="error" role="alert" class="accounting-error mt-6">{{ error }}</p>
        <slot />
        <div class="flex flex-wrap justify-end gap-3 py-6">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="emit('cancel')">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" class="btn" :class="danger ? 'btn-danger' : 'btn-primary'" :disabled="busy || submitDisabled">
            {{ submitLabel || t('common.save') }}
          </button>
        </div>
      </form>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'
import '@/styles/accounting.css'

defineProps<{
  title: string
  subtitle?: string
  backLabel: string
  loading?: boolean
  error?: string
  busy?: boolean
  submitLabel?: string
  submitDisabled?: boolean
  danger?: boolean
}>()
const emit = defineEmits<{ submit: []; cancel: [] }>()
const { t } = useI18n()

// Native constraint validation still runs; novalidate only lets us focus the first invalid field ourselves.
function submit(event: Event) {
  const form = event.target as HTMLFormElement
  if (!form.checkValidity()) {
    form.reportValidity()
    return
  }
  emit('submit')
}
</script>
