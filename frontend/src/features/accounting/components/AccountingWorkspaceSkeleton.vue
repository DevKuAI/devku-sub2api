<template>
  <div role="status" :aria-label="t('accounting.loading')">
    <span class="sr-only">{{ t('accounting.loading') }}</span>
    <div v-if="overview" class="space-y-5" aria-hidden="true">
      <div class="grid gap-3 lg:grid-cols-2">
        <div class="card min-h-48 p-5">
          <div class="accounting-skeleton h-5 w-12" />
          <div class="mt-3 grid grid-cols-2 gap-4">
            <div v-for="metric in 4" :key="metric">
              <div class="accounting-skeleton h-4 w-16" />
              <div class="accounting-skeleton mt-1 h-7 w-28 max-w-full" />
            </div>
          </div>
        </div>
      </div>
      <div v-if="!customer" class="card min-h-44 space-y-4 p-5">
        <div class="accounting-skeleton h-6 w-40" />
        <div class="accounting-skeleton h-10 w-full" />
      </div>
      <div v-if="!customer" class="card min-h-40 space-y-4 p-5">
        <div class="accounting-skeleton h-6 w-24" />
        <div class="accounting-skeleton h-10 w-full" />
      </div>
    </div>
    <div v-else class="card min-h-80 space-y-5 p-5" aria-hidden="true">
      <div class="accounting-skeleton h-10 w-48 max-w-full" />
      <div v-for="row in 4" :key="row" class="accounting-skeleton h-10 w-full" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{ customer: boolean; overview: boolean }>()
const { t } = useI18n()
</script>
