<template>
  <AppLayout>
    <main class="accounting-page mx-auto w-full max-w-4xl space-y-5 pb-10" data-testid="accounting-form-page">
      <PageHeader :title="title" :subtitle="subtitle" :back="{ label: backLabel }" @back="emit('cancel')" />
      <p v-if="loading" role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>
      <form v-else ref="formRef" novalidate class="accounting-form card" :aria-busy="busy || undefined" @submit.prevent="submit">
        <p v-if="error" ref="errorRef" role="alert" tabindex="-1" class="accounting-error mx-6 mt-6">{{ error }}</p>
        <slot />
        <div class="accounting-form-footer">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="emit('cancel')">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" class="btn" :class="danger ? 'btn-danger' : 'btn-primary'" :disabled="busy || submitDisabled">
            <span v-if="busy" class="accounting-spinner" aria-hidden="true" />
            {{ busy ? t('accounting.form.submitting') : submitLabel || t('common.save') }}
          </button>
        </div>
      </form>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from './PageHeader.vue'
import '@/styles/accounting.css'

const props = defineProps<{
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
const formRef = ref<HTMLFormElement>()
const errorRef = ref<HTMLElement>()

watch(() => [props.error, props.loading] as const, async ([error, loading]) => {
  if (!error || loading) return
  await nextTick()
  if (props.error !== error) return
  const target = formRef.value?.querySelector<HTMLElement>('[aria-invalid="true"]:not([disabled])') || errorRef.value
  target?.focus({ preventScroll: true })
  target?.scrollIntoView?.({ block: 'center', behavior: 'instant' })
})

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
