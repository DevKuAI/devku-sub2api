<template>
  <section class="min-w-0 space-y-4 border-t border-gray-200 pt-5 dark:border-dark-700" :aria-label="t('admin.desktop.usageStatistics.memberModels')" data-testid="organization-member-model-usage">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div class="min-w-0">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.desktop.usageStatistics.memberModels') }}</h4>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.memberModelsHint') }}</p>
      </div>
      <label class="w-full space-y-1 sm:w-72">
        <span class="block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.desktop.usageStatistics.memberModelSearch') }}</span>
        <input v-model="search" class="input" type="search" :placeholder="t('admin.desktop.usageStatistics.memberModelSearch')" data-testid="member-model-search" />
      </label>
    </div>
    <DataTable :columns="columns" :data="pageRows" :row-key="rowKey">
      <template #cell-name="{ row }">
        <div class="max-w-56 whitespace-normal">
          <span class="break-words font-medium">{{ row.name }}</span>
          <span v-if="row.deleted" class="ml-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.conversations.deletedMember') }}</span>
          <div class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-dark-400">{{ row.member_id }}</div>
        </div>
      </template>
      <template #cell-model="{ row }"><span class="block max-w-64 whitespace-normal break-all">{{ row.model }}</span></template>
      <template #empty><p>{{ t(rows.length ? 'admin.desktop.usageStatistics.noMatchingMemberModels' : 'admin.desktop.usageStatistics.noUsage') }}</p></template>
    </DataTable>
    <Pagination v-if="filteredRows.length" v-model:page="page" :page-size="pageSize" :total="filteredRows.length" :show-page-size-selector="false" />
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DesktopUsageMemberModel } from '@/api/desktopOrganizationUsage'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import type { Column } from '@/components/common/types'

const props = defineProps<{ rows: DesktopUsageMemberModel[] }>()
const { t } = useI18n()
const search = ref('')
const page = ref(1)
const pageSize = 20
const tokenColumns = [
  { key: 'input_tokens', label: 'inputTokens' },
  { key: 'output_tokens', label: 'outputTokens' },
  { key: 'cache_creation_tokens', label: 'cacheCreationTokens' },
  { key: 'cache_read_tokens', label: 'cacheReadTokens' },
  { key: 'total_tokens', label: 'totalTokens' },
] as const
const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.desktop.usageStatistics.member') },
  { key: 'model', label: t('admin.desktop.usageStatistics.model') },
  { key: 'requests', label: t('admin.desktop.usageStatistics.records'), class: 'text-right tabular-nums', formatter: (value: number) => value.toLocaleString() },
  ...tokenColumns.map(column => ({ key: column.key, label: t(`admin.desktop.usageStatistics.${column.label}`), class: 'text-right tabular-nums', formatter: (value: number) => value.toLocaleString() })),
])
const filteredRows = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return props.rows.filter(row => !query || [row.name, row.member_id, row.model].some(value => value.toLocaleLowerCase().includes(query)))
})
const pageRows = computed(() => filteredRows.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const rowKey = (row: DesktopUsageMemberModel) => JSON.stringify([row.member_id, row.model])
watch([search, () => props.rows], () => { page.value = 1 })
</script>
