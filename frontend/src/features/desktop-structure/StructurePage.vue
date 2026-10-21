<template>
  <AppLayout>
    <main class="mx-auto min-w-0 max-w-6xl space-y-6 p-4 sm:p-6">
      <header class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0"><h1 class="break-words text-2xl font-semibold">{{ title }}</h1><p class="mt-1 break-words text-sm text-gray-500">{{ subtitle }}</p></div>
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="$emit('back')">{{ t('common.back') }}</button>
      </header>
      <p v-if="loading" role="status">{{ t('common.loading') }}</p>
      <div v-if="error" class="rounded-lg border border-red-300 p-4 text-red-700 dark:text-red-300" role="alert">{{ error }}<button class="btn btn-secondary ml-3" type="button" :disabled="busy" @click="$emit('retry')">{{ t('common.refresh') }}</button></div>
      <slot v-if="!loading" />
    </main>
  </AppLayout>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
defineProps<{ title: string; subtitle?: string; loading?: boolean; error?: string; busy?: boolean }>()
defineEmits<{ back: []; retry: [] }>()
const { t } = useI18n()
</script>
