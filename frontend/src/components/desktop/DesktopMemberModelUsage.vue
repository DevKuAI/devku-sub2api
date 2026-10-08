<template>
  <section class="member-usage space-y-5 border-t border-gray-200 pt-6 dark:border-dark-700" :aria-label="t('admin.desktop.usageStatistics.memberModels')" :aria-busy="loading" data-testid="organization-member-model-usage">
    <header>
      <h4 class="text-base font-semibold leading-normal text-gray-900 dark:text-gray-100">{{ t('admin.desktop.usageStatistics.memberModels') }}</h4>
      <p class="mt-1 text-sm leading-normal text-gray-600 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.memberModelDescription') }}</p>
    </header>

    <dl class="usage-overview rounded-xl bg-primary-50 p-5 dark:bg-dark-800" data-testid="member-usage-overview">
      <div class="overview-total min-w-0">
        <dt class="text-sm text-gray-700 dark:text-dark-300">{{ t(hasFilters ? 'admin.desktop.usageStatistics.filteredTokens' : 'admin.desktop.usageStatistics.totalTokens') }}</dt>
        <dd class="mt-2 break-all text-3xl font-semibold leading-tight tabular-nums text-primary-700 dark:text-primary-300" data-testid="member-usage-total"><bdi>{{ loading ? '—' : formatTokens(totalTokens) }}</bdi></dd>
      </div>
      <div class="min-w-0">
        <dt class="text-sm text-gray-700 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.usageMembers') }}</dt>
        <dd class="mt-2 text-xl font-semibold leading-tight tabular-nums text-gray-900 dark:text-gray-100" data-testid="member-usage-count">{{ loading ? '—' : formatNumber(members.length) }}</dd>
      </div>
      <div class="min-w-0">
        <dt class="text-sm text-gray-700 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.usageModels') }}</dt>
        <dd class="mt-2 text-xl font-semibold leading-tight tabular-nums text-gray-900 dark:text-gray-100">{{ loading ? '—' : formatNumber(matchedModelCount) }}</dd>
      </div>
    </dl>

    <div class="usage-toolbar">
      <label class="search-field min-w-0 space-y-1.5">
        <span class="block text-xs font-medium text-gray-700 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.memberModelSearch') }}</span>
        <input ref="searchInput" v-model="search" class="usage-control" type="search" name="member-model-search" :disabled="loading" :placeholder="t('admin.desktop.usageStatistics.memberModelSearchExample')" data-testid="member-model-search" />
      </label>
      <label class="min-w-0 space-y-1.5">
        <span class="block text-xs font-medium text-gray-700 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.model') }}</span>
        <select v-model="selectedModel" class="usage-control" :disabled="loading" data-testid="member-model-filter">
          <option value="">{{ t('admin.desktop.usageStatistics.allModels') }}</option>
          <option v-for="model in availableModels" :key="model" :value="model">{{ model }}</option>
        </select>
      </label>
      <label class="min-w-0 space-y-1.5">
        <span class="block text-xs font-medium text-gray-700 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.memberSort') }}</span>
        <select v-model="sortBy" class="usage-control" :disabled="loading" data-testid="member-usage-sort">
          <option value="tokens">{{ t('admin.desktop.usageStatistics.mostTokensFirst') }}</option>
          <option value="name">{{ t('admin.desktop.usageStatistics.memberNameOrder') }}</option>
        </select>
      </label>
      <fieldset class="unit-field min-w-0">
        <legend class="mb-1.5 text-xs font-medium leading-normal text-gray-700 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.numberDisplay') }}</legend>
        <div class="grid grid-cols-2 gap-2">
          <button v-for="mode in numberModes" :key="mode.value" class="usage-mode-button" :class="{ 'is-selected': numberMode === mode.value }" type="button" :disabled="loading" :aria-pressed="numberMode === mode.value" :data-testid="`member-token-${mode.value}`" @click="numberMode = mode.value">
            <Icon v-if="numberMode === mode.value" name="check" size="xs" :stroke-width="2" aria-hidden="true" />{{ t(`admin.desktop.usageStatistics.${mode.label}`) }}
          </button>
        </div>
      </fieldset>
    </div>

    <div class="flex min-w-0 flex-wrap items-center justify-between gap-3">
      <p class="min-w-0 break-words text-sm leading-normal text-gray-600 dark:text-dark-300" role="status" aria-live="polite" data-testid="member-usage-results">{{ announced ? t(loading ? 'admin.desktop.usageStatistics.loadingMembers' : 'admin.desktop.usageStatistics.memberUsageResults', { members: formatNumber(members.length), models: formatNumber(filteredRows.length) }) : '' }}</p>
      <button v-if="hasFilters && !loading" class="usage-action" type="button" @click="clearFilters">{{ t('admin.desktop.usageStatistics.clearUsageFilters') }}</button>
    </div>
    <p v-if="selectedModel" class="break-all text-sm leading-normal text-gray-600 dark:text-dark-300">{{ t('admin.desktop.usageStatistics.selectedUsageModel', { model: selectedModel }) }}</p>

    <div v-if="loading" class="rounded-xl border border-gray-200 bg-white p-6 text-sm leading-normal text-gray-600 dark:border-dark-700 dark:bg-dark-900" data-testid="member-usage-loading">{{ t('admin.desktop.usageStatistics.loadingMembers') }}</div>
    <div v-else-if="!members.length" class="space-y-3 rounded-xl border border-gray-200 bg-white p-6 text-center dark:border-dark-700 dark:bg-dark-900" data-testid="member-usage-empty">
      <p class="text-sm font-semibold leading-normal text-gray-900 dark:text-gray-100">{{ t(hasFilters ? 'admin.desktop.usageStatistics.noMatchingMemberModels' : 'admin.desktop.usageStatistics.noUsage') }}</p>
      <p class="mx-auto max-w-prose break-words text-sm leading-normal text-gray-600 dark:text-dark-300">{{ hasFilters ? t('admin.desktop.usageStatistics.noMatchingUsageHint', { query: filterDescription }) : t('admin.desktop.usageStatistics.noMemberUsageHint') }}</p>
      <button class="usage-action" type="button" @click="hasFilters ? clearFilters() : emit('refresh')">{{ t(hasFilters ? 'admin.desktop.usageStatistics.clearUsageFilters' : 'common.refresh') }}</button>
    </div>
    <div v-else class="space-y-3" data-testid="member-usage-list">
      <details v-for="(member, index) in pageMembers" :key="member.memberId" :open="index === 0" class="member-card rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900" data-testid="member-usage-card">
        <summary class="member-summary usage-focus cursor-pointer list-none p-4">
          <div class="flex min-w-0 items-start gap-3">
            <Icon name="chevronRight" size="sm" :stroke-width="2" class="member-chevron mt-1 shrink-0 text-gray-600 dark:text-dark-300" aria-hidden="true" />
            <div class="min-w-0 space-y-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="break-words text-sm font-semibold text-gray-900 dark:text-gray-100"><bdi>{{ member.name }}</bdi></span>
                <span v-if="member.deleted" class="rounded bg-gray-100 px-2 py-0.5 text-xs font-medium leading-normal text-gray-700 dark:bg-dark-800 dark:text-dark-300">{{ t('admin.desktop.conversations.deletedMember') }}</span>
              </div>
              <p class="break-all font-mono text-xs leading-normal text-gray-600 dark:text-dark-400"><bdi>{{ member.memberId }}</bdi></p>
              <p class="text-xs leading-normal text-gray-600 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.memberUsageSummary', { models: formatNumber(member.models.length), records: formatNumber(member.requests) }) }}</p>
            </div>
          </div>
          <div class="member-total min-w-0">
            <span class="text-xs text-gray-600 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.totalTokens') }}</span>
            <p class="break-all text-xl font-semibold leading-tight tabular-nums text-gray-900 dark:text-gray-100" data-testid="member-total"><bdi>{{ formatTokens(member.totalTokens) }}</bdi></p>
          </div>
        </summary>
        <div class="divide-y divide-gray-100 border-t border-gray-200 dark:divide-dark-800 dark:border-dark-700">
          <article v-for="model in member.models" :key="model.model" class="space-y-3 p-4" :aria-label="model.model" data-testid="member-model-row">
            <div class="model-heading min-w-0">
              <div class="min-w-0 flex-1">
                <p class="break-all text-sm font-semibold leading-normal text-gray-900 dark:text-gray-100"><bdi>{{ model.model }}</bdi></p>
                <p class="mt-1 text-xs leading-normal text-gray-600 dark:text-dark-400">{{ t('admin.desktop.usageStatistics.billingRecords', { count: formatNumber(model.requests) }) }}</p>
              </div>
              <p class="break-all text-lg font-semibold leading-normal tabular-nums text-primary-700 dark:text-primary-300" data-testid="model-total"><bdi>{{ formatTokens(model.total_tokens) }}</bdi><span class="ms-1 text-xs font-normal text-gray-600 dark:text-dark-400">Token</span></p>
            </div>
            <dl class="token-parts">
              <div v-for="part in tokenParts" :key="part.key" class="min-w-0" :data-token-kind="part.key">
                <dt class="text-xs leading-normal text-gray-600 dark:text-dark-400">{{ t(`admin.desktop.usageStatistics.${part.label}`) }}</dt>
                <dd class="mt-1 break-all text-sm font-medium leading-normal tabular-nums text-gray-900 dark:text-gray-100"><bdi>{{ formatTokens(model[part.key]) }}</bdi></dd>
              </div>
            </dl>
          </article>
        </div>
      </details>
    </div>
    <Pagination v-if="members.length > pageSize && !loading" v-model:page="page" class="usage-pagination" :page-size="pageSize" :total="members.length" :show-page-size-selector="false" />

    <details class="usage-notes text-xs leading-normal text-gray-600 dark:text-dark-400">
      <summary class="usage-focus cursor-pointer py-2 font-medium">{{ t('admin.desktop.usageStatistics.usageCalculation') }}</summary>
      <div class="mt-1 max-w-prose space-y-2">
        <p>{{ t('admin.desktop.usageStatistics.tokenUnitHint') }}</p>
        <p>{{ t('admin.desktop.usageStatistics.memberModelsHint') }}</p>
      </div>
    </details>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DesktopUsageMemberModel } from '@/api/desktopOrganizationUsage'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { createDesktopUsageFormatter, type TokenNumberMode } from '@/utils/desktopUsageFormat'

