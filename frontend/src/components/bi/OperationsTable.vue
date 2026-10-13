<template>
  <div :aria-busy="loading" class="mt-4 space-y-3">
    <p v-if="loading" role="status">{{ t('common.loading') }}</p><div v-if="error" role="alert"><p>{{ error }}</p><button class="btn btn-secondary" @click="$emit('retry')">{{ t('biNative.retry') }}</button></div><p v-if="!loading && !error && !rows.length">{{ t('biNative.noData') }}</p>
    <div v-if="rows.length" class="overflow-x-auto"><table class="min-w-full text-sm"><thead><tr><th v-for="column in columns" :key="column" class="px-3 py-2 text-left font-medium">{{ t(`biNative.fields.${column}`) }}</th><th v-if="$slots.actions" class="px-3 py-2">{{ t('common.actions') }}</th></tr></thead><tbody><tr v-for="(row,index) in rows" :key="String(row.id ?? row.task_id ?? index)" class="border-t dark:border-dark-700"><td v-for="column in columns" :key="column" class="max-w-xs break-words px-3 py-3">{{ typeof row[column] === 'boolean' ? t(row[column] ? 'common.enabled' : 'common.disabled') : row[column] ?? '—' }}</td><td v-if="$slots.actions" class="px-3 py-3"><slot name="actions" :row="row" /></td></tr></tbody></table></div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { BIOperationRow } from '@/api/admin/bi'
defineProps<{ rows: BIOperationRow[]; columns: string[]; loading: boolean; error: string }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>
