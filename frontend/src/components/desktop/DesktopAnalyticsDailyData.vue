<template>
  <details class="analytics-disclosure">
    <summary>{{ t('admin.desktop.usageStatistics.dailyData') }}</summary>
    <p class="analytics-caption mb-2">{{ t('admin.desktop.usageStatistics.dailyDataHint') }}</p>
    <div class="analytics-data" tabindex="0" role="region" :aria-label="caption">
      <table>
        <caption class="sr-only">{{ caption }}</caption>
        <thead><tr><th scope="col">{{ t('admin.desktop.usageStatistics.date') }}</th><th v-for="column in columns" :key="column.key" scope="col">{{ column.label }}</th></tr></thead>
        <tbody><tr v-for="row in rows" :key="row.date"><th scope="row">{{ row.date }}</th><td v-for="column in columns" :key="column.key"><bdi>{{ column.format(Number(row[column.key])) }}</bdi></td></tr></tbody>
      </table>
    </div>
  </details>
</template>

<script setup lang="ts" generic="Row extends { date: string }">
import { useI18n } from 'vue-i18n'
defineProps<{ caption: string; columns: { key: keyof Row & string; label: string; format: (value: number) => string }[]; rows: Row[] }>()
const { t } = useI18n()
</script>