interface UsageMember {
  memberId: string
  name: string
  deleted: boolean
  models: DesktopUsageMemberModel[]
  totalTokens: number
  requests: number
}
const props = withDefaults(defineProps<{ rows: DesktopUsageMemberModel[]; loading?: boolean; numberMode?: TokenNumberMode }>(), { loading: false })
const emit = defineEmits<{ refresh: []; 'update:numberMode': [value: TokenNumberMode] }>()
const { t, locale } = useI18n()
const search = ref('')
const searchInput = ref<HTMLInputElement | null>(null)
const selectedModel = ref('')
const sortBy = ref<'tokens' | 'name'>('tokens')
const localNumberMode = ref<TokenNumberMode>('compact')
const numberMode = computed({
  get: () => props.numberMode ?? localNumberMode.value,
  set: (value: TokenNumberMode) => { localNumberMode.value = value; emit('update:numberMode', value) },
})
const numberModes = [{ value: 'compact', label: 'compactTokens' }, { value: 'exact', label: 'exactTokens' }] as const
const page = ref(1)
const pageSize = 10
const announced = ref(false)
const language = computed(() => locale?.value || 'zh-CN')
const collator = computed(() => new Intl.Collator(language.value, { numeric: true, sensitivity: 'base' }))
const formatter = computed(() => createDesktopUsageFormatter(language.value))
const formatNumber = (value: number) => formatter.value.number(value)
const formatTokens = (value: number) => formatter.value.tokens(value, numberMode.value)
const tokenParts = [
  { key: 'input_tokens', label: 'inputTokens' },
  { key: 'output_tokens', label: 'outputTokens' },
  { key: 'cache_creation_tokens', label: 'cacheCreationTokens' },
  { key: 'cache_read_tokens', label: 'cacheReadTokens' },
] as const
const availableModels = computed(() => [...new Set(props.rows.map(row => row.model))].sort(collator.value.compare))
const hasFilters = computed(() => Boolean(search.value.trim() || selectedModel.value))
const filterDescription = computed(() => search.value.trim() || selectedModel.value)
const filteredRows = computed(() => {
  const query = search.value.trim().toLocaleLowerCase(language.value)
  return props.rows.filter(row => (!selectedModel.value || row.model === selectedModel.value) &&
    (!query || [row.name, row.member_id, row.model].some(value => value.toLocaleLowerCase(language.value).includes(query))))
})
const members = computed(() => {
  const groups = new Map<string, UsageMember>()
  for (const row of filteredRows.value) {
    let member = groups.get(row.member_id)
    if (!member) {
      member = { memberId: row.member_id, name: row.name, deleted: row.deleted, models: [], totalTokens: 0, requests: 0 }
      groups.set(row.member_id, member)
    }
    member.models.push(row)
    member.totalTokens += row.total_tokens
    member.requests += row.requests
  }
  return [...groups.values()].map(member => ({ ...member, models: member.models.sort((a, b) => b.total_tokens - a.total_tokens || collator.value.compare(a.model, b.model)) }))
    .sort((a, b) => (sortBy.value === 'tokens' ? b.totalTokens - a.totalTokens : 0) || collator.value.compare(a.name, b.name) || collator.value.compare(a.memberId, b.memberId))
})
const totalTokens = computed(() => filteredRows.value.reduce((total, row) => total + row.total_tokens, 0))
const matchedModelCount = computed(() => new Set(filteredRows.value.map(row => row.model)).size)
const pageMembers = computed(() => members.value.slice((page.value - 1) * pageSize, page.value * pageSize))
function clearFilters() { search.value = ''; selectedModel.value = ''; searchInput.value?.focus() }
watch([search, selectedModel, sortBy, () => props.rows], () => { page.value = 1 })
watch(() => props.rows, () => { if (!availableModels.value.includes(selectedModel.value)) selectedModel.value = '' })
onMounted(() => { announced.value = true })
</script>

