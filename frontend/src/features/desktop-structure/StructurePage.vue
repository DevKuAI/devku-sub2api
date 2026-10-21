<template>
  <AppLayout>
    <div class="desktop-structure space-y-6">
      <header class="structure-page-header">
        <button type="button" class="btn btn-secondary structure-back" :disabled="busy" @click="$emit('back')">
          <Icon name="arrowLeft" size="sm" aria-hidden="true" />{{ t('common.back') }}
        </button>
        <div class="min-w-0 flex-1">
          <p v-if="subtitle" class="structure-caption mb-1"><bdi>{{ subtitle }}</bdi></p>
          <h1 ref="heading" tabindex="-1" class="structure-page-title">{{ title }}</h1>
          <p v-if="description" class="structure-caption mt-2 max-w-prose">{{ description }}</p>
        </div>
        <slot name="actions" />
      </header>
      <p class="structure-caption" role="status"><span v-if="busy">{{ t('common.saving') }}</span><span v-else-if="dirty">{{ t('desktopStructure.unsaved') }}</span></p>
      <div v-if="loading" class="structure-panel flex items-center gap-3" role="status">
        <Icon name="clock" size="md" aria-hidden="true" /><span>{{ t('common.loading') }}</span>
      </div>
      <div v-if="error" ref="errorPanel" tabindex="-1" class="structure-error" role="alert">
        <div class="min-w-0 flex-1"><p class="font-medium">{{ error }}</p><p class="mt-1">{{ t('desktopStructure.errorRecovery') }}</p></div>
        <button class="btn btn-secondary" type="button" :disabled="busy" @click="$emit('retry')"><Icon name="refresh" size="sm" aria-hidden="true" />{{ t('desktopStructure.reloadAction') }}</button>
      </div>
      <slot v-if="!loading" />
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import '@/styles/desktopStructure.css'
const props = defineProps<{ title: string; subtitle?: string; description?: string; loading?: boolean; error?: string; busy?: boolean; dirty?: boolean; focusError?: boolean }>()
defineEmits<{ back: []; retry: [] }>()
const { t } = useI18n()
const route = useRoute()
const heading = ref<HTMLElement>(), errorPanel = ref<HTMLElement>()
onMounted(() => heading.value?.focus())
watch(() => route.path, async () => { await nextTick(); heading.value?.focus() })
watch(() => [props.title, props.subtitle], () => { document.title = [props.title, props.subtitle].filter(Boolean).join(' · ') }, { immediate: true })
watch(() => props.error, async error => { if (error && props.focusError !== false) { await nextTick(); errorPanel.value?.focus() } })
</script>