<style scoped>
.member-usage { container-type: inline-size; width: 100%; }
.usage-overview { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; }
.overview-total { grid-column: 1 / -1; }
.usage-toolbar { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.search-field, .unit-field { grid-column: 1 / -1; }
.usage-control { @apply w-full rounded-xl border border-gray-500 bg-white px-3 py-2.5 text-base font-normal leading-normal text-gray-900 dark:border-dark-400 dark:bg-dark-800 dark:text-gray-100; }
.usage-control { min-height: 46px; }
.usage-mode-button { @apply inline-flex min-h-11 min-w-0 items-center justify-center gap-1 rounded-lg border border-gray-500 bg-white px-2 py-2 text-sm font-medium leading-normal text-gray-700 dark:border-dark-400 dark:bg-dark-800 dark:text-dark-200; }
.usage-mode-button.is-selected { @apply border-primary-600 bg-primary-50 font-semibold text-primary-700 dark:border-primary-300 dark:bg-dark-800 dark:text-primary-300; }
.usage-action { @apply min-h-11 max-w-full rounded-lg border border-gray-500 bg-white px-3 py-2 text-sm font-medium leading-normal text-gray-700 dark:border-dark-400 dark:bg-dark-800 dark:text-dark-200; }
.usage-control:disabled, .usage-mode-button:disabled { @apply cursor-not-allowed opacity-50; }
.usage-control:focus-visible, .usage-action:focus-visible, .usage-mode-button:focus-visible, .usage-focus:focus-visible, .usage-pagination :deep(button:focus-visible) { outline: 2px solid theme('colors.primary.600'); outline-offset: 3px; }
html.dark .usage-control:focus-visible, html.dark .usage-action:focus-visible, html.dark .usage-mode-button:focus-visible, html.dark .usage-focus:focus-visible, html.dark .usage-pagination :deep(button:focus-visible) { outline-color: theme('colors.primary.400'); }
.member-summary { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; }
.model-heading { display: grid; grid-template-columns: minmax(0, 1fr); gap: 8px; }
.usage-toolbar span, .usage-overview dt, .member-total > span { line-height: 1.5; }
.member-summary::-webkit-details-marker { display: none; }
.member-card[open] > .member-summary .member-chevron { transform: rotate(90deg); }
[dir='rtl'] .member-chevron { transform: rotate(180deg); }
[dir='rtl'] .member-card[open] > .member-summary .member-chevron { transform: rotate(90deg); }
.member-total { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; }
.token-parts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 20px; }
.usage-pagination :deep(button) { @apply min-h-11 border-gray-500 dark:border-dark-400; }
.usage-pagination :deep(button[aria-current='page']) { @apply font-semibold underline underline-offset-4; }
@container (min-width: 32rem) {
  .search-field, .unit-field { grid-column: auto; }
  .usage-control { @apply text-sm; min-height: 44px; }
  .usage-overview { grid-template-columns: minmax(0, 2fr) repeat(2, minmax(0, 1fr)); }
  .overview-total { grid-column: auto; }
  .member-summary { grid-template-columns: minmax(0, 1fr) minmax(0, auto); align-items: center; }
  .member-total { display: block; text-align: end; }
  .member-total p { margin-top: 4px; }
  .model-heading { grid-template-columns: minmax(0, 1fr) minmax(0, auto); align-items: baseline; }
  .token-parts { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}
@container (min-width: 48rem) { .usage-toolbar { grid-template-columns: minmax(0, 2fr) minmax(0, 1.25fr) minmax(0, 1.1fr) minmax(0, 1.65fr); } }
@media (forced-colors: active) { .usage-control:focus-visible, .usage-action:focus-visible, .usage-mode-button:focus-visible, .usage-focus:focus-visible, .usage-pagination :deep(button:focus-visible) { outline-color: Highlight; } }
</style>
